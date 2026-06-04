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
	"path"
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
	sess, err := r.c.NewSession()
	if err != nil {
		return nil, fmt.Errorf("creating SSH session: %w", err)
	}
	return sess, nil
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
// Execution order: CHECK-01 → CHECK-02 → CHECK-03 → CHECK-07 → CHECK-05 → CHECK-06 → CHECK-04.
//
// Fail-fast on first hard-blocking error (D-04). Warnings (CHECK-03,
// CHECK-05, CHECK-07) are printed to os.Stderr and do not cause a non-nil return.
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

	// CHECK-05: Passwordless sudo available (warning only — deploy falls back to
	// interactive password prompt if not configured).
	r = checkPasswordlessSudo(client)
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

// checkPasswordlessSudo probes whether the remote user can run a command via
// sudo without a password (`sudo -n true`). A warning is emitted when
// passwordless sudo is not available — this is never a hard block because the
// deploy falls back to an interactive password prompt (D-07).
func checkPasswordlessSudo(client SSHRunner) CheckResult {
	if err := runCmd(client, "sudo -n true"); err == nil {
		return CheckResult{
			Name:    "passwordless-sudo",
			Status:  "pass",
			Message: "passwordless sudo available",
		}
	}
	fmt.Fprintf(os.Stderr,
		"Warning: passwordless sudo not configured — deploy will prompt for sudo password if target directory requires elevation\n",
	)
	return CheckResult{
		Name:    "passwordless-sudo",
		Status:  "warn",
		Message: "passwordless sudo not configured (will prompt if needed)",
	}
}

// checkTargetDir ensures cfg.Path exists and is writable. It tries:
//  1. test -w <path>              — pass immediately if already writable
//  2. mkdir -p <path> && test -w  — pass if creation succeeds AND dir is writable
//  3. sudo -n mkdir -p <path>     — pass if passwordless sudo can create it
//  4. Warn if no path is accessible (auto-fix deferred to deploy phase)
//
// Step 2 combines mkdir with an explicit writability check because mkdir -p
// returns 0 for directories that already exist — even if the caller cannot
// write to them (e.g. chmod 000). Without the subsequent test -w, an
// existing but inaccessible directory would be falsely reported as "pass".
//
// Step 3 covers users with passwordless sudo (NOPASSWD: ALL). These users
// cannot create directories in /opt directly but can do so via sudo -n.
//
// Returns the CheckResult and any blocking error.
func checkTargetDir(client SSHRunner, cfg config.Config) (CheckResult, error) {
	targetQ := filetransfer.ShellQuote(cfg.Path)

	// WR-05: The atomic swap (mv remoteBase remoteBase-old) operates on entries
	// WITHIN the parent directory — it requires the PARENT to be writable, not
	// the target dir itself. A user-owned /opt/myapp passes "test -w /opt/myapp"
	// but "mv /opt/myapp …" still fails because /opt is root-owned. Align the
	// preflight check with Upload()'s probe (which uses path.Dir(remoteBase)).
	// path (not filepath) is used — the remote is always Linux.
	parentPath := path.Dir(cfg.Path)
	parentQ := filetransfer.ShellQuote(parentPath)

	// Try: is the target directory AND its parent already writable?
	if err := runCmd(client, "test -w "+targetQ+" && test -w "+parentQ); err == nil {
		return CheckResult{
			Name:    "target-dir",
			Status:  "pass",
			Message: cfg.Path + " is writable",
		}, nil
	}

	// Try: mkdir -p without sudo, then verify actual writability (both target and parent).
	// mkdir -p succeeds for existing directories regardless of their permissions,
	// so a subsequent test -w is required to confirm real write access.
	if err := runCmd(client, "mkdir -p "+targetQ+" && test -w "+targetQ+" && test -w "+parentQ); err == nil {
		return CheckResult{
			Name:    "target-dir",
			Status:  "pass",
			Message: cfg.Path + " created",
		}, nil
	}

	// Try: passwordless sudo (sudo -n). This succeeds for users with NOPASSWD: ALL
	// configured, indicating Upload() will not need to prompt for a password.
	if err := runCmd(client, "sudo -n mkdir -p "+targetQ); err == nil {
		return CheckResult{
			Name:    "target-dir",
			Status:  "pass",
			Message: cfg.Path + " created via sudo",
		}, nil
	}

	// Directory not accessible without a password. Warn the user but don't fail.
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

	if cfg.Verbose {
		sudoLOut, sudoLErr := runOutput(client, "sudo -l")
		if sudoLErr == nil {
			fmt.Fprintf(os.Stderr, "[sudo -l]\n%s\n", strings.TrimSpace(string(sudoLOut)))
		}
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
	defer session.Close() //nolint:errcheck
	out, err := session.Output(cmd)
	if err != nil {
		return nil, fmt.Errorf("running SSH command %q: %w", cmd, err)
	}
	return out, nil
}

// runCmd opens a new session and runs cmd for its side effect. Non-zero exit
// returns an error. The session is closed before returning.
func runCmd(client SSHRunner, cmd string) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("creating SSH session: %w", err)
	}
	defer session.Close() //nolint:errcheck
	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("running SSH command %q: %w", cmd, err)
	}
	return nil
}
