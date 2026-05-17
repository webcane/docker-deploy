package preflight_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	gossh "golang.org/x/crypto/ssh"

	"github.com/mniedre/docker-deploy/internal/config"
	"github.com/mniedre/docker-deploy/internal/preflight"
)

// ---------------------------------------------------------------------------
// Fake SSH client infrastructure
// ---------------------------------------------------------------------------

// fakeCmd defines the behaviour of a single fake SSH exec invocation.
type fakeCmd struct {
	match    string // substring the command must contain
	output   []byte // stdout to return from Output()
	exitCode int    // 0 = success; non-zero = ExitError
}

// fakeSSHClient is a minimal stand-in for preflight.SSHRunner.
type fakeSSHClient struct {
	cmds    []fakeCmd
	matched []string // commands actually matched (for assertions)
}

func (f *fakeSSHClient) NewSession() (preflight.Session, error) {
	return &fakeSession{parent: f}, nil
}

type fakeSession struct {
	parent *fakeSSHClient
}

func (s *fakeSession) Output(cmd string) ([]byte, error) {
	for i, fc := range s.parent.cmds {
		if strings.Contains(cmd, fc.match) {
			s.parent.matched = append(s.parent.matched, cmd)
			s.parent.cmds = append(s.parent.cmds[:i], s.parent.cmds[i+1:]...)
			if fc.exitCode != 0 {
				return nil, &gossh.ExitError{Waitmsg: gossh.Waitmsg{}}
			}
			return fc.output, nil
		}
	}
	// No match → success with empty output
	s.parent.matched = append(s.parent.matched, cmd)
	return nil, nil
}

func (s *fakeSession) Run(cmd string) error {
	_, err := s.Output(cmd)
	return err
}

func (s *fakeSession) Close() error { return nil }

