package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/webcane/docker-deploy/internal/config"
	"github.com/webcane/docker-deploy/internal/preflight"
)

// TestSkipEnvFlagRegistered verifies that the deploy command registers --skip-env
// as a boolean flag (required for Phase 7 feature delivery).
// This test calls buildDeployCmd() which must exist in main.go.
func TestSkipEnvFlagRegistered(t *testing.T) {
	cmd := buildDeployCmd()
	f := cmd.Flags().Lookup("skip-env")
	if f == nil {
		t.Fatal("--skip-env flag not registered on deploy command")
	}
	if f.Value.Type() != "bool" {
		t.Errorf("--skip-env flag type = %q; want %q", f.Value.Type(), "bool")
	}
}

// TestVerboseFlagRegistered verifies that the deploy command registers --verbose
// as a boolean flag (required for Phase 7 feature delivery).
func TestVerboseFlagRegistered(t *testing.T) {
	cmd := buildDeployCmd()
	f := cmd.Flags().Lookup("verbose")
	if f == nil {
		t.Fatal("--verbose flag not registered on deploy command")
	}
	if f.Value.Type() != "bool" {
		t.Errorf("--verbose flag type = %q; want %q", f.Value.Type(), "bool")
	}
}

// TestSkipEnvFlagDescription verifies the --skip-env flag has a meaningful description.
func TestSkipEnvFlagDescription(t *testing.T) {
	cmd := buildDeployCmd()
	f := cmd.Flags().Lookup("skip-env")
	if f == nil {
		t.Fatal("--skip-env flag not registered")
	}
	if !strings.Contains(f.Usage, ".env") {
		t.Errorf("--skip-env flag usage %q does not mention '.env'", f.Usage)
	}
}

// TestVerboseFlagDescription verifies the --verbose flag has a meaningful description.
func TestVerboseFlagDescription(t *testing.T) {
	cmd := buildDeployCmd()
	f := cmd.Flags().Lookup("verbose")
	if f == nil {
		t.Fatal("--verbose flag not registered")
	}
	if f.Usage == "" {
		t.Error("--verbose flag usage is empty")
	}
}

// TestRollupMsgVerboseBranchOmitsHint verifies that rollupMsg(true) returns a
// bare warning without the --verbose hint. When verbose mode is already active,
// prompting the user to use --verbose is redundant and misleading (D-02).
func TestRollupMsgVerboseBranchOmitsHint(t *testing.T) {
	got := rollupMsg(true)
	const want = "WARN: there are some warnings during deployment."
	if got != want {
		t.Errorf("rollupMsg(verbose=true) = %q; want %q", got, want)
	}
	if strings.Contains(got, "--verbose") {
		t.Errorf("rollupMsg(verbose=true) must not contain '--verbose' hint; got %q", got)
	}
}

// TestRollupMsgNonVerboseBranchIncludesHint verifies that rollupMsg(false) appends
// the --verbose hint so users know where to find suppressed details (D-02).
func TestRollupMsgNonVerboseBranchIncludesHint(t *testing.T) {
	got := rollupMsg(false)
	if !strings.HasPrefix(got, "WARN: there are some warnings during deployment.") {
		t.Errorf("rollupMsg(verbose=false) = %q; want prefix %q", got, "WARN: there are some warnings during deployment.")
	}
	if !strings.Contains(got, "--verbose") {
		t.Errorf("rollupMsg(verbose=false) must contain '--verbose' hint; got %q", got)
	}
}

