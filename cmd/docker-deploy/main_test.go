package main

import (
	"strings"
	"testing"

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
