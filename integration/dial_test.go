//go:build integration

package integration_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	internalssh "github.com/webcane/docker-deploy/internal/ssh"
)

// TestDial_Timeout verifies that Dial returns a "timed out" error when the target
// host (192.0.2.1 — TEST-NET, guaranteed non-routable per RFC 5737) does not respond.
func TestDial_Timeout(t *testing.T) {
	cfg := internalssh.DialConfig{
		User:           "nobody",
		Hostname:       "192.0.2.1", // TEST-NET, guaranteed non-routable
		Port:           22,
		Timeout:        500 * time.Millisecond,
		KnownHostsPath: emptyKnownHosts(t),
		Stdin:          strings.NewReader(""),
		Stdout:         os.Stderr,
	}
	_, err := internalssh.Dial(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected error containing 'timed out', got: %v", err)
	}
}

// TestDial_UnknownHost_TOFU verifies that connecting to Container A (sshA) with an
// empty known_hosts and the user responding "no" returns a non-nil error.
func TestDial_UnknownHost_TOFU(t *testing.T) {
	cfg := internalssh.DialConfig{
		User:           "testuser",
		Hostname:       sshA.host,
		Port:           sshA.port,
		KnownHostsPath: emptyKnownHosts(t),
		Timeout:        15 * time.Second,
		Stdin:          strings.NewReader("no\n"),
		Stdout:         os.Stderr,
	}
	_, err := internalssh.Dial(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error when user rejects unknown host, got nil")
	}
}

// TestDial_UnknownHost_TOFU_Accepted verifies that responding "yes" to the TOFU prompt
// causes the host key to be appended to the known_hosts file.
func TestDial_UnknownHost_TOFU_Accepted(t *testing.T) {
	khFile := emptyKnownHosts(t)

	cfg := internalssh.DialConfig{
		User:           "testuser",
		Hostname:       sshA.host,
		Port:           sshA.port,
		KnownHostsPath: khFile,
		Timeout:        15 * time.Second,
		Stdin:          strings.NewReader("yes\n"),
		Stdout:         os.Stderr,
	}
	// Ignore dial error — auth will fail since no key is configured. What matters
	// is that the TOFU path fires and writes to known_hosts.
	internalssh.Dial(context.Background(), cfg) //nolint:errcheck

	data, err := os.ReadFile(khFile)
	if err != nil {
		t.Fatalf("read known_hosts: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected known_hosts to contain host entry after TOFU acceptance, but file is empty")
	}
}

// TestDial_Success verifies that Dial progresses past host-key verification when a
// valid known_hosts entry exists for Container A (sshA). Auth failure is acceptable
// (no key injected into the container); what must NOT happen is a timeout or TOFU error.
func TestDial_Success(t *testing.T) {
	khFile := seedKnownHosts(t, sshA.host, sshA.port, sshA.hostKey)
	_, keyFile := generateTestKeyFile(t)
	_ = keyFile // key is not injected into container; auth will fail

	cfg := internalssh.DialConfig{
		User:           "testuser",
		Hostname:       sshA.host,
		Port:           sshA.port,
		KnownHostsPath: khFile,
		Timeout:        15 * time.Second,
		Stdin:          strings.NewReader(""),
		Stdout:         os.Stderr,
	}

	client, err := internalssh.Dial(context.Background(), cfg)
	if err == nil {
		// Actually succeeded (e.g., container accepted some default auth). Clean up.
		client.Close()
		return
	}

	// Auth errors are acceptable; timeout or TOFU errors are not.
	if strings.Contains(err.Error(), "timed out") {
		t.Fatalf("unexpected timeout (host key seeded correctly, network OK): %v", err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "unknown host") {
		t.Fatalf("unexpected TOFU error (host key was seeded in known_hosts): %v", err)
	}
	// Auth failure is the expected path when no key is injected.
	t.Logf("TestDial_Success: host-key verification passed; auth failed as expected (no key injected): %v", err)
}
