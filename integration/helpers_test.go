//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Package-level container variables, populated by TestMain and shared across all tests.
var (
	sshA *sshContainer  // Container A: SSH-only (linuxserver/openssh-server)
	sshB *dinDContainer // Container B: DinD+SSH (custom Dockerfile.sshd)
)

// sshContainer holds a running SSH container for tests.
type sshContainer struct {
	host    string
	port    int
	hostKey gossh.PublicKey
	cleanup func()
}

// dinDContainer holds a running DinD+SSH container for tests.
// signers is keyed by username: "root", "sshuser", "nosudouser", "sudopassuser".
type dinDContainer struct {
	host    string
	port    int
	hostKey gossh.PublicKey
	cleanup func()
	signers map[string]gossh.Signer
}

// startSSHContainer launches an OpenSSH container and returns test connection info.
func startSSHContainer(t *testing.T) *sshContainer {
	t.Helper()
	c, err := newSSHContainer(context.Background())
	if err != nil {
		t.Fatalf("start SSH container: %v", err)
	}
	return c
}

// newSSHContainer is the internal startup helper used by TestMain (no *testing.T).
func newSSHContainer(ctx context.Context) (*sshContainer, error) {
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
		return nil, fmt.Errorf("start SSH container: %w", err)
	}

	mappedPort, err := container.MappedPort(ctx, "2222")
	if err != nil {
		container.Terminate(ctx) //nolint:errcheck
		return nil, fmt.Errorf("get mapped port: %w", err)
	}

	hostIP, err := container.Host(ctx)
	if err != nil {
		container.Terminate(ctx) //nolint:errcheck
		return nil, fmt.Errorf("get container host: %w", err)
	}

	portInt, err := strconv.Atoi(mappedPort.Port())
	if err != nil {
		container.Terminate(ctx) //nolint:errcheck
		return nil, fmt.Errorf("parse port %q: %w", mappedPort.Port(), err)
	}

	hostKey, err := captureHostKeyNoT(hostIP, portInt, "testuser", "testpass")
	if err != nil {
		container.Terminate(ctx) //nolint:errcheck
		return nil, fmt.Errorf("capture host key: %w", err)
	}

	return &sshContainer{
		host:    hostIP,
		port:    portInt,
		hostKey: hostKey,
		cleanup: func() { container.Terminate(ctx) }, //nolint:errcheck
	}, nil
}

// startDinDContainer builds and starts the custom DinD+SSH container.
func startDinDContainer(t *testing.T) *dinDContainer {
	t.Helper()
	c, err := newDinDContainer(context.Background())
	if err != nil {
		t.Fatalf("start DinD container: %v", err)
	}
	return c
}

// newDinDContainer is the internal startup helper used by TestMain (no *testing.T).
func newDinDContainer(ctx context.Context) (*dinDContainer, error) {
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "testdata/",
			Dockerfile: "Dockerfile.sshd",
			KeepImage:  false,
		},
		ExposedPorts: []string{"22/tcp"},
		WaitingFor:   wait.ForListeningPort("22/tcp").WithStartupTimeout(120 * time.Second),
		Privileged:   true, // required for Docker-in-Docker
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("start DinD container: %w", err)
	}

	mappedPort, err := container.MappedPort(ctx, "22")
	if err != nil {
		container.Terminate(ctx) //nolint:errcheck
		return nil, fmt.Errorf("get mapped port: %w", err)
	}

	hostIP, err := container.Host(ctx)
	if err != nil {
		container.Terminate(ctx) //nolint:errcheck
		return nil, fmt.Errorf("get container host: %w", err)
	}

	portInt, err := strconv.Atoi(mappedPort.Port())
	if err != nil {
		container.Terminate(ctx) //nolint:errcheck
		return nil, fmt.Errorf("parse port %q: %w", mappedPort.Port(), err)
	}

	// Capture host key using root credentials with initial insecure callback
	// (we have no known_hosts yet — this is the bootstrap capture step).
	hostKey, err := captureHostKeyFromContainer(ctx, container, hostIP, portInt)
	if err != nil {
		container.Terminate(ctx) //nolint:errcheck
		return nil, fmt.Errorf("capture host key: %w", err)
	}

	// Extract per-user private keys from the container via exec (no SSH dial needed).
	users := []string{"root", "sshuser", "nosudouser", "sudopassuser"}
	signers := make(map[string]gossh.Signer, len(users))
	for _, user := range users {
		keyPath := fmt.Sprintf("/etc/ssh/test_keys/%s_rsa", user)
		exitCode, reader, err := container.Exec(ctx, []string{"cat", keyPath})
		if err != nil {
			container.Terminate(ctx) //nolint:errcheck
			return nil, fmt.Errorf("exec cat %s: %w", keyPath, err)
		}
		if exitCode != 0 {
			container.Terminate(ctx) //nolint:errcheck
			return nil, fmt.Errorf("cat %s exited with code %d", keyPath, exitCode)
		}
		keyBytes, err := io.ReadAll(reader)
		if err != nil {
			container.Terminate(ctx) //nolint:errcheck
			return nil, fmt.Errorf("read key bytes for %s: %w", user, err)
		}
		signer, err := gossh.ParsePrivateKey(keyBytes)
		if err != nil {
			container.Terminate(ctx) //nolint:errcheck
			return nil, fmt.Errorf("parse private key for %s: %w", user, err)
		}
		signers[user] = signer
	}

	return &dinDContainer{
		host:    hostIP,
		port:    portInt,
		hostKey: hostKey,
		cleanup: func() { container.Terminate(ctx) }, //nolint:errcheck
		signers: signers,
	}, nil
}

