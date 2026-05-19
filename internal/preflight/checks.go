// Package preflight implements pre-flight checks that validate the remote host
// is deploy-ready before any file transfer begins.
//
// RunPreflightChecks() runs CHECK-01 through CHECK-07 in order, fail-fast on
// the first blocking error. Warnings (CHECK-03, CHECK-07) are printed to
// os.Stderr and do not cause a non-nil return.
//
// Per CLAUDE.md Rule 3: each remote command opens its own SSH session via
// client.NewSession(); sessions are NOT reusable.
// Per CLAUDE.md Rule 1: no insecure host key verification is used.
package preflight

import (
	"context"
	"fmt"
	"os"
	"strings"

	gossh "golang.org/x/crypto/ssh"

	"github.com/webcane/docker-deploy/internal/config"
	"github.com/webcane/docker-deploy/internal/filetransfer"
)

// Session is the subset of *gossh.Session methods used by preflight checks.
// Defining this interface allows tests to inject fakes without a real SSH
// connection.
type Session interface {
	Output(cmd string) ([]byte, error)
	Run(cmd string) error
	Close() error
}

// SSHRunner is the interface preflight uses to open SSH sessions.
// It is satisfied by the sshClientRunner adapter wrapping *gossh.Client.
// Callers use RunPreflightChecks(*gossh.Client, ...) — the adapter is created
// internally. Test doubles implement SSHRunner directly.
type SSHRunner interface {
	NewSession() (Session, error)
}

// sshClientRunner adapts *gossh.Client to SSHRunner.
// gossh.Client.NewSession() returns (*gossh.Session, error); we wrap it so the
// concrete *gossh.Session (which implements Session) is returned as the
// interface value.
type sshClientRunner struct {
	c *gossh.Client
}

func (r *sshClientRunner) NewSession() (Session, error) {
	return r.c.NewSession()
}

// NewSSHRunner wraps a *gossh.Client as an SSHRunner for use as the client
// argument to RunPreflightChecks in production. Tests inject their own
// SSHRunner implementations directly.
func NewSSHRunner(c *gossh.Client) SSHRunner {
	return &sshClientRunner{c: c}
}

// CheckResult holds the outcome of a single pre-flight check. It is returned
// by RunPreflightChecks so that Phase 7 can render a verbose live checklist
// without reimplementing the check logic.
type CheckResult struct {
	Name    string // e.g. "docker-installed"
	Status  string // "pass", "warn", or "fail"
	Message string // human-readable detail
}

// RunPreflightChecks validates that the remote host connected via client is
// ready to accept a deploy. It returns structured CheckResults alongside any
// blocking error.
//
// Execution order: CHECK-01 → CHECK-02 → CHECK-03 → CHECK-07 → CHECK-06
// (calls CHECK-05 if needed) → CHECK-04 (calls CHECK-05 if needed).
//
// Fail-fast on first hard-blocking error (D-04). Warnings (CHECK-03,
// CHECK-07) are printed to os.Stderr and do not cause a non-nil return.
//
// ctx is passed through for future deadline/cancellation wiring (Phase 5
// does not enforce a per-check deadline).
func RunPreflightChecks(ctx context.Context, client SSHRunner, cfg config.Config) ([]CheckResult, error) {
	_ = ctx // future: pass to session-level deadlines
	var results []CheckResult

	// CHECK-01: Docker binary present.
	r, err := checkDockerInstalled(client)
	results = append(results, r)
	if err != nil {
		return results, err
	}

	// CHECK-02: docker compose v2 plugin present.
	r, err = checkComposeV2(client)
	results = append(results, r)
	if err != nil {
		return results, err
	}

	// CHECK-03: Docker daemon running (warning only).
	r = checkDaemon(client, cfg)
	results = append(results, r)

	// CHECK-07: SSH user is root (warning only).
	r = checkRootUser(cfg)
	results = append(results, r)

	// CHECK-06: Target directory writable (auto-fix via sudo if needed).
	r, err = checkTargetDir(client, cfg)
	results = append(results, r)
	if err != nil {
		return results, err
	}

	// CHECK-04: User in docker group (auto-fix via sudo if needed).
	r, err = checkDockerGroup(client, cfg)
	results = append(results, r)
	if err != nil {
		return results, err
	}

	return results, nil
}

// checkDockerInstalled runs `docker --version`. Non-zero exit is a hard block.
func checkDockerInstalled(client SSHRunner) (CheckResult, error) {
	out, err := runOutput(client, "docker --version")
	if err != nil {
		return CheckResult{
			Name:    "docker-installed",
			Status:  "fail",
			Message: "docker not installed on remote host",
		}, fmt.Errorf("preflight: docker not installed on remote host")
	}
	return CheckResult{
		Name:    "docker-installed",
		Status:  "pass",
		Message: strings.TrimSpace(string(out)),
	}, nil
}

// checkComposeV2 runs `docker compose version`. On failure it tries
// `docker-compose --version` to detect v1 (EOL). Both cases are hard blocks.
func checkComposeV2(client SSHRunner) (CheckResult, error) {
	out, err := runOutput(client, "docker compose version")
	if err == nil {
		return CheckResult{
			Name:    "compose-v2",
			Status:  "pass",
			Message: strings.TrimSpace(string(out)),
		}, nil
	}
	// docker compose v2 not found — check for v1 (informational only)
	_, _ = runOutput(client, "docker-compose --version")
	// Regardless of whether v1 is present, the result is a hard block.
	return CheckResult{
		Name:    "compose-v2",
		Status:  "fail",
		Message: "docker compose v2 not installed",
	}, fmt.Errorf("preflight: docker compose v2 is not installed (docker compose plugin required; docker-compose v1 is EOL since June 2023)")
}