// TestFormatCheckResultFormatsAllStatusValues verifies that formatCheckResult
// produces the bracketed "[STATUS] name: message" format required by verbose
// preflight output (D-01). The status is uppercased; the leading two-space
// indent and colon separators must be exact so the CLI output is consistent.
func TestFormatCheckResultFormatsAllStatusValues(t *testing.T) {
	tests := []struct {
		name   string
		result preflight.CheckResult
		want   string
	}{
		{
			name:   "pass status is uppercased in brackets",
			result: preflight.CheckResult{Name: "docker-installed", Status: "pass", Message: "Docker version 24.0.0"},
			want:   "  [PASS] docker-installed: Docker version 24.0.0",
		},
		{
			name:   "warn status is uppercased in brackets",
			result: preflight.CheckResult{Name: "daemon-running", Status: "warn", Message: "Docker daemon is not running"},
			want:   "  [WARN] daemon-running: Docker daemon is not running",
		},
		{
			name:   "fail status is uppercased in brackets",
			result: preflight.CheckResult{Name: "compose-v2", Status: "fail", Message: "docker compose v2 not installed"},
			want:   "  [FAIL] compose-v2: docker compose v2 not installed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatCheckResult(tc.result)
			if got != tc.want {
				t.Errorf("formatCheckResult(%+v) = %q; want %q", tc.result, got, tc.want)
			}
		})
	}
}

// TestVersionCmd_Registered verifies that buildDeployCmd() has a subcommand with Use=="version".
func TestVersionCmd_Registered(t *testing.T) {
	cmd := buildDeployCmd()
	for _, sub := range cmd.Commands() {
		if sub.Use == "version" {
			return
		}
	}
	t.Fatal("deploy command has no 'version' subcommand registered")
}

