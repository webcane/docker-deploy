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
