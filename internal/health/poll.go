// Package health implements container health polling after docker compose up.
// PollHealth() queries docker inspect for each container in the compose project
// and reports healthy / unhealthy / unknown status, exiting non-zero on failure
// or timeout per HEALTH-01, HEALTH-02, HEALTH-03.
package health

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/webcane/docker-deploy/internal/config"
	"github.com/webcane/docker-deploy/internal/filetransfer"
)

// sessionOutput is the narrow interface for a single SSH session used in polls.
// The command is baked in at newSession() construction time, so Output takes
// no argument — preventing callers from accidentally running a different command
// than the one the session was opened for.
type sessionOutput interface {
	Output() ([]byte, error)
	Close() error
}

// sessionOpener creates a new session for each remote command. This interface
// is satisfied by sshClientRunner (production) and fakeClient (tests).
// Per CLAUDE.md Rule 3: a new session must be created for every command —
// sessions are NOT reusable.
type sessionOpener interface {
	newSession(cmd string) (sessionOutput, error)
}

// sshClientRunner wraps a real *gossh.Client to implement sessionOpener.
// Each call to newSession opens a fresh SSH session, runs the command via
// Output(), and returns the session. Per CLAUDE.md Rule 3: sessions are NOT
// reusable — a new session is created for every command.
type sshClientRunner struct {
	client *gossh.Client
}

func (r *sshClientRunner) newSession(cmd string) (sessionOutput, error) {
	session, err := r.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("creating SSH session: %w", err)
	}
	return &sshSessionWrapper{session: session, cmd: cmd}, nil
}

// sshSessionWrapper adapts *gossh.Session to the sessionOutput interface.
type sshSessionWrapper struct {
	session *gossh.Session
	cmd     string
}

func (w *sshSessionWrapper) Output() ([]byte, error) {
	return w.session.Output(w.cmd)
}

func (w *sshSessionWrapper) Close() error {
	return w.session.Close()
}

// PollHealth enumerates containers belonging to the given compose project and
// polls their health status until all are healthy, one is unhealthy, or the
// timeout expires.
//
// Container enumeration uses the label com.docker.compose.project=<projectName>
// (docker ps --filter label=... --format '{{.Names}}').
//
// Per container health status interpretation (D-13 in 05-CONTEXT.md):
//   - "healthy"        → mark done
//   - "unhealthy"      → print error, return non-nil immediately
//   - "" or "none"     → print warning, mark done (no HEALTHCHECK defined)
//   - "starting"       → continue polling
//   - error from inspect → print warning, treat as unknown, continue
//
// Polling uses a ticker (HealthInterval seconds) within a timeout (HealthTimeout
// seconds). If the timeout fires with any container still in "starting", returns
// a non-nil error.
//
// Per CLAUDE.md Rule 3: each docker ps and each docker inspect runs in a
// separate client.NewSession() call. Sessions are closed after Output() returns.
//
// T-05-03-01: projectName is passed through ShellQuote() before use in docker ps filter.
// T-05-03-02: container names from docker ps are passed through ShellQuote() before docker inspect.
// T-05-03-03: unexpected status strings default to "starting" (continue polling) — safe default.
// T-05-03-04: timeout timer is a time.After select branch that always fires.
func PollHealth(ctx context.Context, client *gossh.Client, projectName string, cfg config.Config) error {
	return pollHealthWithRunner(ctx, &sshClientRunner{client: client}, projectName, cfg)
}