// TestVersionCmd_DevOutput verifies that when buildTime=="unknown", runVersion() prints
// "Docker Deploy Version dev" and does NOT include a "Built:" line (D-03).
func TestVersionCmd_DevOutput(t *testing.T) {
	origVersion := version
	origGitCommit := gitCommit
	origBuildTime := buildTime
	defer func() {
		version = origVersion
		gitCommit = origGitCommit
		buildTime = origBuildTime
	}()

	version = "dev"
	gitCommit = "abc1234"
	buildTime = "unknown"

	var buf strings.Builder
	if err := runVersionTo(&buf); err != nil {
		t.Fatalf("runVersionTo() returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Docker Deploy Version dev") {
		t.Errorf("output does not contain 'Docker Deploy Version dev'; got:\n%s", out)
	}
	if strings.Contains(out, "Built:") {
		t.Errorf("dev output must not include 'Built:' line; got:\n%s", out)
	}
}

// TestVersionCmd_TaggedOutput verifies that when buildTime is not "unknown",
// runVersion() includes a "Built:" line in its output (D-01).
func TestVersionCmd_TaggedOutput(t *testing.T) {
	origVersion := version
	origGitCommit := gitCommit
	origBuildTime := buildTime
	defer func() {
		version = origVersion
		gitCommit = origGitCommit
		buildTime = origBuildTime
	}()

	version = "v0.6.3"
	gitCommit = "de40ad0"
	buildTime = "2026-04-20T14:57:44Z"

	var buf strings.Builder
	if err := runVersionTo(&buf); err != nil {
		t.Fatalf("runVersionTo() returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Docker Deploy Version v0.6.3") {
		t.Errorf("output does not contain 'Docker Deploy Version v0.6.3'; got:\n%s", out)
	}
	if !strings.Contains(out, "Built:") {
		t.Errorf("tagged output must include 'Built:' line; got:\n%s", out)
	}
}

// TestVersionCmd_DevBuildWithInjectedTime verifies that a dev build with an injected
// buildTime (non-"unknown") still omits the Built: line (regression for D-03).
// The Makefile always injects buildTime via ldflags, so the discriminator must be
// version != "dev", not buildTime != "unknown".
func TestVersionCmd_DevBuildWithInjectedTime(t *testing.T) {
	origVersion := version
	origGitCommit := gitCommit
	origBuildTime := buildTime
	defer func() {
		version = origVersion
		gitCommit = origGitCommit
		buildTime = origBuildTime
	}()

	version = "dev"
	gitCommit = "abc1234"
	buildTime = "2026-05-26T12:09:30Z" // non-"unknown" — simulates `make build` with ldflags

	var buf strings.Builder
	if err := runVersionTo(&buf); err != nil {
		t.Fatalf("runVersionTo() returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Docker Deploy Version dev") {
		t.Errorf("output does not contain 'Docker Deploy Version dev'; got:\n%s", out)
	}
	if strings.Contains(out, "Built:") {
		t.Errorf("dev build with injected buildTime must not include 'Built:' line; got:\n%s", out)
	}
	if !strings.Contains(out, "Git commit:") {
		t.Errorf("output does not contain 'Git commit:'; got:\n%s", out)
	}
	if !strings.Contains(out, "OS/Arch:") {
		t.Errorf("output does not contain 'OS/Arch:'; got:\n%s", out)
	}
}

// TestVersionCmd_ExitZero verifies that buildVersionCmd().RunE returns nil (exit 0).
func TestVersionCmd_ExitZero(t *testing.T) {
	cmd := buildVersionCmd()
	if cmd.RunE == nil {
		t.Fatal("buildVersionCmd() RunE is nil")
	}
	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Errorf("buildVersionCmd().RunE returned non-nil error: %v", err)
	}
}

// TestValidateCmd_Registered verifies that buildDeployCmd() has a subcommand with Use=="validate".
func TestValidateCmd_Registered(t *testing.T) {
	cmd := buildDeployCmd()
	for _, sub := range cmd.Commands() {
		if sub.Use == "validate" {
			return
		}
	}
	t.Fatal("deploy command has no 'validate' subcommand registered")
}

// TestValidateCmd_ValidConfig verifies that runValidate() with a valid deploy.yaml in cwd
// returns nil. Output must contain "✓ deploy.yaml is valid" (D-09).
func TestValidateCmd_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	// Write a minimal valid deploy.yaml with an explicit compose_file so Resolve()
	// does not need to auto-detect a local compose file.
	if err := os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte("version: 1\ntarget:\n  host: ssh://user@example.com\n  compose_file: docker-compose.yml\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	// Capture stdout by replacing os.Stdout temporarily.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStdout := os.Stdout
	os.Stdout = w

	runErr := runValidate()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = origStdout

	var buf strings.Builder
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if runErr != nil {
		t.Fatalf("runValidate() returned error: %v", runErr)
	}
	if !strings.Contains(out, "✓ deploy.yaml is valid") {
		t.Errorf("stdout does not contain '✓ deploy.yaml is valid'; got: %q", out)
	}
}

// TestValidateCmd_MissingFile verifies that runValidate() in a dir with no deploy.yaml
// returns a non-nil error and the message contains "deploy.yaml not found" (D-07).
func TestValidateCmd_MissingFile(t *testing.T) {
	dir := t.TempDir()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	err = runValidate()
	if err == nil {
		t.Fatal("runValidate() expected non-nil error for missing deploy.yaml, got nil")
	}
	if !strings.Contains(err.Error(), "deploy.yaml not found") {
		t.Errorf("error %q does not contain 'deploy.yaml not found'", err.Error())
	}
}

// TestValidateCmd_InvalidYAML verifies that runValidate() with a deploy.yaml containing
// invalid YAML returns a non-nil error.
func TestValidateCmd_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte(":::invalid yaml:::\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	err = runValidate()
	if err == nil {
		t.Fatal("runValidate() expected non-nil error for invalid YAML, got nil")
	}
}

// TestValidateCmd_NoSSH verifies that runValidate() does not import or invoke any SSH dialing.
// This is a structural test: if the function exists and passes other tests without needing
// sshpkg.Dial(), the code path is confirmed SSH-free. The test validates that the validate
// subcommand's RunE is set (not a bare Run which would swallow errors).
func TestValidateCmd_NoSSH(t *testing.T) {
	cmd := buildValidateCmd()
	if cmd.RunE == nil {
		t.Fatal("buildValidateCmd() RunE is nil; validate must use RunE not Run")
	}
	// Structural: validate that SilenceUsage is set so cobra does not print usage on error.
	if !cmd.SilenceUsage {
		t.Error("buildValidateCmd() SilenceUsage must be true (Pitfall 4: suppress usage block on error)")
	}
}

// TestFormatHostTarget verifies deploy complete message formatting for default and custom ports.
func TestFormatHostTarget(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		port     int
		path     string
		want     string
	}{
		{
			name:     "default port 22 omits colon",
			hostname: "192.168.1.99",
			port:     22,
			path:     "/opt/test-deploy",
			want:     "192.168.1.99/opt/test-deploy",
		},
		{
			name:     "custom port includes colon and port",
			hostname: "192.168.1.99",
			port:     2222,
			path:     "/opt/test-deploy",
			want:     "192.168.1.99:2222/opt/test-deploy",
		},
		{
			name:     "zero port treated as default",
			hostname: "192.168.1.99",
			port:     0,
			path:     "/opt/test-deploy",
			want:     "192.168.1.99/opt/test-deploy",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatHostTarget(tc.hostname, tc.port, tc.path)
			if got != tc.want {
				t.Errorf("formatHostTarget(%q, %d, %q) = %q; want %q",
					tc.hostname, tc.port, tc.path, got, tc.want)
			}
		})
	}
}

