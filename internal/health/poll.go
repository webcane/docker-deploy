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
	out, err := w.session.Output(w.cmd)
	if err != nil {
		return nil, fmt.Errorf("running SSH command: %w", err)
	}
	return out, nil
}

func (w *sshSessionWrapper) Close() error {
	if err := w.session.Close(); err != nil {
		return fmt.Errorf("closing SSH session: %w", err)
	}
	return nil
}

// PollHealth enumerates containers belonging to the given compose project and
// polls their health status until all are healthy, one is unhealthy, or the
// timeout expires.
//
// Container enumeration uses the label com.docker.compose.project=<projectName>
// (docker ps --filter label=... --format '{{.Names}}').
//
// Per container health status interpretation (D-13 in 05-CONTEXT.md):
//   - "healthy"        → mark done; reset per-container failCount to 0 (D-09)
//   - "no-healthcheck" → mark done; reset per-container failCount to 0
//   - "unhealthy"      → if cfg.Healthcheck.Retries==0: immediate fail (preserves existing behaviour);
//     if Retries>0: increment per-container failCount; fail only when failCount >= Retries (D-09, D-10)
//   - "starting"       → continue polling
//   - error from inspect → print warning, treat as unknown, continue
//
// Polling uses a ticker (cfg.Healthcheck.Interval) within a timeout (cfg.Healthcheck.Timeout).
// If the timeout fires with any container still in "starting", returns a non-nil error.
//
// Per CLAUDE.md Rule 3: each docker ps and each docker inspect runs in a
// separate client.NewSession() call. Sessions are closed after Output() returns.
//
// T-05-03-01: projectName is passed through ShellQuote() before use in docker ps filter.
// T-05-03-02: container names from docker ps are passed through ShellQuote() before docker inspect.
// T-05-03-03: unexpected status strings default to "starting" (continue polling) — safe default.
// T-05-03-04: timeout timer is a time.After select branch that always fires.
// T-15-02-01: zero-value guards ensure ticker cannot panic and timeout cannot be infinite.
// T-15-02-04: timeout timer always fires regardless of retries — polling cannot exceed Timeout.
func PollHealth(ctx context.Context, client *gossh.Client, projectName string, cfg config.Config) error {
	return pollHealthWithRunner(ctx, &sshClientRunner{client: client}, projectName, cfg)
}

