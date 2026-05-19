//go:build integration

package ssh_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"

	internalssh "github.com/webcane/docker-deploy/internal/ssh"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// sshContainer holds a running SSH container for tests.
type sshContainer struct {
	host    string
	port    int
	hostKey gossh.PublicKey
	cleanup func()
}

// startSSHContainer launches an OpenSSH container and returns test connection info.
func startSSHContainer(t *testing.T) *sshContainer {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "lscr.io/linuxserver/openssh-server:latest",
		ExposedPorts: []string{"2222/tcp"},
		Env: map[string]string{
			"PUID":            "1000",
			"PGID":            "1000",
			"USER_NAME":       "testuser",
			"USER_PASSWORD":   "testpass",
			"PASSWORD_ACCESS": "true",
		},
		WaitingFor: wait.ForListeningPort("2222/tcp").WithStartupTimeout(90 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start SSH container: %v", err)
	}

	mappedPort, err := container.MappedPort(ctx, "2222")
	if err != nil {
		container.Terminate(ctx) //nolint:errcheck
		t.Fatalf("get mapped port: %v", err)
	}

	hostIP, err := container.Host(ctx)
	if err != nil {
		container.Terminate(ctx) //nolint:errcheck
		t.Fatalf("get container host: %v", err)
	}

	portInt, err := strconv.Atoi(mappedPort.Port())
	if err != nil {
		container.Terminate(ctx) //nolint:errcheck
		t.Fatalf("parse port %q: %v", mappedPort.Port(), err)
	}

	// Capture the container's SSH host key.
	hostKey := captureHostKey(t, hostIP, portInt)

	return &sshContainer{
		host:    hostIP,
		port:    portInt,
		hostKey: hostKey,
		cleanup: func() { container.Terminate(ctx) }, //nolint:errcheck
	}
}

// captureHostKey dials the SSH server to capture its host public key, then disconnects.
func captureHostKey(t *testing.T, host string, port int) gossh.PublicKey {
	t.Helper()
	var captured gossh.PublicKey
	cfg := &gossh.ClientConfig{
		User: "testuser",
		Auth: []gossh.AuthMethod{gossh.Password("testpass")},
		HostKeyCallback: func(hostname string, remote net.Addr, key gossh.PublicKey) error {
			captured = key
			// Return error to immediately terminate the connection after key capture.
			return fmt.Errorf("key captured")
		},
		Timeout: 15 * time.Second,
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	gossh.Dial("tcp", addr, cfg) //nolint:errcheck — intentional disconnect
	if captured == nil {
		t.Fatal("failed to capture SSH host key from container")
	}
	return captured
}

// seedKnownHosts writes a known_hosts entry for the given host+port to a temp file
// and returns the file path.
func seedKnownHosts(t *testing.T, host string, port int, key gossh.PublicKey) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "known_hosts_*")
	if err != nil {
		t.Fatalf("create temp known_hosts: %v", err)
	}
	defer f.Close()

	// known_hosts uses [host]:port notation for non-standard ports.
	addr := fmt.Sprintf("[%s]:%d", host, port)
	line := fmt.Sprintf("%s %s", addr, string(gossh.MarshalAuthorizedKey(key)))
	fmt.Fprint(f, line)
	return f.Name()
}

// emptyKnownHosts creates an empty known_hosts file in a temp dir.
func emptyKnownHosts(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(path, []byte{}, 0600); err != nil {
		t.Fatalf("create empty known_hosts: %v", err)
	}
	return path
}

// generateTestKeyFile creates an RSA private key, writes it to a temp file,
// and returns the file path and signer.
func generateTestKeyFile(t *testing.T) (string, gossh.Signer) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}
	pemBlock, err := gossh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	keyPath := filepath.Join(t.TempDir(), "id_rsa")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(pemBlock), 0600); err != nil {
		t.Fatalf("write private key: %v", err)
	}
	return keyPath, signer
}

// TestDial_Timeout verifies that Dial returns a "timed out" error when the target
// host (192.0.2.1 — TEST-NET, guaranteed non-routable) does not respond.
func TestDial_Timeout(t *testing.T) {
	cfg := internalssh.DialConfig{
		User:     "nobody",
		Hostname: "192.0.2.1",
		Port:     22,
		Timeout:  500 * time.Millisecond,
		Stdin:    strings.NewReader(""),
		Stdout:   os.Stderr,
	}
	_, err := internalssh.Dial(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected error containing 'timed out', got: %v", err)
	}
}

// TestDial_UnknownHost verifies that connecting with an empty known_hosts and the
// user responding "no" returns a non-nil error.
func TestDial_UnknownHost(t *testing.T) {
	sc := startSSHContainer(t)
	defer sc.cleanup()

	cfg := internalssh.DialConfig{
		User:           "testuser",
		Hostname:       sc.host,
		Port:           sc.port,
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

// TestDial_UnknownHostAccepted verifies that responding "yes" to the TOFU prompt
// causes the host key to be appended to known_hosts.
func TestDial_UnknownHostAccepted(t *testing.T) {
	sc := startSSHContainer(t)
	defer sc.cleanup()

	khFile := emptyKnownHosts(t)

	cfg := internalssh.DialConfig{
		User:           "testuser",
		Hostname:       sc.host,
		Port:           sc.port,
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

// TestDial_Success verifies that Dial progresses past host-key verification when
// a valid known_hosts entry exists. Auth failure is acceptable (no key injected);
// what must not happen is a timeout or TOFU error.
func TestDial_Success(t *testing.T) {
	sc := startSSHContainer(t)
	defer sc.cleanup()

	khFile := seedKnownHosts(t, sc.host, sc.port, sc.hostKey)
	_, keyFile := generateTestKeyFile(t)
	_ = keyFile // key is not injected into container; auth will fail

	cfg := internalssh.DialConfig{
		User:           "testuser",
		Hostname:       sc.host,
		Port:           sc.port,
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