// captureHostKeyFromContainer captures the SSH host key from a running container
// using a short-lived connection that captures the key then intentionally disconnects.
func captureHostKeyFromContainer(ctx context.Context, _ testcontainers.Container, host string, port int) (gossh.PublicKey, error) {
	var captured gossh.PublicKey
	cfg := &gossh.ClientConfig{
		User: "root",
		Auth: []gossh.AuthMethod{gossh.Password("")},
		HostKeyCallback: func(_ string, _ net.Addr, key gossh.PublicKey) error {
			captured = key
			// Return error to immediately disconnect after key capture.
			return fmt.Errorf("key captured")
		},
		Timeout: 15 * time.Second,
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	gossh.Dial("tcp", addr, cfg) //nolint:errcheck — intentional disconnect
	if captured == nil {
		return nil, fmt.Errorf("failed to capture SSH host key from DinD container")
	}
	return captured, nil
}

// captureHostKey dials the SSH server to capture its host public key, then disconnects.
func captureHostKey(t *testing.T, host string, port int) gossh.PublicKey {
	t.Helper()
	key, err := captureHostKeyNoT(host, port, "testuser", "testpass")
	if err != nil {
		t.Fatalf("captureHostKey: %v", err)
	}
	return key
}

// captureHostKeyNoT is the t-free variant used by newSSHContainer.
func captureHostKeyNoT(host string, port int, user, password string) (gossh.PublicKey, error) {
	var captured gossh.PublicKey
	cfg := &gossh.ClientConfig{
		User: user,
		Auth: []gossh.AuthMethod{gossh.Password(password)},
		HostKeyCallback: func(_ string, _ net.Addr, key gossh.PublicKey) error {
			captured = key
			// Return error to immediately terminate the connection after key capture.
			return fmt.Errorf("key captured")
		},
		Timeout: 15 * time.Second,
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	gossh.Dial("tcp", addr, cfg) //nolint:errcheck — intentional disconnect
	if captured == nil {
		return nil, fmt.Errorf("failed to capture SSH host key from %s", addr)
	}
	return captured, nil
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

// TestMain starts both containers once, runs all tests, then tears down.
func TestMain(m *testing.M) {
	ctx := context.Background()

	// Start Container A (SSH-only: linuxserver/openssh-server).
	var errA error
	sshA, errA = newSSHContainer(ctx)
	if errA != nil {
		log.Fatalf("TestMain: failed to start Container A (SSH-only): %v", errA)
	}

	// Start Container B (DinD+SSH: custom Dockerfile.sshd).
	var errB error
	sshB, errB = newDinDContainer(ctx)
	if errB != nil {
		if sshA != nil {
			sshA.cleanup()
		}
		log.Fatalf("TestMain: failed to start Container B (DinD+SSH): %v", errB)
	}

	code := m.Run()

	// Teardown.
	if sshA != nil {
		sshA.cleanup()
	}
	if sshB != nil {
		sshB.cleanup()
	}

	os.Exit(code)
}

// dialContainer dials Container B (DinD+SSH) as the specified user and returns a
// ready *gossh.Client. The client is closed automatically when the test ends.
// HostKeyCallback uses knownhosts.New() — never InsecureIgnoreHostKey (CLAUDE.md Rule 1).
func dialContainer(t *testing.T, user string) *gossh.Client {
	t.Helper()
	signer, ok := sshB.signers[user]
	if !ok {
		t.Fatalf("no signer for user %q", user)
	}
	khFile := seedKnownHosts(t, sshB.host, sshB.port, sshB.hostKey)
	kh, err := knownhosts.New(khFile)
	if err != nil {
		t.Fatalf("knownhosts.New: %v", err)
	}
	clientCfg := &gossh.ClientConfig{
		User:            user,
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: kh,
		Timeout:         15 * time.Second,
	}
	client, err := gossh.Dial("tcp", fmt.Sprintf("%s:%d", sshB.host, sshB.port), clientCfg)
	if err != nil {
		t.Fatalf("dial container as %q: %v", user, err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

// sshExecHelper runs a command on the remote host via a new SSH session.
// Per CLAUDE.md Rule 3, each call creates its own NewSession().
func sshExecHelper(t *testing.T, client *gossh.Client, cmd string) {
	t.Helper()
	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer session.Close()
	if err := session.Run(cmd); err != nil {
		t.Fatalf("run %q: %v", cmd, err)
	}
}

// sshExecOutputHelper runs a command and returns its stdout as a string.
// Per CLAUDE.md Rule 3, each call creates its own NewSession().
func sshExecOutputHelper(t *testing.T, client *gossh.Client, cmd string) string {
	t.Helper()
	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer session.Close()
	out, err := session.Output(cmd)
	if err != nil {
		t.Fatalf("output %q: %v", cmd, err)
	}
	return string(out)
}

// captureStderr redirects os.Stderr during fn(), captures the output, and returns it.
// Used for CHECK-03 and CHECK-07 warning assertions in preflight_test.go.
func captureStderr(fn func()) string {
	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}
