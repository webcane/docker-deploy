package compose

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	gossh "golang.org/x/crypto/ssh"
)

// mockComposeServer is a minimal in-process SSH server that records all
// commands executed via SSH exec sessions, supports configurable exit codes,
// and counts how many sessions have been opened.
type mockComposeServer struct {
	mu           sync.Mutex
	commands     []string
	exitCode     uint32
	sessionCount int32 // atomic
}

func newMockComposeServer(exitCode uint32) *mockComposeServer {
	return &mockComposeServer{exitCode: exitCode}
}

func (m *mockComposeServer) record(cmd string) {
	m.mu.Lock()
	m.commands = append(m.commands, cmd)
	m.mu.Unlock()
}

func (m *mockComposeServer) getCommands() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]string, len(m.commands))
	copy(cp, m.commands)
	return cp
}

func (m *mockComposeServer) incrementSession() {
	atomic.AddInt32(&m.sessionCount, 1)
}

func (m *mockComposeServer) getSessionCount() int {
	return int(atomic.LoadInt32(&m.sessionCount))
}

// startMockComposeSSHServer starts an in-process SSH server and returns a
// connected *gossh.Client. Exec sessions are handled by recording the command
// and returning the configured exit code.
func startMockComposeSSHServer(t *testing.T, srv *mockComposeServer) *gossh.Client {
	t.Helper()

	hostKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	hostSigner, err := gossh.NewSignerFromKey(hostKey)
	if err != nil {
		t.Fatalf("host signer: %v", err)
	}

	serverCfg := &gossh.ServerConfig{
		PasswordCallback: func(c gossh.ConnMetadata, pass []byte) (*gossh.Permissions, error) {
			return nil, nil
		},
		PublicKeyCallback: func(c gossh.ConnMetadata, key gossh.PublicKey) (*gossh.Permissions, error) {
			return nil, nil
		},
	}
	serverCfg.AddHostKey(hostSigner)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() }) //nolint:errcheck

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleComposeConn(conn, serverCfg, srv)
		}
	}()

	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}
	clientSigner, err := gossh.NewSignerFromKey(clientKey)
	if err != nil {
		t.Fatalf("client signer: %v", err)
	}

	clientCfg := &gossh.ClientConfig{
		User: "testuser",
		Auth: []gossh.AuthMethod{gossh.PublicKeys(clientSigner)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint — test-only
	}

	client, err := gossh.Dial("tcp", ln.Addr().String(), clientCfg)
	if err != nil {
		t.Fatalf("dial mock SSH server: %v", err)
	}
	t.Cleanup(func() { client.Close() }) //nolint:errcheck
	return client
}

func handleComposeConn(conn net.Conn, cfg *gossh.ServerConfig, srv *mockComposeServer) {
	sshConn, chans, reqs, err := gossh.NewServerConn(conn, cfg)
	if err != nil {
		return
	}
	defer sshConn.Close() //nolint:errcheck
	go gossh.DiscardRequests(reqs)

	for newChan := range chans {
		switch newChan.ChannelType() {
		case "session":
			ch, requests, err := newChan.Accept()
			if err != nil {
				continue
			}
			srv.incrementSession()
			go handleComposeSession(ch, requests, srv)
		default:
			newChan.Reject(gossh.UnknownChannelType, "unsupported") //nolint:errcheck
		}
	}
}

func handleComposeSession(ch gossh.Channel, requests <-chan *gossh.Request, srv *mockComposeServer) {
	defer ch.Close() //nolint:errcheck
	for req := range requests {
		switch req.Type {
		case "pty-req":
			// Accept PTY request without doing anything special in the mock.
			req.Reply(true, nil) //nolint:errcheck

		case "exec":
			if len(req.Payload) < 4 {
				req.Reply(false, nil) //nolint:errcheck
				continue
			}
			cmdLen := int(req.Payload[0])<<24 | int(req.Payload[1])<<16 |
				int(req.Payload[2])<<8 | int(req.Payload[3])
			if 4+cmdLen > len(req.Payload) {
				req.Reply(false, nil) //nolint:errcheck
				continue
			}
			cmd := string(req.Payload[4 : 4+cmdLen])
			srv.record(cmd)
			req.Reply(true, nil) //nolint:errcheck

			// Write a small output message to stdout.
			io.WriteString(ch, "mock compose output\n") //nolint:errcheck

			// Send the configured exit code.
			exitMsg := gossh.Marshal(struct{ Code uint32 }{srv.exitCode})
			ch.SendRequest("exit-status", false, exitMsg) //nolint:errcheck
			return

		default:
			req.Reply(false, nil) //nolint:errcheck
		}
	}
}