// newClient builds a fakeSSHClient pre-loaded with the given responses.
func newClient(cmds ...fakeCmd) *fakeSSHClient {
	return &fakeSSHClient{cmds: cmds}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func defaultCfg() config.Config {
	return config.Config{
		Host: config.Host{User: "deploy", Hostname: "example.com", Port: 22},
		Path: "/opt/myapp",
	}
}

func captureStderr(fn func()) string {
	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

// ---------------------------------------------------------------------------
// CHECK-01: docker --version
// ---------------------------------------------------------------------------

func TestCheck01_DockerNotInstalled(t *testing.T) {
	client := newClient(
		fakeCmd{match: "docker --version", exitCode: 1},
	)
	_, err := preflight.RunPreflightChecks(context.Background(), client, defaultCfg())
	if err == nil {
		t.Fatal("expected error when docker --version fails, got nil")
	}
	if !strings.Contains(err.Error(), "docker not installed") {
		t.Errorf("error %q does not contain 'docker not installed'", err.Error())
	}
}

func TestCheck01_DockerInstalled(t *testing.T) {
	client := newClient(
		fakeCmd{match: "docker --version", output: []byte("Docker version 25.0.3, build abc123")},
		fakeCmd{match: "docker compose version", output: []byte("Docker Compose version v2.24.0")},
		fakeCmd{match: "docker info", output: []byte("Containers: 0")},
	)
	_, err := preflight.RunPreflightChecks(context.Background(), client, defaultCfg())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// CHECK-02: docker compose v2
// ---------------------------------------------------------------------------

func TestCheck02_ComposeV2NotInstalled_V1Detected(t *testing.T) {
	client := newClient(
		fakeCmd{match: "docker --version", output: []byte("Docker version 25.0.3")},
		fakeCmd{match: "docker compose version", exitCode: 1},
		fakeCmd{match: "docker-compose --version", output: []byte("docker-compose version 1.29.2, build 5becea4c")},
	)
	_, err := preflight.RunPreflightChecks(context.Background(), client, defaultCfg())
	if err == nil {
		t.Fatal("expected error for compose v2 not installed, got nil")
	}
	if !strings.Contains(err.Error(), "docker compose v2 is not installed") {
		t.Errorf("error %q does not mention 'docker compose v2 is not installed'", err.Error())
	}
	if !strings.Contains(err.Error(), "EOL") {
		t.Errorf("error %q does not mention EOL for v1", err.Error())
	}
}

func TestCheck02_ComposeV2Installed(t *testing.T) {
	client := newClient(
		fakeCmd{match: "docker --version", output: []byte("Docker version 25.0.3")},
		fakeCmd{match: "docker compose version", output: []byte("Docker Compose version v2.24.0")},
		fakeCmd{match: "docker info", output: []byte("Containers: 0")},
	)
	_, err := preflight.RunPreflightChecks(context.Background(), client, defaultCfg())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// CHECK-03: docker info (daemon running) — warning only
// ---------------------------------------------------------------------------

func TestCheck03_DaemonNotRunning_WarningOnly(t *testing.T) {
	client := newClient(
		fakeCmd{match: "docker --version", output: []byte("Docker version 25.0.3")},
		fakeCmd{match: "docker compose version", output: []byte("Docker Compose version v2.24.0")},
		fakeCmd{match: "docker info", exitCode: 1},
	)
	var err error
	stderr := captureStderr(func() {
		_, err = preflight.RunPreflightChecks(context.Background(), client, defaultCfg())
	})
	if err != nil {
		t.Errorf("CHECK-03 must not return error, got: %v", err)
	}
	if !strings.Contains(stderr, "Docker daemon is not running") {
		t.Errorf("expected stderr warning about daemon, got: %q", stderr)
	}
}

// ---------------------------------------------------------------------------
// CHECK-04: docker group membership
// ---------------------------------------------------------------------------

func TestCheck04_UserAlreadyInDockerGroup(t *testing.T) {
	client := newClient(
		fakeCmd{match: "docker --version", output: []byte("Docker version 25.0.3")},
		fakeCmd{match: "docker compose version", output: []byte("Docker Compose version v2.24.0")},
		fakeCmd{match: "docker info", output: []byte("Containers: 0")},
		fakeCmd{match: "id -nG", output: []byte("sudo docker adm")},
		fakeCmd{match: "test -w", output: nil, exitCode: 0},
	)
	_, err := preflight.RunPreflightChecks(context.Background(), client, defaultCfg())
	if err != nil {
		t.Errorf("unexpected error when user already in docker group: %v", err)
	}
}

func TestCheck04_UserNotInDockerGroup_SudoUsermodSuccess(t *testing.T) {
	client := newClient(
		fakeCmd{match: "docker --version", output: []byte("Docker version 25.0.3")},
		fakeCmd{match: "docker compose version", output: []byte("Docker Compose version v2.24.0")},
		fakeCmd{match: "docker info", output: []byte("Containers: 0")},
		fakeCmd{match: "test -w", output: nil, exitCode: 0},
		fakeCmd{match: "id -nG", output: []byte("sudo adm")}, // no "docker"
		fakeCmd{match: "sudo -n true", output: nil, exitCode: 0},
		fakeCmd{match: "sudo usermod", output: nil, exitCode: 0},
	)
	_, err := preflight.RunPreflightChecks(context.Background(), client, defaultCfg())
	if err != nil {
		t.Errorf("unexpected error after sudo usermod success: %v", err)
	}
}

func TestCheck04_UserNotInDockerGroup_SudoUsermodFails(t *testing.T) {
	client := newClient(
		fakeCmd{match: "docker --version", output: []byte("Docker version 25.0.3")},
		fakeCmd{match: "docker compose version", output: []byte("Docker Compose version v2.24.0")},
		fakeCmd{match: "docker info", output: []byte("Containers: 0")},
		fakeCmd{match: "test -w", output: nil, exitCode: 0},
		fakeCmd{match: "id -nG", output: []byte("sudo adm")}, // no "docker"
		fakeCmd{match: "sudo -n true", output: nil, exitCode: 0},
		fakeCmd{match: "sudo usermod", output: nil, exitCode: 1},
	)
	_, err := preflight.RunPreflightChecks(context.Background(), client, defaultCfg())
	if err == nil {
		t.Fatal("expected error when sudo usermod fails, got nil")
	}
	if !strings.Contains(err.Error(), "user not in docker group") {
		t.Errorf("error %q does not contain 'user not in docker group'", err.Error())
	}
	// Error message must include the fix command
	if !strings.Contains(err.Error(), "sudo usermod -aG docker") {
		t.Errorf("error %q does not include fix command", err.Error())
	}
}

// ---------------------------------------------------------------------------
// CHECK-05: sudo access — conditional
// ---------------------------------------------------------------------------

func TestCheck05_NoPasswordlessSudo_ReturnsError(t *testing.T) {
	client := newClient(
		fakeCmd{match: "docker --version", output: []byte("Docker version 25.0.3")},
		fakeCmd{match: "docker compose version", output: []byte("Docker Compose version v2.24.0")},
		fakeCmd{match: "docker info", output: []byte("Containers: 0")},
		fakeCmd{match: "test -w", exitCode: 1},            // dir not writable → needs sudo
		fakeCmd{match: "mkdir -p", exitCode: 1},           // mkdir fails → needs sudo
		fakeCmd{match: "sudo -n true", exitCode: 1},        // no passwordless sudo
	)
	_, err := preflight.RunPreflightChecks(context.Background(), client, defaultCfg())
	if err == nil {
		t.Fatal("expected error when sudo not available, got nil")
	}
	if !strings.Contains(err.Error(), "no passwordless sudo") {
		t.Errorf("error %q does not contain 'no passwordless sudo'", err.Error())
	}
}

func TestCheck05_NotExecutedWhenNoSudoNeeded(t *testing.T) {
	// User is in docker group + dir is writable → CHECK-05 should never be called
	client := newClient(
		fakeCmd{match: "docker --version", output: []byte("Docker version 25.0.3")},
		fakeCmd{match: "docker compose version", output: []byte("Docker Compose version v2.24.0")},
		fakeCmd{match: "docker info", output: []byte("Containers: 0")},
		fakeCmd{match: "test -w", exitCode: 0},            // writable
		fakeCmd{match: "id -nG", output: []byte("docker deploy")},
	)
	_, err := preflight.RunPreflightChecks(context.Background(), client, defaultCfg())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// Verify sudo -n true was NOT called
	for _, cmd := range client.matched {
		if strings.Contains(cmd, "sudo -n true") {
			t.Errorf("CHECK-05 (sudo -n true) should not have been executed but was: %q", cmd)
		}
	}
}

// ---------------------------------------------------------------------------
// CHECK-06: target directory writable
// ---------------------------------------------------------------------------

func TestCheck06_DirWritable(t *testing.T) {
	client := newClient(
		fakeCmd{match: "docker --version", output: []byte("Docker version 25.0.3")},
		fakeCmd{match: "docker compose version", output: []byte("Docker Compose version v2.24.0")},
		fakeCmd{match: "docker info", output: []byte("Containers: 0")},
		fakeCmd{match: "test -w", exitCode: 0},
		fakeCmd{match: "id -nG", output: []byte("docker deploy")},
	)
	_, err := preflight.RunPreflightChecks(context.Background(), client, defaultCfg())
	if err != nil {
		t.Errorf("unexpected error when dir is writable: %v", err)
	}
}

func TestCheck06_DirNotWritable_MkdirSucceeds(t *testing.T) {
	client := newClient(
		fakeCmd{match: "docker --version", output: []byte("Docker version 25.0.3")},
		fakeCmd{match: "docker compose version", output: []byte("Docker Compose version v2.24.0")},
		fakeCmd{match: "docker info", output: []byte("Containers: 0")},
		fakeCmd{match: "test -w", exitCode: 1},    // not writable
		fakeCmd{match: "mkdir -p", exitCode: 0},   // mkdir succeeds without sudo
		fakeCmd{match: "id -nG", output: []byte("docker deploy")},
	)
	_, err := preflight.RunPreflightChecks(context.Background(), client, defaultCfg())
	if err != nil {
		t.Errorf("unexpected error when mkdir succeeds: %v", err)
	}
}

func TestCheck06_DirNotWritable_NeedsSudoMkdir(t *testing.T) {
	client := newClient(
		fakeCmd{match: "docker --version", output: []byte("Docker version 25.0.3")},
		fakeCmd{match: "docker compose version", output: []byte("Docker Compose version v2.24.0")},
		fakeCmd{match: "docker info", output: []byte("Containers: 0")},
		fakeCmd{match: "test -w", exitCode: 1},          // not writable
		fakeCmd{match: "mkdir -p", exitCode: 1},          // mkdir without sudo fails
		fakeCmd{match: "sudo -n true", exitCode: 0},      // sudo available
		fakeCmd{match: "sudo mkdir -p", exitCode: 0},     // sudo mkdir succeeds
		fakeCmd{match: "sudo chown", exitCode: 0},        // sudo chown succeeds
		fakeCmd{match: "id -nG", output: []byte("docker deploy")},
	)
	_, err := preflight.RunPreflightChecks(context.Background(), client, defaultCfg())
	if err != nil {
		t.Errorf("unexpected error when sudo mkdir/chown succeeds: %v", err)
	}
}

// ---------------------------------------------------------------------------
// CHECK-07: root user warning — never blocks
// ---------------------------------------------------------------------------

func TestCheck07_RootUser_WarningOnly(t *testing.T) {
	cfg := defaultCfg()
	cfg.Host.User = "root"

	client := newClient(
		fakeCmd{match: "docker --version", output: []byte("Docker version 25.0.3")},
		fakeCmd{match: "docker compose version", output: []byte("Docker Compose version v2.24.0")},
		fakeCmd{match: "docker info", output: []byte("Containers: 0")},
		fakeCmd{match: "test -w", exitCode: 0},
		fakeCmd{match: "id -nG", output: []byte("root docker")},
	)
	var err error
	stderr := captureStderr(func() {
		_, err = preflight.RunPreflightChecks(context.Background(), client, cfg)
	})
	if err != nil {
		t.Errorf("CHECK-07 must not return error for root user, got: %v", err)
	}
	if !strings.Contains(stderr, "deploying as root is not recommended") {
		t.Errorf("expected root warning in stderr, got: %q", stderr)
	}
}

// ---------------------------------------------------------------------------
// CheckResult struct tests
// ---------------------------------------------------------------------------

func TestCheckResult_Structure(t *testing.T) {
	// Ensure CheckResult type is exported with the right fields
	cr := preflight.CheckResult{
		Name:    "docker-installed",
		Status:  "pass",
		Message: "Docker version 25.0.3",
	}
	if cr.Name == "" || cr.Status == "" {
		t.Error("CheckResult fields not accessible")
	}
}

func TestRunPreflightChecks_ReturnsCheckResults(t *testing.T) {
	client := newClient(
		fakeCmd{match: "docker --version", output: []byte("Docker version 25.0.3")},
		fakeCmd{match: "docker compose version", output: []byte("Docker Compose version v2.24.0")},
		fakeCmd{match: "docker info", output: []byte("Containers: 0")},
		fakeCmd{match: "test -w", exitCode: 0},
		fakeCmd{match: "id -nG", output: []byte("docker deploy")},
	)
	results, err := preflight.RunPreflightChecks(context.Background(), client, defaultCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) == 0 {
		t.Error("RunPreflightChecks returned empty results slice")
	}
	for _, r := range results {
		if r.Status != "pass" && r.Status != "warn" && r.Status != "fail" {
			t.Errorf("result %q has invalid status %q", r.Name, r.Status)
		}
	}
}

// ---------------------------------------------------------------------------
// Compile-time interface check
// ---------------------------------------------------------------------------

// Ensure NewSSHRunner wraps *gossh.Client into SSHRunner for production use.
// This is a compile-time guard — if NewSSHRunner changes signature or the
// adapter no longer satisfies SSHRunner, this file will not compile.
func TestSSHRunnerInterface_GosshClientSatisfies(t *testing.T) {
	// NewSSHRunner(*gossh.Client) returns SSHRunner — compile-time check only.
	// We use a nil pointer cast to avoid dialling a real SSH server.
	var client *gossh.Client
	var _ preflight.SSHRunner = preflight.NewSSHRunner(client)
	_ = fmt.Sprintf // suppress unused import
}