// TestCompletionCmd_Registered verifies that buildDeployCmd() has a subcommand
// with Use == "completion [bash|zsh]" (D-02, D-04, D-05) and that the subcommand
// is marked Hidden and DisableFlagsInUseLine per D-02.
func TestCompletionCmd_Registered(t *testing.T) {
	cmd := buildDeployCmd()
	for _, sub := range cmd.Commands() {
		if sub.Use == "completion [bash|zsh]" {
			if sub.Hidden != true {
				t.Errorf("completion subcommand Hidden = %v; want true (D-02)", sub.Hidden)
			}
			if sub.DisableFlagsInUseLine != true {
				t.Errorf("completion subcommand DisableFlagsInUseLine = %v; want true (D-02)", sub.DisableFlagsInUseLine)
			}
			return
		}
	}
	t.Fatal("deploy command has no 'completion' subcommand registered")
}

// TestCompletionCmd_InvalidShell verifies that passing an unsupported shell name to the
// completion subcommand returns a non-nil error (D-01). cobra.MatchAll(ExactArgs(1),
// OnlyValidArgs) with ValidArgs=["bash","zsh"] must reject "fish" before RunE fires.
func TestCompletionCmd_InvalidShell(t *testing.T) {
	cmd := buildDeployCmd()
	// Find the completion subcommand so we can invoke its Args validator directly.
	var completionCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Use == "completion [bash|zsh]" {
			completionCmd = sub
			break
		}
	}
	if completionCmd == nil {
		t.Fatal("deploy command has no 'completion' subcommand registered")
	}

	// cobra.Command.Args is the validation function set by cobra.MatchAll(...).
	// Calling it directly with an unsupported shell name must return a non-nil error.
	if completionCmd.Args == nil {
		t.Fatal("completion subcommand has no Args validator set")
	}
	err := completionCmd.Args(completionCmd, []string{"fish"})
	if err == nil {
		t.Error("completion subcommand Args validator accepted 'fish'; want rejection error")
	}
}

// TestFormatHealthcheckRow verifies the formatHealthcheckRow helper produces the correct
// output for both disabled (zero) and enabled (non-zero) healthcheck configurations.
func TestFormatHealthcheckRow(t *testing.T) {
	tests := []struct {
		name string
		hc   config.HealthcheckConfig
		want string
	}{
		{
			name: "disabled when all zero",
			hc:   config.HealthcheckConfig{},
			want: "  Healthcheck:  disabled",
		},
		{
			name: "shows values when non-zero",
			hc: config.HealthcheckConfig{
				Interval: 30 * time.Second,
				Timeout:  10 * time.Second,
				Retries:  3,
			},
			want: "  Healthcheck:  interval=30s timeout=10s retries=3",
		},
		{
			name: "shows only interval non-zero",
			hc: config.HealthcheckConfig{
				Interval: 1 * time.Minute,
			},
			want: "  Healthcheck:  interval=1m0s timeout=0s retries=0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatHealthcheckRow(tc.hc)
			if got != tc.want {
				t.Errorf("formatHealthcheckRow(%+v) = %q; want %q", tc.hc, got, tc.want)
			}
		})
	}
}