// TestRunCompose_CommandConstruction verifies that RunCompose builds the
// correct SSH exec command for the given remotePath and composeFile.
func TestRunCompose_CommandConstruction(t *testing.T) {
	srv := newMockComposeServer(0)
	client := startMockComposeSSHServer(t, srv)

	err := RunCompose(context.Background(), client, "/opt/myapp", "compose.yaml")
	if err != nil {
		t.Fatalf("RunCompose returned unexpected error: %v", err)
	}

	cmds := srv.getCommands()
	if len(cmds) == 0 {
		t.Fatal("no commands recorded by mock server")
	}
	// CR-01 fix: both remotePath and composeFile are combined and shell-quoted
	// as a single token, so the full path is wrapped in one set of quotes.
	want := "docker compose -f '/opt/myapp/compose.yaml' up -d --remove-orphans"
	if cmds[0] != want {
		t.Errorf("command = %q; want %q", cmds[0], want)
	}
}

// TestRunCompose_ExitCodeZero verifies that a zero exit code results in nil error.
func TestRunCompose_ExitCodeZero(t *testing.T) {
	srv := newMockComposeServer(0)
	client := startMockComposeSSHServer(t, srv)

	err := RunCompose(context.Background(), client, "/opt/myapp", "compose.yaml")
	if err != nil {
		t.Errorf("RunCompose returned non-nil error for exit 0: %v", err)
	}
}

// TestRunCompose_ExitCodeNonZero verifies that a non-zero exit code results in
// a non-nil error containing "docker compose exited with code 1".
func TestRunCompose_ExitCodeNonZero(t *testing.T) {
	srv := newMockComposeServer(1)
	client := startMockComposeSSHServer(t, srv)

	err := RunCompose(context.Background(), client, "/opt/myapp", "compose.yaml")
	if err == nil {
		t.Fatal("RunCompose returned nil error for exit 1; want non-nil")
	}
	if !strings.Contains(err.Error(), "docker compose exited with code 1") {
		t.Errorf("error message %q does not contain 'docker compose exited with code 1'", err.Error())
	}
}

// TestRunCompose_ShellQuoteRemotePath verifies that a path containing spaces is
// properly single-quoted in the exec command.
func TestRunCompose_ShellQuoteRemotePath(t *testing.T) {
	srv := newMockComposeServer(0)
	client := startMockComposeSSHServer(t, srv)

	err := RunCompose(context.Background(), client, "/opt/my app", "compose.yaml")
	if err != nil {
		t.Fatalf("RunCompose returned unexpected error: %v", err)
	}

	cmds := srv.getCommands()
	if len(cmds) == 0 {
		t.Fatal("no commands recorded by mock server")
	}
	// CR-01 fix: the full path (remotePath + "/" + composeFile) is combined
	// and shell-quoted as a single token, so check for the combined quoted form.
	if !strings.Contains(cmds[0], "'/opt/my app/compose.yaml'") {
		t.Errorf("command %q does not contain quoted combined path '/opt/my app/compose.yaml'", cmds[0])
	}
}

// TestRunCompose_NewSessionPerCall verifies that two sequential RunCompose calls
// each open a new SSH session (session counter increments to 2).
func TestRunCompose_NewSessionPerCall(t *testing.T) {
	srv := newMockComposeServer(0)
	client := startMockComposeSSHServer(t, srv)

	if err := RunCompose(context.Background(), client, "/opt/myapp", "compose.yaml"); err != nil {
		t.Fatalf("first RunCompose: %v", err)
	}
	if err := RunCompose(context.Background(), client, "/opt/myapp", "compose.yaml"); err != nil {
		t.Fatalf("second RunCompose: %v", err)
	}

	if got := srv.getSessionCount(); got != 2 {
		t.Errorf("session count = %d; want 2", got)
	}
}
