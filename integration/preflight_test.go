//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/webcane/docker-deploy/internal/config"
	"github.com/webcane/docker-deploy/internal/preflight"
)

// findResult searches a CheckResult slice by Name and returns a pointer to the
// matching result, or nil if not found. Using a pointer avoids copying the struct
// and lets callers detect missing results vs zero-value results.
func findResult(results []preflight.CheckResult, name string) *preflight.CheckResult {
	for i := range results {
		if results[i].Name == name {
			return &results[i]
		}
	}
	return nil
}

// defaultCfg constructs a config.Config for Container B with the given user.
// Path is set to a predictable per-user path to avoid cross-test interference.
func defaultCfg(user string) config.Config {
	return config.Config{
		Host: config.Host{User: user, Hostname: sshB.host, Port: sshB.port},
		Path: "/opt/testapp-preflight",
	}
}

// dialContainerA dials Container A (SSH-only, no Docker) as "testuser" using
// password authentication. Used exclusively by CHECK-01, CHECK-02, and CHECK-03
// fail scenarios where Docker is absent.
//
// Container A is configured with USER_NAME: "testuser" and USER_PASSWORD: "testpass"
// via startSSHContainer() in helpers_test.go.
func dialContainerA(t *testing.T) *gossh.Client {
	t.Helper()
	khFile := seedKnownHosts(t, sshA.host, sshA.port, sshA.hostKey)
	kh, err := knownhosts.New(khFile)
	if err != nil {
		t.Fatalf("knownhosts.New: %v", err)
	}
	clientCfg := &gossh.ClientConfig{
		User:            "testuser",
		Auth:            []gossh.AuthMethod{gossh.Password("testpass")},
		HostKeyCallback: kh,
		Timeout:         15 * time.Second,
	}
	client, err := gossh.Dial("tcp", fmt.Sprintf("%s:%d", sshA.host, sshA.port), clientCfg)
	if err != nil {
		t.Fatalf("dial Container A: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

// ---------------------------------------------------------------------------
// CHECK-01: docker --version
// ---------------------------------------------------------------------------

// TestPreflight_CHECK01_DockerInstalled_Pass verifies that docker binary detection
// succeeds when connecting to Container B (which has Docker installed).
func TestPreflight_CHECK01_DockerInstalled_Pass(t *testing.T) {
	client := dialContainer(t, "sshuser")
	runner := preflight.NewSSHRunner(client)
	cfg := defaultCfg("sshuser")

	results, err := preflight.RunPreflightChecks(context.Background(), runner, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := findResult(results, "docker-installed")
	if r == nil {
		t.Fatal("docker-installed CheckResult not found")
	}
	if r.Status != "pass" {
		t.Errorf("expected Status='pass' for docker-installed, got %q (message: %s)", r.Status, r.Message)
	}
}

// TestPreflight_CHECK01_DockerInstalled_Fail verifies that docker binary detection
// fails and returns a non-nil error when connecting to Container A (no Docker).
func TestPreflight_CHECK01_DockerInstalled_Fail(t *testing.T) {
	client := dialContainerA(t)
	runner := preflight.NewSSHRunner(client)
	cfg := config.Config{
		Host: config.Host{User: "testuser", Hostname: sshA.host, Port: sshA.port},
		Path: "/opt/testapp-preflight",
	}

	_, err := preflight.RunPreflightChecks(context.Background(), runner, cfg)
	if err == nil {
		t.Fatal("expected error when docker is not installed, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "docker") {
		t.Errorf("error %q does not mention 'docker'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// CHECK-02: docker compose v2
// ---------------------------------------------------------------------------

// TestPreflight_CHECK02_ComposeV2_Pass verifies that compose v2 detection succeeds
// when connecting to Container B (which has docker compose v2 installed).
func TestPreflight_CHECK02_ComposeV2_Pass(t *testing.T) {
	client := dialContainer(t, "sshuser")
	runner := preflight.NewSSHRunner(client)
	cfg := defaultCfg("sshuser")

	results, err := preflight.RunPreflightChecks(context.Background(), runner, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := findResult(results, "compose-v2")
	if r == nil {
		t.Fatal("compose-v2 CheckResult not found")
	}
	if r.Status != "pass" {
		t.Errorf("expected Status='pass' for compose-v2, got %q (message: %s)", r.Status, r.Message)
	}
}

// TestPreflight_CHECK02_ComposeV2_Fail verifies that the check chain returns an error
// when connecting to Container A (no Docker, no compose v2).
//
// NOTE: This test uses CHECK-01 failure as a proxy for CHECK-02 failure. Container A
// has no Docker installed, so the check chain terminates at CHECK-01 before CHECK-02
// is reached. The proxy is valid because compose v2 cannot pass without docker
// installed — if docker is absent, compose v2 is necessarily absent. Isolated
// CHECK-02 fail scenarios (e.g. docker present but only compose v1 installed) can
// be added in a future phase if needed.
func TestPreflight_CHECK02_ComposeV2_Fail(t *testing.T) {
	client := dialContainerA(t)
	runner := preflight.NewSSHRunner(client)
	cfg := config.Config{
		Host: config.Host{User: "testuser", Hostname: sshA.host, Port: sshA.port},
		Path: "/opt/testapp-preflight",
	}

	_, err := preflight.RunPreflightChecks(context.Background(), runner, cfg)
	if err == nil {
		t.Fatal("expected error when compose v2 is not available, got nil")
	}
	// The check chain stops at CHECK-01 or CHECK-02 — error must mention docker or compose.
	errLower := strings.ToLower(err.Error())
	if !strings.Contains(errLower, "docker") && !strings.Contains(errLower, "compose") {
		t.Errorf("error %q does not mention 'docker' or 'compose'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// CHECK-03: Docker daemon running (warning only)
// ---------------------------------------------------------------------------

// TestPreflight_CHECK03_DaemonRunning_Pass verifies that the daemon check succeeds
// (pass or warn — both are acceptable) when connecting to Container B with a running
// Docker daemon.
func TestPreflight_CHECK03_DaemonRunning_Pass(t *testing.T) {
	client := dialContainer(t, "sshuser")
	runner := preflight.NewSSHRunner(client)
	cfg := defaultCfg("sshuser")

	results, err := preflight.RunPreflightChecks(context.Background(), runner, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := findResult(results, "daemon-running")
	if r == nil {
		t.Fatal("daemon-running CheckResult not found")
	}
	// Both "pass" and "warn" are acceptable — daemon is running in DinD but the
	// socket path may differ from the standard location.
	if r.Status != "pass" && r.Status != "warn" {
		t.Errorf("expected Status='pass' or 'warn' for daemon-running, got %q", r.Status)
	}
}

// TestPreflight_CHECK03_DaemonRunning_Fail verifies that connecting to Container A
// (no Docker daemon) causes the check chain to terminate with an error.
//
// NOTE: This test uses CHECK-01 failure as a proxy for CHECK-03 failure. Container A
// has no Docker binary, so the check chain terminates at CHECK-01 before CHECK-03 is
// reached. The proxy is valid because a running Docker daemon cannot exist without
// the Docker binary being present. Isolated CHECK-03 fail scenarios (e.g. docker
// binary installed but daemon stopped) can be added in a future phase if needed.
func TestPreflight_CHECK03_DaemonRunning_Fail(t *testing.T) {
	client := dialContainerA(t)
	runner := preflight.NewSSHRunner(client)
	cfg := config.Config{
		Host: config.Host{User: "testuser", Hostname: sshA.host, Port: sshA.port},
		Path: "/opt/testapp-preflight",
	}

	_, err := preflight.RunPreflightChecks(context.Background(), runner, cfg)
	if err == nil {
		t.Fatal("expected error from Container A (no docker), got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "docker") {
		t.Errorf("error %q does not mention 'docker'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// CHECK-07: Root user warning (SC-3) — warning only, never blocks
// ---------------------------------------------------------------------------

// TestPreflight_CHECK07_RootWarning_DoesNotBlock (SC-3) verifies that connecting
// as root emits a warning to stderr but does NOT cause RunPreflightChecks to return
// an error. The root-user CheckResult must have Status == "warn".
func TestPreflight_CHECK07_RootWarning_DoesNotBlock(t *testing.T) {
	client := dialContainer(t, "root")
	runner := preflight.NewSSHRunner(client)
	cfg := defaultCfg("root")

	var results []preflight.CheckResult
	var err error
	output := captureStderr(t, func() {
		results, err = preflight.RunPreflightChecks(context.Background(), runner, cfg)
	})

	if err != nil {
		t.Fatalf("expected nil error for root user (warning only), got: %v", err)
	}
	if !strings.Contains(strings.ToLower(output), "root") {
		t.Errorf("expected root warning on stderr, got: %q", output)
	}

	r := findResult(results, "root-user")
	if r == nil {
		t.Fatal("root-user CheckResult not found")
	}
	if r.Status != "warn" {
		t.Errorf("expected Status='warn' for root-user, got %q", r.Status)
	}
}

// ---------------------------------------------------------------------------
// CHECK-04: Docker group membership (SC-4)
// ---------------------------------------------------------------------------

// TestPreflight_CHECK04_DockerGroup_Pass_sshuser (SC-4 happy path) verifies that
// docker group membership check passes for sshuser, who is in the docker group per
// the Dockerfile.sshd setup.
func TestPreflight_CHECK04_DockerGroup_Pass_sshuser(t *testing.T) {
	client := dialContainer(t, "sshuser")
	runner := preflight.NewSSHRunner(client)
	cfg := defaultCfg("sshuser")

	results, err := preflight.RunPreflightChecks(context.Background(), runner, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := findResult(results, "docker-group")
	if r == nil {
		t.Fatal("docker-group CheckResult not found")
	}
	if r.Status != "pass" {
		t.Errorf("expected Status='pass' for docker-group (sshuser is in docker group), got %q (message: %s)", r.Status, r.Message)
	}
}

// TestPreflight_CHECK04_DockerGroup_Fail_nosudouser (SC-4 fail path) verifies that
// the docker group check produces a non-passing outcome for nosudouser, who is not
// in the docker group and has no sudo access.
func TestPreflight_CHECK04_DockerGroup_Fail_nosudouser(t *testing.T) {
	client := dialContainer(t, "nosudouser")
	runner := preflight.NewSSHRunner(client)
	cfg := defaultCfg("nosudouser")

	results, err := preflight.RunPreflightChecks(context.Background(), runner, cfg)

	// Either a blocking error OR a non-passing result is acceptable — the important
	// thing is that nosudouser does NOT silently pass.
	if err != nil {
		// Hard fail: acceptable outcome, message must be non-empty
		if err.Error() == "" {
			t.Error("expected non-empty error message for nosudouser docker-group fail")
		}
		return
	}

	// No error: must have a warn or fail result for docker-group
	r := findResult(results, "docker-group")
	if r == nil {
		t.Fatal("docker-group CheckResult not found")
	}
	if r.Status == "pass" {
		t.Errorf("nosudouser should NOT pass docker-group check, got Status='pass' (message: %s)", r.Message)
	}
	if r.Message == "" {
		t.Error("expected non-empty Message for docker-group non-pass result")
	}
}

// ---------------------------------------------------------------------------
// CHECK-05: Passwordless sudo (SC-4 / SC-6)
// ---------------------------------------------------------------------------

// TestPreflight_CHECK05_PasswordlessSudo_Pass_sshuser (SC-4 happy path for sudo)
// verifies that sshuser (NOPASSWD: ALL) can create the target directory without
// errors and the target-dir result passes.
func TestPreflight_CHECK05_PasswordlessSudo_Pass_sshuser(t *testing.T) {
	client := dialContainer(t, "sshuser")
	runner := preflight.NewSSHRunner(client)
	cfg := config.Config{
		Host: config.Host{User: "sshuser", Hostname: sshB.host, Port: sshB.port},
		Path: "/opt/testapp-check05-sshuser",
	}

	results, err := preflight.RunPreflightChecks(context.Background(), runner, cfg)
	if err != nil {
		t.Fatalf("unexpected error for sshuser with passwordless sudo: %v", err)
	}

	// sshuser has passwordless sudo — target-dir creation must succeed
	r := findResult(results, "target-dir")
	if r == nil {
		t.Fatal("target-dir CheckResult not found")
	}
	if r.Status != "pass" {
		t.Errorf("expected Status='pass' for target-dir (sshuser has passwordless sudo), got %q (message: %s)", r.Status, r.Message)
	}
}

// TestPreflight_CHECK05_PasswordlessSudo_Fail_nosudouser (SC-6 fail scenario for
// CHECK-05) verifies that nosudouser, who has no sudo access at all, produces a
// non-passing outcome for the sudo-dependent checks.
func TestPreflight_CHECK05_PasswordlessSudo_Fail_nosudouser(t *testing.T) {
	client := dialContainer(t, "nosudouser")
	runner := preflight.NewSSHRunner(client)
	cfg := config.Config{
		Host: config.Host{User: "nosudouser", Hostname: sshB.host, Port: sshB.port},
		Path: "/opt/testapp-check05-nosudo",
	}

	results, err := preflight.RunPreflightChecks(context.Background(), runner, cfg)

	// The call must not succeed silently — nosudouser must produce a non-passing outcome
	// for sudo-dependent checks (target-dir or docker-group).
	if err != nil {
		// Hard fail: acceptable
		return
	}

	// No error: check that at least one of the sudo-dependent checks is non-passing.
	targetDir := findResult(results, "target-dir")
	dockerGroup := findResult(results, "docker-group")

	allPass := true
	if targetDir != nil && targetDir.Status != "pass" {
		allPass = false
	}
	if dockerGroup != nil && dockerGroup.Status != "pass" {
		allPass = false
	}

	if allPass {
		t.Error("nosudouser must produce at least one non-passing result for target-dir or docker-group")
	}
}

// ---------------------------------------------------------------------------
// CHECK-06: Target directory writable (SC-4 / SC-6)
// ---------------------------------------------------------------------------

// TestPreflight_CHECK06_TargetDir_Pass_sshuser (SC-4) verifies that sshuser can
// create and access the target directory.
func TestPreflight_CHECK06_TargetDir_Pass_sshuser(t *testing.T) {
	client := dialContainer(t, "sshuser")
	runner := preflight.NewSSHRunner(client)
	cfg := config.Config{
		Host: config.Host{User: "sshuser", Hostname: sshB.host, Port: sshB.port},
		Path: "/opt/testapp-check06",
	}

	results, err := preflight.RunPreflightChecks(context.Background(), runner, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := findResult(results, "target-dir")
	if r == nil {
		t.Fatal("target-dir CheckResult not found")
	}
	if r.Status != "pass" {
		t.Errorf("expected Status='pass' for target-dir, got %q (message: %s)", r.Status, r.Message)
	}
}

// TestPreflight_CHECK06_TargetDir_Fail_nosudouser (SC-6 fail scenario) verifies
// that nosudouser cannot write to a restricted target directory (chmod 000) and
// the check produces a non-passing outcome.
//
// The restricted directory is pre-created as root with chmod 000. nosudouser has no
// sudo access so cannot create or write to it. Permissions are restored via t.Cleanup.
func TestPreflight_CHECK06_TargetDir_Fail_nosudouser(t *testing.T) {
	// Pre-create the restricted directory as root.
	rootClient := dialContainer(t, "root")
	restrictedPath := "/opt/testapp-check06-nosudo"
	sshExecHelper(t, rootClient, fmt.Sprintf("mkdir -p %s && chmod 000 %s", restrictedPath, restrictedPath))

	// Restore permissions after the test completes so subsequent test runs start clean.
	t.Cleanup(func() {
		// Use a fresh root connection for cleanup — the original client may have been closed.
		cleanupClient := dialContainer(t, "root")
		session, err := cleanupClient.NewSession()
		if err != nil {
			return
		}
		defer session.Close()
		session.Run(fmt.Sprintf("chmod 755 %s", restrictedPath)) //nolint:errcheck
	})

	// Dial as nosudouser and run preflight checks.
	nosudoClient := dialContainer(t, "nosudouser")
	runner := preflight.NewSSHRunner(nosudoClient)
	cfg := config.Config{
		Host: config.Host{User: "nosudouser", Hostname: sshB.host, Port: sshB.port},
		Path: restrictedPath,
	}

	results, err := preflight.RunPreflightChecks(context.Background(), runner, cfg)

	// nosudouser cannot write to a chmod 000 directory and has no sudo.
	// Either an error or a non-pass result is expected.
	if err != nil {
		// Hard fail: acceptable — no further assertions needed.
		return
	}

	// No error: must have a warn or fail result for target-dir.
	r := findResult(results, "target-dir")
	if r == nil {
		t.Fatal("target-dir CheckResult not found")
	}
	if r.Status == "pass" {
		t.Errorf("nosudouser should NOT pass target-dir check on chmod 000 dir, got Status='pass'")
	}
}
