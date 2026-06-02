package keychain

import (
	"fmt"
	"os/exec"
	"testing"
)

// stubSecurity replaces execSecurityFunc with a stub that records args and
// returns the configured output/error.
func stubSecurity(t *testing.T, out string, err error) (restore func()) {
	t.Helper()
	orig := execSecurityFunc
	execSecurityFunc = func(args ...string) (string, error) { return out, err }
	return func() { execSecurityFunc = orig }
}

// stubSecurityCapture replaces execSecurityFunc and captures the args it was
// called with.
func stubSecurityCapture(t *testing.T, out string, err error) (captured *[]string, restore func()) {
	t.Helper()
	orig := execSecurityFunc
	var got []string
	execSecurityFunc = func(args ...string) (string, error) {
		got = append([]string{}, args...)
		return out, err
	}
	return &got, func() { execSecurityFunc = orig }
}

// TestLookup_Found verifies that Lookup returns the password when security
// prints it to stdout.
func TestLookup_Found(t *testing.T) {
	restore := stubSecurity(t, "s3cr3t", nil)
	defer restore()

	pw, err := Lookup("example.com", "deploy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pw != "s3cr3t" {
		t.Errorf("got %q, want %q", pw, "s3cr3t")
	}
}

// TestLookup_NotFound verifies that Lookup returns ("", nil) when security
// exits with a non-zero status (e.g. exit 44 = item not found).
func TestLookup_NotFound(t *testing.T) {
	restore := stubSecurity(t, "", fmt.Errorf("exit status 44"))
	defer restore()

	pw, err := Lookup("example.com", "deploy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pw != "" {
		t.Errorf("expected empty password, got %q", pw)
	}
}

// TestLookup_BinaryMissing verifies graceful fallback when /usr/bin/security
// is not available (returns ("", nil) so callers fall back to prompting).
func TestLookup_BinaryMissing(t *testing.T) {
	restore := stubSecurity(t, "", &exec.Error{Name: "/usr/bin/security", Err: exec.ErrNotFound})
	defer restore()

	pw, err := Lookup("example.com", "deploy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pw != "" {
		t.Errorf("expected empty password, got %q", pw)
	}
}

// TestStore_CallsSecurityWithCorrectArgs verifies that Store issues the right
// add-generic-password invocation.
func TestStore_CallsSecurityWithCorrectArgs(t *testing.T) {
	got, restore := stubSecurityCapture(t, "", nil)
	defer restore()

	if err := Store("example.com", "deploy", "s3cr3t"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{
		"add-generic-password", "-U",
		"-s", "docker-deploy",
		"-a", "deploy@example.com",
		"-w", "s3cr3t",
	}
	if len(*got) != len(want) {
		t.Fatalf("arg count: got %d, want %d: %v", len(*got), len(want), *got)
	}
	for i, w := range want {
		if (*got)[i] != w {
			t.Errorf("arg[%d]: got %q, want %q", i, (*got)[i], w)
		}
	}
}

// TestStore_ReturnsErrorOnFailure verifies that Store propagates security errors.
func TestStore_ReturnsErrorOnFailure(t *testing.T) {
	restore := stubSecurity(t, "", fmt.Errorf("exit status 1"))
	defer restore()

	if err := Store("example.com", "deploy", "pw"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestDelete_CallsSecurityWithCorrectArgs verifies the delete-generic-password
// invocation.
func TestDelete_CallsSecurityWithCorrectArgs(t *testing.T) {
	got, restore := stubSecurityCapture(t, "", nil)
	defer restore()

	if err := Delete("example.com", "deploy"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{
		"delete-generic-password",
		"-s", "docker-deploy",
		"-a", "deploy@example.com",
	}
	if len(*got) != len(want) {
		t.Fatalf("arg count: got %d, want %d: %v", len(*got), len(want), *got)
	}
	for i, w := range want {
		if (*got)[i] != w {
			t.Errorf("arg[%d]: got %q, want %q", i, (*got)[i], w)
		}
	}
}

// TestAccount verifies the account identifier format.
func TestAccount(t *testing.T) {
	if got, want := account("deploy", "example.com"), "deploy@example.com"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
