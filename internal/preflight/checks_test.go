package preflight_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	gossh "golang.org/x/crypto/ssh"

	"github.com/webcane/docker-deploy/internal/config"
	"github.com/webcane/docker-deploy/internal/preflight"
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
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
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
	// User not in docker group → returns warning (not error), allows deploy to proceed
	client := newClient(
		fakeCmd{match: "docker --version", output: []byte("Docker version 25.0.3")},
		fakeCmd{match: "docker compose version", output: []byte("Docker Compose version v2.24.0")},
		fakeCmd{match: "docker info", output: []byte("Containers: 0")},
		fakeCmd{match: "test -w", output: nil, exitCode: 0},
		fakeCmd{match: "id -nG", output: []byte("sudo adm")}, // no "docker"
	)
	results, err := preflight.RunPreflightChecks(context.Background(), client, defaultCfg())
	if err != nil {
		t.Fatalf("expected nil error (warning only), got %v", err)
	}
	// Find the docker-group check result
	var dockerGroupResult *preflight.CheckResult
	for i := range results {
		if results[i].Name == "docker-group" {
			dockerGroupResult = &results[i]
			break
		}
	}
	if dockerGroupResult == nil {
		t.Fatal("docker-group check result not found")
	}
	if dockerGroupResult.Status != "warn" {
		t.Errorf("expected status 'warn', got %q", dockerGroupResult.Status)
	}
}

// ---------------------------------------------------------------------------
// CHECK-04 verbose sudo -l tests
// ---------------------------------------------------------------------------

