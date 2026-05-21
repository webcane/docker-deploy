package main

import (
	"strings"
	"testing"
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