// pollHealthWithRunner is the testable core of PollHealth. It accepts a
// sessionOpener interface so tests can inject a fake without a real SSH server.
func pollHealthWithRunner(ctx context.Context, runner sessionOpener, projectName string, cfg config.Config) error { //nolint:gocognit // poll loop handles 4 terminal states + timeout + context cancel — complexity is inherent to polling logic
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
	// Healthcheck.Interval == 0 (absent from all config sources) is treated as
	// 1ms in tests (too fast for production, but avoids blocking test runs).
	// Healthcheck.Timeout == 0 is treated as 1s minimum to avoid an immediate timeout.
	healthInterval := cfg.Healthcheck.Interval
	if healthInterval <= 0 {
		healthInterval = time.Millisecond // test-fast mode / absent config
	}
	healthTimeout := cfg.Healthcheck.Timeout
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
	// failCount tracks consecutive unhealthy results per container for retries semantics (D-09, D-10).
	failCount := make(map[string]int, len(containers))

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("health: context cancelled: %w", ctx.Err())

		case <-timeoutTimer.C:
			// Timeout expired — print error for each still-starting container.
			// T-05-03-04: this select branch always fires at cfg.Healthcheck.Timeout.
			// T-15-02-04: timeout fires regardless of retries setting.
			for _, c := range containers {
				if !done[c] {
					fmt.Fprintf(os.Stderr, "Health check timed out after %s: container %s is not yet running\n", cfg.Healthcheck.Timeout, c)
				}
			}
			return fmt.Errorf("health: timed out waiting for containers to become healthy")

		case <-ticker.C:
			allDone, pollErr := pollContainers(runner, containers, done, failCount, cfg.Healthcheck.Retries)
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
	// Quote the entire filter token as one unit so the shell sees it as a single
	// argument and Docker receives the value without surrounding single-quote chars
	// (CR-02: ShellQuote must wrap the full label=key=value token, not just the value).
	// Note: Docker label filter parsing splits on the first '=', so a projectName
	// containing '=' would produce a malformed filter. Directory names with '=' are
	// an edge case but worth documenting as a known Docker CLI limitation.
	filterVal := "label=com.docker.compose.project=" + projectName
	cmd := "docker ps -a --filter " + filetransfer.ShellQuote(filterVal) + " --format '{{.Names}}'"
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
// updates the done and failCount maps. Returns (true, nil) when all containers
// are done, (false, nil) to continue polling, or (false, non-nil) when a
// container has failed.
//
// The failCount map tracks consecutive unhealthy results per container (D-10).
// When a container becomes healthy or no-healthcheck, its failCount is reset to
// zero so that a later flap starts counting fresh (D-09: a single healthy result
// resets the consecutive counter).
//
// The retries parameter controls when an unhealthy result terminates polling:
//   - retries == 0: existing immediate-fail behaviour preserved (backwards compat)
//   - retries > 0: increment failCount[container]; only fail when failCount >= retries
func pollContainers(runner sessionOpener, containers []string, done map[string]bool, failCount map[string]int, retries int) (bool, error) {
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
		case "healthy", "no-healthcheck":
			done[container] = true
			failCount[container] = 0 // reset consecutive-unhealthy counter (D-09)

		case "unhealthy":
			if err := recordUnhealthy(container, failCount, retries); err != nil {
				return false, err
			}

		case "exited", "dead":
			fmt.Fprintf(os.Stderr, "Health check failed: container %s stopped unexpectedly\n", container)
			return false, fmt.Errorf("health: container %s stopped unexpectedly", container)

		default:
			// "starting" or unexpected → continue polling.
		}
	}

	return allContainersDone(containers, done), nil
}

// recordUnhealthy accumulates a consecutive-unhealthy result for a container
// and returns an error once the failure threshold is reached. retries==0 means
// fail immediately (preserves existing behaviour, T-15-02-02).
func recordUnhealthy(container string, failCount map[string]int, retries int) error {
	if retries == 0 {
		fmt.Fprintf(os.Stderr, "Health check failed: container %s is unhealthy\n", container)
		return fmt.Errorf("health: container %s is unhealthy", container)
	}
	// Retries configured: accumulate consecutive unhealthy results (D-09, D-10).
	failCount[container]++
	if failCount[container] >= retries {
		fmt.Fprintf(os.Stderr, "Health check failed: container %s is unhealthy (%d consecutive unhealthy results)\n", container, failCount[container])
		return fmt.Errorf("health: container %s is unhealthy after %d consecutive unhealthy results", container, failCount[container])
	}
	// Below threshold — continue polling on next tick; do not mark done.
	return nil
}

// allContainersDone reports whether every container has reached a terminal state.
func allContainersDone(containers []string, done map[string]bool) bool {
	for _, c := range containers {
		if !done[c] {
			return false
		}
	}
	return true
}

// inspectHealth returns the health status of a container. For containers with a
// HEALTHCHECK it returns the health check status ("healthy", "unhealthy", "starting");
// for containers without one it returns "no-healthcheck". Lifecycle states
// ("exited", "dead") are detected via a fallback format template.
// T-05-03-02: containerName from docker ps is wrapped in ShellQuote() before use.
func inspectHealth(runner sessionOpener, containerName string) (string, error) {
	cmd := "docker inspect --format '{{if or (eq .State.Status \"exited\") (eq .State.Status \"dead\")}}{{.State.Status}}{{else if .State.Health}}{{.State.Health.Status}}{{else}}no-healthcheck{{end}}' " + filetransfer.ShellQuote(containerName)
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