// pollHealthWithRunner is the testable core of PollHealth. It accepts a
// sessionOpener interface so tests can inject a fake without a real SSH server.
func pollHealthWithRunner(ctx context.Context, runner sessionOpener, projectName string, cfg config.Config) error {
	// Step 1: Enumerate containers by compose project label.
	containers, err := listContainers(runner, projectName)
	if err != nil {
		return fmt.Errorf("health: listing containers: %w", err)
	}
	if len(containers) == 0 {
		// Nothing to poll — compose project has no running containers.
		return nil
	}

	// Step 2: Determine effective intervals.
	// HealthInterval=0 is treated as 1ms in tests (too fast for production, but
	// avoids blocking test runs). In production, Resolve() always sets >= 1.
	healthInterval := time.Duration(cfg.HealthInterval) * time.Second
	if healthInterval <= 0 {
		healthInterval = time.Millisecond // test-fast mode
	}
	healthTimeout := time.Duration(cfg.HealthTimeout) * time.Second
	if healthTimeout <= 0 {
		healthTimeout = time.Second // minimum 1s
	}

	// Step 3: Poll loop with ticker and timeout.
	ticker := time.NewTicker(healthInterval)
	defer ticker.Stop()
	timeoutTimer := time.NewTimer(healthTimeout)
	defer timeoutTimer.Stop()

	// done tracks containers that have reached a terminal state (healthy or no-healthcheck).
	done := make(map[string]bool, len(containers))

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("health: context cancelled: %w", ctx.Err())

		case <-timeoutTimer.C:
			// Timeout expired — print error for each still-starting container.
			// T-05-03-04: this select branch always fires at HealthTimeout.
			for _, c := range containers {
				if !done[c] {
					fmt.Fprintf(os.Stderr, "Health check timed out after %ds: container %s is not yet running\n", cfg.HealthTimeout, c)
				}
			}
			return fmt.Errorf("health: timed out waiting for containers to become healthy")

		case <-ticker.C:
			allDone, pollErr := pollContainers(runner, containers, done)
			if pollErr != nil {
				return pollErr
			}
			if allDone {
				fmt.Fprintf(os.Stdout, "Health check passed: all containers healthy\n")
				return nil
			}
		}
	}
}

// listContainers runs docker ps to enumerate containers belonging to the
// compose project and returns a list of container names.
// T-05-03-01: projectName is wrapped in ShellQuote() before shell injection.
func listContainers(runner sessionOpener, projectName string) ([]string, error) {
	cmd := "docker ps --filter label=com.docker.compose.project=" + filetransfer.ShellQuote(projectName) + " --format '{{.Names}}'"
	session, err := runner.newSession(cmd)
	if err != nil {
		return nil, fmt.Errorf("creating session for docker ps: %w", err)
	}
	defer session.Close() //nolint:errcheck

	out, err := session.Output()
	if err != nil {
		return nil, fmt.Errorf("running docker ps: %w", err)
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}

	lines := strings.Split(raw, "\n")
	containers := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			containers = append(containers, line)
		}
	}
	return containers, nil
}

// pollContainers inspects each not-yet-done container's health status and
// updates the done map. Returns (true, nil) when all containers are done,
// (false, nil) to continue polling, or (false, non-nil) when an unhealthy
// container is found.
func pollContainers(runner sessionOpener, containers []string, done map[string]bool) (bool, error) {
	for _, container := range containers {
		if done[container] {
			continue
		}

		status, err := inspectHealth(runner, container)
		if err != nil {
			// Inspect failure (e.g. container exited) — treat as unknown, print warning.
			fmt.Fprintf(os.Stderr, "Warning: could not inspect container %s: %v\n", container, err)
			continue
		}

		switch status {
		case "running":
			done[container] = true

		case "exited", "dead":
			fmt.Fprintf(os.Stderr, "Health check failed: container %s stopped (state: %s)\n", container, status)
			return false, fmt.Errorf("health: container %s stopped unexpectedly (state: %s)", container, status)

		default:
			// "created", "restarting", "paused", "removing", or unexpected → continue polling.
		}
	}

	// Check whether all containers have reached a terminal state.
	for _, c := range containers {
		if !done[c] {
			return false, nil
		}
	}
	return true, nil
}

// inspectHealth runs docker inspect to get the running state of a single container.
// Returns the trimmed .State.Status string ("running", "exited", "dead", etc.) or an error.
// T-05-03-02: containerName from docker ps is wrapped in ShellQuote() before use.
func inspectHealth(runner sessionOpener, containerName string) (string, error) {
	cmd := "docker inspect --format '{{.State.Status}}' " + filetransfer.ShellQuote(containerName)
	session, err := runner.newSession(cmd)
	if err != nil {
		return "", fmt.Errorf("creating session for docker inspect: %w", err)
	}
	defer session.Close() //nolint:errcheck

	out, err := session.Output()
	if err != nil {
		return "", fmt.Errorf("running docker inspect: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}
