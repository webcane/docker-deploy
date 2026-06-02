package ssh

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// generateTestKey creates a fresh RSA public/private key pair for use in tests.
// Returns the gossh.PublicKey for callback invocations and the gossh.Signer
// (which wraps the private key) so callers can derive the known_hosts line.
func generateTestKey(t *testing.T) (gossh.PublicKey, gossh.Signer) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("gossh.NewSignerFromKey: %v", err)
	}
	return signer.PublicKey(), signer
}

// writeKnownHostsLine writes a single known_hosts line for hostname+key into path.
func writeKnownHostsLine(t *testing.T, path, hostname string, key gossh.PublicKey) {
	t.Helper()
	line := knownhosts.Line([]string{hostname}, key) + "\n"
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatalf("writeKnownHostsLine: %v", err)
	}
}

// TestBuildHostKeyCallback_KnownHostAccepted verifies that buildHostKeyCallback
// returns nil when the callback is invoked with the exact key stored in known_hosts
// (CFG-01: known-hosts verification accepts matching key).
func TestBuildHostKeyCallback_KnownHostAccepted(t *testing.T) {
	dir := t.TempDir()
	knownHostsPath := filepath.Join(dir, "known_hosts")

	const hostname = "example.com:22"
	pubKey, _ := generateTestKey(t)
	writeKnownHostsLine(t, knownHostsPath, hostname, pubKey)

	cb, err := buildHostKeyCallback(knownHostsPath)
	if err != nil {
		t.Fatalf("buildHostKeyCallback returned error: %v", err)
	}

	addr := &net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 22}
	if err := cb(hostname, addr, pubKey); err != nil {
		t.Errorf("callback returned non-nil error for known matching key: %v", err)
	}
}

// TestBuildHostKeyCallback_UnknownHost verifies that buildHostKeyCallback returns
// *UnknownHostError when the host is not present in known_hosts
// (CFG-01: TOFU detection — unknown host must produce *UnknownHostError).
func TestBuildHostKeyCallback_UnknownHost(t *testing.T) {
	dir := t.TempDir()
	// Create an empty known_hosts file — no entries.
	knownHostsPath := filepath.Join(dir, "known_hosts")
	if err := os.WriteFile(knownHostsPath, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	pubKey, _ := generateTestKey(t)

	cb, err := buildHostKeyCallback(knownHostsPath)
	if err != nil {
		t.Fatalf("buildHostKeyCallback returned error: %v", err)
	}

	addr := &net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 22}
	cbErr := cb("example.com:22", addr, pubKey)

	if cbErr == nil {
		t.Fatal("callback returned nil for unknown host; want *UnknownHostError")
	}
	var uhe *UnknownHostError
	if !asUnknownHostError(cbErr, &uhe) {
		t.Errorf("callback returned %T (%v); want *UnknownHostError", cbErr, cbErr)
	}
}

// TestBuildHostKeyCallback_KeyMismatch verifies that buildHostKeyCallback returns
// *KeyMismatchError when the host is in known_hosts but presents a different key
// (CFG-01: changed-fingerprint must be a hard fail — *KeyMismatchError).
func TestBuildHostKeyCallback_KeyMismatch(t *testing.T) {
	dir := t.TempDir()
	knownHostsPath := filepath.Join(dir, "known_hosts")

	const hostname = "example.com:22"

	// keyA is stored in known_hosts; keyB is what the "server" presents.
	keyA, _ := generateTestKey(t)
	keyB, _ := generateTestKey(t)

	writeKnownHostsLine(t, knownHostsPath, hostname, keyA)

	cb, err := buildHostKeyCallback(knownHostsPath)
	if err != nil {
		t.Fatalf("buildHostKeyCallback returned error: %v", err)
	}

	addr := &net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 22}
	cbErr := cb(hostname, addr, keyB)

	if cbErr == nil {
		t.Fatal("callback returned nil for key mismatch; want *KeyMismatchError")
	}
	var kme *KeyMismatchError
	if !asKeyMismatchError(cbErr, &kme) {
		t.Errorf("callback returned %T (%v); want *KeyMismatchError", cbErr, cbErr)
	}
}

// asUnknownHostError attempts a type assertion on err to *UnknownHostError.
// Using errors.As is not necessary here because UnknownHostError is returned
// directly (not wrapped), but we use the same pattern as asKeyError for consistency.
func asUnknownHostError(err error, target **UnknownHostError) bool {
	if err == nil {
		return false
	}
	v := &UnknownHostError{}
	ok := errors.As(err, &v)
	if ok {
		*target = v
	}
	return ok
}

// asKeyMismatchError attempts a type assertion on err to *KeyMismatchError.
func asKeyMismatchError(err error, target **KeyMismatchError) bool {
	if err == nil {
		return false
	}
	v := &KeyMismatchError{}
	ok := errors.As(err, &v)
	if ok {
		*target = v
	}
	return ok
}

// TestBuildAuthMethods_NoPasswordOrKeyboardInteractive verifies that buildAuthMethods
// never adds password or keyboard-interactive auth methods (CFG-01 / CLAUDE.md Rule 1).
//
// The gossh.AuthMethod interface does not expose its inner type, so we cannot
// enumerate method names at runtime. Instead this test:
//  1. Calls buildAuthMethods and asserts it returns without error.
//  2. Asserts the returned slice has at most 2 entries (agent + key files).
//  3. Documents that the structural guarantee — no gossh.Password / gossh.KeyboardInteractive
//     call sites — is enforced by the source code of buildAuthMethods in client.go.
//
// An SSH server that only accepts password auth would reject all returned methods,
// confirming no password fallback exists.
func TestBuildAuthMethods_NoPasswordOrKeyboardInteractive(t *testing.T) {
	// Unset SSH_AUTH_SOCK so agent is skipped deterministically.
	origSock := os.Getenv("SSH_AUTH_SOCK")
	if err := os.Unsetenv("SSH_AUTH_SOCK"); err != nil {
		t.Fatalf("os.Unsetenv: %v", err)
	}
	defer func() {
		if origSock != "" {
			_ = os.Setenv("SSH_AUTH_SOCK", origSock)
		}
	}()

	methods, err := buildAuthMethods("example.com", "testuser")
	if err != nil {
		t.Fatalf("buildAuthMethods returned error: %v", err)
	}

	// With no SSH_AUTH_SOCK and (likely) no identity files matching "example.com"
	// in the test environment's ~/.ssh/config, result is at most 1 entry.
	// We assert an upper bound of 2 (agent slot + key-file slot) to catch any
	// future addition of password/keyboard-interactive methods.
	const maxExpected = 2
	if len(methods) > maxExpected {
		t.Errorf("buildAuthMethods returned %d methods; want at most %d — "+
			"extra methods may include forbidden password or keyboard-interactive auth",
			len(methods), maxExpected)
	}

	// Structural assertion: the source of buildAuthMethods in client.go must not
	// contain calls to gossh.Password or gossh.KeyboardInteractive.
	// We verify this by reading the source file and failing if the banned symbols appear.
	src, err := os.ReadFile("client.go")
	if err != nil {
		t.Fatalf("reading client.go for structural check: %v", err)
	}
	srcStr := string(src)

	if contains(srcStr, "gossh.Password(") {
		t.Error("client.go contains gossh.Password() — password auth is forbidden (CLAUDE.md Rule 1)")
	}
	if contains(srcStr, "gossh.KeyboardInteractive(") {
		t.Error("client.go contains gossh.KeyboardInteractive() — keyboard-interactive auth is forbidden")
	}
}

// contains is a simple substring check to keep the test free of extra imports.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexString(s, substr) >= 0)
}

func indexString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