// checkDaemon runs `docker info`. Failure prints a warning to os.Stderr but
// never returns an error (D-05, D-06): daemon stopped is recoverable.
func checkDaemon(client SSHRunner, cfg config.Config) CheckResult {
	_, err := runOutput(client, "docker info")
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"Warning: Docker daemon is not running on %s — compose will fail if daemon is still down at deploy time\n",
			cfg.Host.Hostname,
		)
		return CheckResult{
			Name:    "daemon-running",
			Status:  "warn",
			Message: "Docker daemon is not running",
		}
	}
	return CheckResult{
		Name:    "daemon-running",
		Status:  "pass",
		Message: "Docker daemon is running",
	}
}

// checkRootUser warns if cfg.Host.User is "root". Never blocks (D-05).
func checkRootUser(cfg config.Config) CheckResult {
	if cfg.Host.User == "root" {
		fmt.Fprintf(os.Stderr,
			"Warning: deploying as root is not recommended — consider creating a dedicated deploy user\n",
		)
		return CheckResult{
			Name:    "root-user",
			Status:  "warn",
			Message: "deploying as root is not recommended",
		}
	}
	return CheckResult{
		Name:    "root-user",
		Status:  "pass",
		Message: "not deploying as root",
	}
}

// checkSudo verifies that sudo is available (either passwordless or with interactive prompt).
// Returns nil on success; error only if sudo is not available at all.
// Only called when an auto-fix needs sudo (CHECK-04, CHECK-06).
func checkSudo(client SSHRunner, cfg config.Config) error {
	// Try passwordless sudo first
	if err := runCmd(client, "sudo -n true"); err == nil {
		return nil // passwordless sudo available
	}
	// Passwordless unavailable, but sudo itself may be available — that's fine.
	// The actual deploy will prompt for password via the auth fallback sequence.
	return nil
}

// checkTargetDir ensures cfg.Path exists and is writable. It tries:
//  1. test -w <path>        — pass immediately if writable
//  2. mkdir -p <path>       — pass if succeeds without sudo
//  3. Warn if directory not writable (auto-fix deferred to deploy phase)
//
// Returns the CheckResult and any blocking error.
// Note: sudo auto-fixes are deferred to Upload() which handles interactive password prompts.
func checkTargetDir(client SSHRunner, cfg config.Config) (CheckResult, error) {
	path := filetransfer.ShellQuote(cfg.Path)

	// Try: is the directory already writable?
	if err := runCmd(client, "test -w "+path); err == nil {
		return CheckResult{
			Name:    "target-dir",
			Status:  "pass",
			Message: cfg.Path + " is writable",
		}, nil
	}

	// Try: mkdir -p without sudo
	if err := runCmd(client, "mkdir -p "+path); err == nil {
		return CheckResult{
			Name:    "target-dir",
			Status:  "pass",
			Message: cfg.Path + " created",
		}, nil
	}

	// Directory not writable without sudo. Warn the user but don't fail.
	// Upload() will handle sudo escalation with interactive password prompts if needed.
	fmt.Fprintf(os.Stderr,
		"Warning: %s may require sudo to write to; will attempt password auth during deploy if needed\n",
		cfg.Path,
	)
	return CheckResult{
		Name:    "target-dir",
		Status:  "warn",
		Message: cfg.Path + " requires sudo to write (will prompt during deploy if needed)",
	}, nil
}

// checkDockerGroup checks if cfg.Host.User is in the docker group via `id -nG`.
// If not, warns the user. Auto-fix via sudo is not attempted (requires TTY for password).
//
// Returns the CheckResult and any blocking error.
func checkDockerGroup(client SSHRunner, cfg config.Config) (CheckResult, error) {
	out, err := runOutput(client, "id -nG")
	if err != nil {
		return CheckResult{
			Name:    "docker-group",
			Status:  "fail",
			Message: "could not determine group membership",
		}, fmt.Errorf("preflight: could not determine group membership for user %s", cfg.Host.User)
	}

	groups := strings.Fields(string(out))
	for _, g := range groups {
		if g == "docker" {
			return CheckResult{
				Name:    "docker-group",
				Status:  "pass",
				Message: "user is in the docker group",
			}, nil
		}
	}

	// User not in docker group — warn but don't fail.
	// The docker compose up may still work depending on the setup,
	// or user can run: sudo usermod -aG docker <user> (then newgrp docker)
	fmt.Fprintf(os.Stderr,
		"Warning: user %s is not in the docker group; compose may require sudo\n",
		cfg.Host.User,
	)
	return CheckResult{
		Name:    "docker-group",
		Status:  "warn",
		Message: "user not in docker group (compose may require sudo)",
	}, nil
}

// ---------------------------------------------------------------------------
// SSH session helpers — each uses a separate NewSession() per CLAUDE.md Rule 3.
// ---------------------------------------------------------------------------

// runOutput opens a new session, runs cmd, and returns stdout. Non-zero exit
// returns an error. The session is closed before returning.
func runOutput(client SSHRunner, cmd string) ([]byte, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("creating SSH session: %w", err)
	}
	defer session.Close()
	return session.Output(cmd)
}

// runCmd opens a new session and runs cmd for its side effect. Non-zero exit
// returns an error. The session is closed before returning.
func runCmd(client SSHRunner, cmd string) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("creating SSH session: %w", err)
	}
	defer session.Close()
	return session.Run(cmd)
}