// TestCheckDockerGroup_SudoL_VerboseShown: when cfg.Verbose=true and "sudo -l"
// returns exit 0, stderr contains "[sudo -l]\n<output>".
func TestCheckDockerGroup_SudoL_VerboseShown(t *testing.T) {
	client := newClient(
		fakeCmd{match: "docker --version", output: []byte("Docker version 25.0.3")},
		fakeCmd{match: "docker compose version", output: []byte("Docker Compose version v2.24.0")},
		fakeCmd{match: "docker info", output: []byte("Containers: 0")},
		fakeCmd{match: "test -w", exitCode: 0},
		fakeCmd{match: "id -nG", output: []byte("sudo docker adm")},
		fakeCmd{match: "sudo -l", output: []byte("User root may run the following commands on host:\n    (ALL) NOPASSWD: ALL\n")},
	)
	cfg := defaultCfg()
	cfg.Verbose = true
	var err error
	stderr := captureStderr(func() {
		_, err = preflight.RunPreflightChecks(context.Background(), client, cfg)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr, "[sudo -l]") {
		t.Errorf("expected '[sudo -l]' in stderr, got: %q", stderr)
	}
	if !strings.Contains(stderr, "User root may run") {
		t.Errorf("expected sudo -l output in stderr, got: %q", stderr)
	}
}

// TestCheckDockerGroup_SudoL_FailureSilenced: when cfg.Verbose=true and "sudo -l"
// returns exit 1 (error), no "[sudo -l]" appears in stderr and checkDockerGroup
// returns normally.
func TestCheckDockerGroup_SudoL_FailureSilenced(t *testing.T) {
	client := newClient(
		fakeCmd{match: "docker --version", output: []byte("Docker version 25.0.3")},
		fakeCmd{match: "docker compose version", output: []byte("Docker Compose version v2.24.0")},
		fakeCmd{match: "docker info", output: []byte("Containers: 0")},
		fakeCmd{match: "test -w", exitCode: 0},
		fakeCmd{match: "id -nG", output: []byte("sudo docker adm")},
		fakeCmd{match: "sudo -l", exitCode: 1}, // sudo -l fails
	)
	cfg := defaultCfg()
	cfg.Verbose = true
	var err error
	stderr := captureStderr(func() {
		_, err = preflight.RunPreflightChecks(context.Background(), client, cfg)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(stderr, "[sudo -l]") {
		t.Errorf("expected no '[sudo -l]' in stderr when sudo -l fails, got: %q", stderr)
	}
}

// TestCheckDockerGroup_SudoL_NotVerbose: when cfg.Verbose=false, "sudo -l" is
// never invoked (mock session receives no "sudo -l" command).
func TestCheckDockerGroup_SudoL_NotVerbose(t *testing.T) {
	client := newClient(
		fakeCmd{match: "docker --version", output: []byte("Docker version 25.0.3")},
		fakeCmd{match: "docker compose version", output: []byte("Docker Compose version v2.24.0")},
		fakeCmd{match: "docker info", output: []byte("Containers: 0")},
		fakeCmd{match: "test -w", exitCode: 0},
		fakeCmd{match: "id -nG", output: []byte("sudo docker adm")},
	)
	cfg := defaultCfg()
	cfg.Verbose = false
	_, err := preflight.RunPreflightChecks(context.Background(), client, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, cmd := range client.matched {
		if strings.Contains(cmd, "sudo -l") {
			t.Errorf("sudo -l should not be invoked when cfg.Verbose=false, but was called: %q", cmd)
		}
	}
}

// ---------------------------------------------------------------------------
// CHECK-05: sudo access — conditional
// ---------------------------------------------------------------------------

func TestCheck05_NoPasswordlessSudo_ReturnsWarn(t *testing.T) {
	// No passwordless sudo available → CHECK-05 warns; target-dir also warns.
	// Both must be non-blocking (no error returned).
	client := newClient(
		fakeCmd{match: "docker --version", output: []byte("Docker version 25.0.3")},
		fakeCmd{match: "docker compose version", output: []byte("Docker Compose version v2.24.0")},
		fakeCmd{match: "docker info", output: []byte("Containers: 0")},
		fakeCmd{match: "sudo -n true", exitCode: 1},        // CHECK-05: no passwordless sudo
		fakeCmd{match: "test -w", exitCode: 1},             // dir not writable
		fakeCmd{match: "mkdir -p", exitCode: 1},            // mkdir without sudo fails
		fakeCmd{match: "sudo -n mkdir -p", exitCode: 1},    // sudo mkdir also fails
		fakeCmd{match: "id -nG", output: []byte("docker")}, // user in docker group
	)
	results, err := preflight.RunPreflightChecks(context.Background(), client, defaultCfg())
	if err != nil {
		t.Fatalf("expected nil error (warning only), got %v", err)
	}

	// CHECK-05 result must be present and warn
	var check05Result *preflight.CheckResult
	for i := range results {
		if results[i].Name == "passwordless-sudo" {
			check05Result = &results[i]
			break
		}
	}
	if check05Result == nil {
		t.Fatal("passwordless-sudo check result not found")
	}
	if check05Result.Status != "warn" {
		t.Errorf("expected CHECK-05 status 'warn', got %q", check05Result.Status)
	}

	// target-dir result must also be warn (sudo mkdir also failed)
	var targetDirResult *preflight.CheckResult
	for i := range results {
		if results[i].Name == "target-dir" {
			targetDirResult = &results[i]
			break
		}
	}
	if targetDirResult == nil {
		t.Fatal("target-dir check result not found")
	}
	if targetDirResult.Status != "warn" {
		t.Errorf("expected target-dir status 'warn', got %q", targetDirResult.Status)
	}
}

func TestCheck05_AlwaysRuns_PassWhenPasswordlessSudoAvailable(t *testing.T) {
	// CHECK-05 is unconditional — it must always run and report pass when
	// passwordless sudo is available, regardless of directory writability.
	client := newClient(
		fakeCmd{match: "docker --version", output: []byte("Docker version 25.0.3")},
		fakeCmd{match: "docker compose version", output: []byte("Docker Compose version v2.24.0")},
		fakeCmd{match: "docker info", output: []byte("Containers: 0")},
		fakeCmd{match: "sudo -n true", exitCode: 0}, // CHECK-05: passwordless sudo available
		fakeCmd{match: "test -w", exitCode: 0},      // dir writable
		fakeCmd{match: "id -nG", output: []byte("docker deploy")},
	)
	results, err := preflight.RunPreflightChecks(context.Background(), client, defaultCfg())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify CHECK-05 ran and is in the results
	var check05Result *preflight.CheckResult
	for i := range results {
		if results[i].Name == "passwordless-sudo" {
			check05Result = &results[i]
			break
		}
	}
	if check05Result == nil {
		t.Fatal("passwordless-sudo check result not found — CHECK-05 must always run")
	}
	if check05Result.Status != "pass" {
		t.Errorf("expected CHECK-05 status 'pass' when sudo -n true succeeds, got %q", check05Result.Status)
	}

	// Verify sudo -n true WAS called (unconditional)
	found := false
	for _, cmd := range client.matched {
		if strings.Contains(cmd, "sudo -n true") {
			found = true
			break
		}
	}
	if !found {
		t.Error("CHECK-05 (sudo -n true) must always run, but was not called")
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
		fakeCmd{match: "test -w", exitCode: 1},  // not writable
		fakeCmd{match: "mkdir -p", exitCode: 0}, // mkdir succeeds without sudo
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
		fakeCmd{match: "test -w", exitCode: 1},          // dir not writable
		fakeCmd{match: "mkdir -p", exitCode: 1},         // mkdir without sudo fails
		fakeCmd{match: "sudo -n mkdir -p", exitCode: 0}, // passwordless sudo mkdir succeeds → pass
		fakeCmd{match: "id -nG", output: []byte("docker deploy")},
	)
	_, err := preflight.RunPreflightChecks(context.Background(), client, defaultCfg())
	if err != nil {
		t.Errorf("unexpected error when sudo -n mkdir succeeds: %v", err)
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
func TestSSHRunnerInterface_GosshClientSatisfies(_ *testing.T) {
	// NewSSHRunner(*gossh.Client) returns SSHRunner — compile-time check only.
	// We use a nil pointer cast to avoid dialling a real SSH server.
	var client *gossh.Client
	var _ = preflight.NewSSHRunner(client)
	_ = fmt.Sprintf // suppress unused import
}
