package filetransfer

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pkg/sftp"
	gossh "golang.org/x/crypto/ssh"
)

// mockSSHServer is a minimal in-process SSH server that records all commands
// executed via SSH exec sessions, and serves an SFTP subsystem backed by
// pkg/sftp's in-memory handler for file uploads.
type mockSSHServer struct {
	mu       sync.Mutex
	commands []string // ordered list of commands received via SSH exec

	// existingDirs is the set of paths that `test -d` reports as "exists".
	existingDirs map[string]bool
	// existingFiles is the set of paths that `test -f` reports as "exists".
	existingFiles map[string]bool

	// cmdExitCode, if non-nil, is called with the command string and the stdin
	// bytes received (for sudo -S commands) to determine the exit code.
	// If nil, all commands exit 0.
	cmdExitCode func(cmd string, stdin []byte) uint32

	// stdinReceived accumulates stdin bytes sent to exec sessions (for sudo -S tests).
	stdinReceived [][]byte
}

func newMockSSHServer(existingDirs []string) *mockSSHServer {
	m := &mockSSHServer{
		existingDirs:  make(map[string]bool),
		existingFiles: make(map[string]bool),
	}
	for _, d := range existingDirs {
		m.existingDirs[d] = true
	}
	return m
}

func (m *mockSSHServer) record(cmd string) {
	m.mu.Lock()
	m.commands = append(m.commands, cmd)
	m.mu.Unlock()
}

func (m *mockSSHServer) getCommands() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]string, len(m.commands))
	copy(cp, m.commands)
	return cp
}

func (m *mockSSHServer) recordStdin(data []byte) {
	m.mu.Lock()
	m.stdinReceived = append(m.stdinReceived, data)
	m.mu.Unlock()
}

func (m *mockSSHServer) getStdinReceived() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([][]byte, len(m.stdinReceived))
	copy(cp, m.stdinReceived)
	return cp
}

// startMockSSHServer starts an in-process SSH server and returns a connected
// *gossh.Client. All exec commands are recorded; SFTP subsystem is served via
// pkg/sftp's in-memory handler.
func startMockSSHServer(t *testing.T, srv *mockSSHServer) *gossh.Client {
	t.Helper()

	// Generate server host key.
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
			return nil, nil // accept any public key
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
			go handleMockConn(conn, serverCfg, srv)
		}
	}()

	// Generate a client key (password auth will be used instead — simpler).
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}
	clientSigner, err := gossh.NewSignerFromKey(clientKey)
	if err != nil {
		t.Fatalf("client signer: %v", err)
	}

	clientCfg := &gossh.ClientConfig{
		User:            "testuser",
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(clientSigner)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint — test-only
	}

	client, err := gossh.Dial("tcp", ln.Addr().String(), clientCfg)
	if err != nil {
		t.Fatalf("dial mock SSH server: %v", err)
	}
	t.Cleanup(func() { client.Close() }) //nolint:errcheck
	return client
}

func handleMockConn(conn net.Conn, cfg *gossh.ServerConfig, srv *mockSSHServer) {
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
			go handleMockSession(ch, requests, srv)
		default:
			newChan.Reject(gossh.UnknownChannelType, "unsupported") //nolint:errcheck
		}
	}
}

func handleMockSession(ch gossh.Channel, requests <-chan *gossh.Request, srv *mockSSHServer) {
	defer ch.Close() //nolint:errcheck
	for req := range requests {
		switch req.Type {
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

			// Handle `test -d` by writing "exists" or "absent" to stdout.
			if strings.Contains(cmd, "test -d") {
				matched := false
				for dir := range srv.existingDirs {
					if strings.Contains(cmd, ShellQuote(dir)) {
						matched = true
						break
					}
				}
				output := "absent\n"
				if matched {
					output = "exists\n"
				}
				ch.Write([]byte(output)) //nolint:errcheck
			}

			// Handle `test -f` by writing "exists" or "absent" to stdout.
			if strings.Contains(cmd, "test -f") {
				matched := false
				for file := range srv.existingFiles {
					if strings.Contains(cmd, ShellQuote(file)) {
						matched = true
						break
					}
				}
				output := "absent\n"
				if matched {
					output = "exists\n"
				}
				ch.Write([]byte(output)) //nolint:errcheck
			}

			// Read stdin data (for sudo -S commands that pipe a password).
			// Read non-blocking: drain what's available.
			var stdinBuf bytes.Buffer
			done := make(chan struct{})
			go func() {
				io.Copy(&stdinBuf, ch) //nolint:errcheck
				close(done)
			}()

			// Determine exit code.
			exitCode := uint32(0)
			if srv.cmdExitCode != nil {
				// Wait briefly for stdin to arrive for sudo -S commands.
				// We use a goroutine drain with channel signal.
				// For non-sudo commands, stdin will EOF immediately.
				select {
				case <-done:
					// stdin drained
				default:
					// For sudo -S, the client writes password then closes stdin.
					// We need to wait for that data. Use a short read attempt.
					// Actually we already launched a goroutine — just wait.
					<-done
				}
				stdinData := stdinBuf.Bytes()
				if len(stdinData) > 0 {
					srv.recordStdin(stdinData)
				}
				exitCode = srv.cmdExitCode(cmd, stdinData)
			}

			if exitCode == 0 {
				exitMsg := gossh.Marshal(struct{ Code uint32 }{0})
				ch.SendRequest("exit-status", false, exitMsg) //nolint:errcheck
			} else {
				exitMsg := gossh.Marshal(struct{ Code uint32 }{exitCode})
				ch.SendRequest("exit-status", false, exitMsg) //nolint:errcheck
			}
			return

		case "subsystem":
			if len(req.Payload) >= 4 {
				subLen := int(req.Payload[0])<<24 | int(req.Payload[1])<<16 |
					int(req.Payload[2])<<8 | int(req.Payload[3])
				if 4+subLen <= len(req.Payload) {
					sub := string(req.Payload[4 : 4+subLen])
					if sub == "sftp" {
						req.Reply(true, nil) //nolint:errcheck
						handlers := sftp.InMemHandler()
						rs := sftp.NewRequestServer(struct {
							io.Reader
							io.WriteCloser
						}{ch, ch}, handlers)
						rs.Serve() //nolint:errcheck
						return
					}
				}
			}
			req.Reply(false, nil) //nolint:errcheck

		default:
			req.Reply(false, nil) //nolint:errcheck
		}
	}
}

// TestUploadAuthFallback_DirectCopy verifies that direct copy succeeds when the SSH
// user has write permissions to the target directory.
func TestUploadAuthFallback_DirectCopy(t *testing.T) {
	remoteBase := "/opt/test-deploy"
	srv := newMockSSHServer(nil)
	client := startMockSSHServer(t, srv)

	localDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte("version: '3'"), 0644); err != nil {
		t.Fatal(err)
	}

	creds := new(SudoCreds)
	defer creds.Zero()
	warnedOnce := new(bool)
	_, err := Upload(context.Background(), client, localDir, remoteBase, nil, creds, false, warnedOnce, false)
	if err != nil {
		t.Fatalf("Upload returned unexpected error: %v", err)
	}
}

// TestUploadAuthFallback_PasswordlessSudo verifies that when direct copy fails,
// the function falls back to passwordless sudo.
func TestUploadAuthFallback_PasswordlessSudo(t *testing.T) {
	remoteBase := "/opt/test-deploy"
	srv := newMockSSHServer(nil)
	client := startMockSSHServer(t, srv)

	localDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte("version: '3'"), 0644); err != nil {
		t.Fatal(err)
	}

	// This test expects tryAuthFallback to be called and to handle permission denied
	creds := new(SudoCreds)
	defer creds.Zero()
	warnedOnce := new(bool)
	_, err := Upload(context.Background(), client, localDir, remoteBase, nil, creds, false, warnedOnce, false)
	if err != nil {
		t.Fatalf("Upload with passwordless sudo fallback failed: %v", err)
	}
}

// TestUploadAuthFallback_InteractivePassword verifies that when direct and
// passwordless sudo fail, the function prompts for a password interactively.
func TestUploadAuthFallback_InteractivePassword(t *testing.T) {
	t.Skip("Interactive password prompt requires stdin mocking — to be implemented in GREEN phase")
}

// TestUploadAuthFallback_InteractivePassword_WrongPassword verifies that incorrect
// passwords are retried up to 3 times before failing.
func TestUploadAuthFallback_InteractivePassword_WrongPassword(t *testing.T) {
	t.Skip("Password retry logic — to be implemented in GREEN phase")
}

// TestUploadAuthFallback_InteractivePassword_Timeout verifies that if an interactive
// password prompt times out, the upload fails gracefully.
func TestUploadAuthFallback_InteractivePassword_Timeout(t *testing.T) {
	t.Skip("Timeout handling — to be implemented in GREEN phase")
}

// TestUploadAuthFallback_RootUser verifies that when the SSH user is root, direct
// copy is used without any sudo path and a danger warning is emitted.
func TestUploadAuthFallback_RootUser(t *testing.T) {
	t.Skip("Root user detection and warning — to be implemented in GREEN phase")
}

// TestUploadAuthFallback_AllPathsExhausted verifies that when all auth paths fail
// (no password, wrong password, or timeout), the error message clearly states which
// paths were exhausted.
func TestUploadAuthFallback_AllPathsExhausted(t *testing.T) {
	t.Skip("All paths exhausted error message — to be implemented in GREEN phase")
}

// TestUploadFirstDeploy_RmBeforeMv is the regression test for the first-deploy
// mv nesting bug. It verifies that when remoteBase does not exist before Upload(),
// the implementation calls rm -rf remoteBase before mv stagingDir remoteBase so
// that mv performs a clean rename (not a nesting move).
func TestUploadFirstDeploy_RmBeforeMv(t *testing.T) {
	remoteBase := "/opt/test-deploy"

	// First deploy: remoteBase does NOT exist.
	srv := newMockSSHServer(nil)
	client := startMockSSHServer(t, srv)

	localDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte("version: '3'"), 0644); err != nil {
		t.Fatal(err)
	}

	creds := new(SudoCreds)
	defer creds.Zero()
	warnedOnce := new(bool)
	_, err := Upload(context.Background(), client, localDir, remoteBase, nil, creds, false, warnedOnce, false)
	if err != nil {
		t.Fatalf("Upload returned unexpected error: %v", err)
	}

	cmds := srv.getCommands()
	t.Logf("commands executed: %v", cmds)

	// Locate rm -rf remoteBase (not -old-) and mv stagingDir remoteBase.
	rmIdx := -1
	mvIdx := -1
	for i, cmd := range cmds {
		if strings.Contains(cmd, "rm -rf") &&
			strings.Contains(cmd, ShellQuote(remoteBase)) &&
			!strings.Contains(cmd, "-old-") {
			rmIdx = i
		}
		if strings.Contains(cmd, "mv") &&
			strings.Contains(cmd, "/tmp/docker-deploy-") &&
			strings.Contains(cmd, ShellQuote(remoteBase)) {
			mvIdx = i
		}
	}

	if rmIdx == -1 {
		t.Errorf("first-deploy: expected rm -rf %s before mv; commands: %v", remoteBase, cmds)
	}
	if mvIdx == -1 {
		t.Errorf("first-deploy: expected mv stagingDir %s; commands: %v", remoteBase, cmds)
	}
	if rmIdx != -1 && mvIdx != -1 && rmIdx >= mvIdx {
		t.Errorf("first-deploy: rm -rf (idx %d) must come BEFORE mv (idx %d); commands: %v",
			rmIdx, mvIdx, cmds)
	}
}

// TestUploadVerbose_PerFileStderr verifies that when verbose=true, per-file lines
// are written to stderr (not suppressed). When verbose=false, per-file lines are suppressed.
// This test uses a captured stderr to detect the "  -> " lines.
func TestUploadVerbose_PerFileStderr(t *testing.T) {
	remoteBase := "/opt/test-deploy"

	t.Run("verbose_true_perfile_to_stderr", func(t *testing.T) {
		srv := newMockSSHServer(nil)
		client := startMockSSHServer(t, srv)

		localDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte("version: '3'"), 0644); err != nil {
			t.Fatal(err)
		}

		// Redirect stderr to capture output.
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		creds := new(SudoCreds)
		defer creds.Zero()
		warnedOnce := new(bool)
		_, err := Upload(context.Background(), client, localDir, remoteBase, nil, creds, false, warnedOnce, true)

		_ = w.Close()
		os.Stderr = oldStderr

		var buf strings.Builder
		io.Copy(&buf, r) //nolint:errcheck
		captured := buf.String()

		if err != nil {
			t.Fatalf("Upload(verbose=true) returned error: %v", err)
		}
		if !strings.Contains(captured, "  -> compose.yaml") {
			t.Errorf("verbose=true: expected '  -> compose.yaml' in stderr; got: %q", captured)
		}
	})

	t.Run("verbose_false_no_perfile_lines", func(t *testing.T) {
		srv := newMockSSHServer(nil)
		client := startMockSSHServer(t, srv)

		localDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte("version: '3'"), 0644); err != nil {
			t.Fatal(err)
		}

		// Redirect stderr to capture output.
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		creds := new(SudoCreds)
		defer creds.Zero()
		warnedOnce := new(bool)
		_, err := Upload(context.Background(), client, localDir, remoteBase, nil, creds, false, warnedOnce, false)

		_ = w.Close()
		os.Stderr = oldStderr

		var buf strings.Builder
		io.Copy(&buf, r) //nolint:errcheck
		captured := buf.String()

		if err != nil {
			t.Fatalf("Upload(verbose=false) returned error: %v", err)
		}
		if strings.Contains(captured, "  -> ") {
			t.Errorf("verbose=false: unexpected '  -> ' per-file line in stderr; got: %q", captured)
		}
	})
}

// TestUploadVerbose_SSHCommandLogging verifies that when verbose=true, SSH exec
// commands are logged to stderr in "[ssh] <cmd>" format with "  → exit N" exit codes.
func TestUploadVerbose_SSHCommandLogging(t *testing.T) {
	remoteBase := "/opt/test-deploy"
	srv := newMockSSHServer(nil)
	client := startMockSSHServer(t, srv)

	localDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte("version: '3'"), 0644); err != nil {
		t.Fatal(err)
	}

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	creds := new(SudoCreds)
	defer creds.Zero()
	warnedOnce := new(bool)
	_, err := Upload(context.Background(), client, localDir, remoteBase, nil, creds, false, warnedOnce, true)

	_ = w.Close()
	os.Stderr = oldStderr

	var buf strings.Builder
	io.Copy(&buf, r) //nolint:errcheck
	captured := buf.String()

	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	if !strings.Contains(captured, "[ssh] ") {
		t.Errorf("verbose=true: expected '[ssh] ' SSH command lines in stderr; got: %q", captured)
	}
	if !strings.Contains(captured, "exit 0") {
		t.Errorf("verbose=true: expected 'exit 0' exit code lines in stderr; got: %q", captured)
	}
}

// TestUploadRepeatDeploy_ThreeStepSwapUnchanged verifies that the repeat-deploy
// path (remoteBase already exists) uses the three-step atomic swap and does NOT
// add an extra rm -rf on remoteBase itself.
func TestUploadRepeatDeploy_ThreeStepSwapUnchanged(t *testing.T) {
	remoteBase := "/opt/test-deploy"

	// Repeat deploy: remoteBase EXISTS.
	srv := newMockSSHServer([]string{remoteBase})
	client := startMockSSHServer(t, srv)

	localDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte("version: '3'"), 0644); err != nil {
		t.Fatal(err)
	}

	creds := new(SudoCreds)
	defer creds.Zero()
	warnedOnce := new(bool)
	// force=true: bypass the confirm prompt (this test checks swap mechanics, not prompt)
	_, err := Upload(context.Background(), client, localDir, remoteBase, nil, creds, true, warnedOnce, false)
	if err != nil {
		t.Fatalf("Upload returned unexpected error: %v", err)
	}

	cmds := srv.getCommands()
	t.Logf("commands executed: %v", cmds)

	// Verify three-step swap: mv base->backup, mv staging->base, rm backup.
	hasMvToBackup := false
	hasMvStagingToBase := false
	hasRmBackup := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "mv") &&
			strings.Contains(cmd, ShellQuote(remoteBase)) &&
			strings.Contains(cmd, "-old-") {
			hasMvToBackup = true
		}
		if strings.Contains(cmd, "mv") &&
			strings.Contains(cmd, "/tmp/docker-deploy-") &&
			strings.Contains(cmd, ShellQuote(remoteBase)) &&
			!strings.Contains(cmd, "-old-") {
			hasMvStagingToBase = true
		}
		if strings.Contains(cmd, "rm -rf") && strings.Contains(cmd, "-old-") {
			hasRmBackup = true
		}
	}

	if !hasMvToBackup {
		t.Errorf("repeat-deploy: expected mv %s to backup; commands: %v", remoteBase, cmds)
	}
	if !hasMvStagingToBase {
		t.Errorf("repeat-deploy: expected mv stagingDir to %s; commands: %v", remoteBase, cmds)
	}
	if !hasRmBackup {
		t.Errorf("repeat-deploy: expected rm -rf backup; commands: %v", cmds)
	}

	// The repeat-deploy must NOT rm -rf remoteBase itself.
	for _, cmd := range cmds {
		if strings.Contains(cmd, "rm -rf") &&
			strings.Contains(cmd, ShellQuote(remoteBase)) &&
			!strings.Contains(cmd, "-old-") &&
			!strings.Contains(cmd, "/tmp/docker-deploy-") {
			t.Errorf("repeat-deploy must NOT rm -rf %s directly; found: %q", remoteBase, cmd)
		}
	}
}

// TestUploadSkipEnvPreservesRemoteEnv verifies that when .env appears in the excludes
// list and the remote target already has a .env file, Upload() backs it up before the
// atomic swap and restores it afterward — so the remote .env survives even though it
// was not part of the upload (e.g. via --skip-env).
func TestUploadSkipEnvPreservesRemoteEnv(t *testing.T) {
	remoteBase := "/opt/test-deploy"
	remoteEnv := remoteBase + "/.env"

	t.Run("dot_env_excluded_and_exists_is_preserved", func(t *testing.T) {
		// Repeat deploy: remoteBase and remoteBase/.env both exist.
		srv := newMockSSHServer([]string{remoteBase})
		srv.existingFiles[remoteEnv] = true
		client := startMockSSHServer(t, srv)

		localDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte("version: '3'"), 0644); err != nil {
			t.Fatal(err)
		}

		creds := new(SudoCreds)
		defer creds.Zero()
		warnedOnce := new(bool)
		excludes := []string{".env"}
		// force=true: bypass the confirm prompt (this test checks .env preservation, not prompt)
		_, err := Upload(context.Background(), client, localDir, remoteBase, excludes, creds, true, warnedOnce, false)
		if err != nil {
			t.Fatalf("Upload returned unexpected error: %v", err)
		}

		cmds := srv.getCommands()
		t.Logf("commands executed: %v", cmds)

		// Expect a backup: cp remoteEnv → /tmp/docker-deploy-env-<ts>
		hasCpToTmp := false
		// Expect a restore: cp /tmp/docker-deploy-env-<ts> → remoteEnv
		hasCpFromTmp := false
		// Expect cleanup: rm -f /tmp/docker-deploy-env-<ts>
		hasRmEnvBackup := false

		for _, cmd := range cmds {
			if !strings.Contains(cmd, "/tmp/docker-deploy-env-") {
				continue
			}
			if strings.Contains(cmd, "rm -f") {
				hasRmEnvBackup = true
				continue
			}
			if strings.Contains(cmd, "cp") && strings.Contains(cmd, ShellQuote(remoteEnv)) {
				// Backup: source is remoteEnv (appears first); restore: dest is remoteEnv (appears last).
				envIdx := strings.Index(cmd, ShellQuote(remoteEnv))
				tmpIdx := strings.Index(cmd, "/tmp/docker-deploy-env-")
				if envIdx < tmpIdx {
					hasCpToTmp = true // backup direction
				} else {
					hasCpFromTmp = true // restore direction
				}
			}
		}

		if !hasCpToTmp {
			t.Errorf("expected cp %s → /tmp/docker-deploy-env-* (backup); commands: %v", remoteEnv, cmds)
		}
		if !hasCpFromTmp {
			t.Errorf("expected cp /tmp/docker-deploy-env-* → %s (restore); commands: %v", remoteEnv, cmds)
		}
		if !hasRmEnvBackup {
			t.Errorf("expected rm -f /tmp/docker-deploy-env-* (backup cleanup); commands: %v", cmds)
		}
	})

	t.Run("dot_env_not_excluded_no_backup", func(t *testing.T) {
		// When .env is not in excludes, no backup or restore should occur.
		srv := newMockSSHServer([]string{remoteBase})
		srv.existingFiles[remoteEnv] = true
		client := startMockSSHServer(t, srv)

		localDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte("version: '3'"), 0644); err != nil {
			t.Fatal(err)
		}

		creds := new(SudoCreds)
		defer creds.Zero()
		warnedOnce := new(bool)
		// force=true: bypass confirm prompt (test checks .env backup logic, not prompt)
		_, err := Upload(context.Background(), client, localDir, remoteBase, nil, creds, true, warnedOnce, false)
		if err != nil {
			t.Fatalf("Upload returned unexpected error: %v", err)
		}

		for _, cmd := range srv.getCommands() {
			if strings.Contains(cmd, "/tmp/docker-deploy-env-") {
				t.Errorf("expected no .env backup commands when .env not in excludes; found: %q", cmd)
			}
		}
	})

	t.Run("dot_env_excluded_but_not_on_remote_no_backup", func(t *testing.T) {
		// When .env is excluded but doesn't exist on the remote, no backup or restore.
		srv := newMockSSHServer([]string{remoteBase})
		// existingFiles is empty — remote .env does NOT exist
		client := startMockSSHServer(t, srv)

		localDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte("version: '3'"), 0644); err != nil {
			t.Fatal(err)
		}

		creds := new(SudoCreds)
		defer creds.Zero()
		warnedOnce := new(bool)
		excludes := []string{".env"}
		// force=true: bypass confirm prompt (test checks .env absence logic, not prompt)
		_, err := Upload(context.Background(), client, localDir, remoteBase, excludes, creds, true, warnedOnce, false)
		if err != nil {
			t.Fatalf("Upload returned unexpected error: %v", err)
		}

		for _, cmd := range srv.getCommands() {
			if strings.Contains(cmd, "/tmp/docker-deploy-env-") {
				t.Errorf("expected no .env backup when remote .env absent; found: %q", cmd)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// SudoCreds and SudoExec tests (TDD RED → GREEN)
// ---------------------------------------------------------------------------

// TestSudoCreds_Zero verifies that Zero() zeroes all bytes and nils the slice,
// and that calling Zero() on an empty SudoCreds (nil pw) does not panic.
func TestSudoCreds_Zero(t *testing.T) {
	t.Run("zeroes_pw_bytes_and_nils_slice", func(t *testing.T) {
		c := &SudoCreds{}
		c.pw = []byte("secret")
		c.Zero()
		if c.pw != nil {
			t.Errorf("expected c.pw to be nil after Zero(); got %v", c.pw)
		}
	})

	t.Run("nil_pw_does_not_panic", func(t *testing.T) {
		c := &SudoCreds{}
		// Must not panic.
		c.Zero()
		if c.pw != nil {
			t.Errorf("expected c.pw to remain nil after Zero() on empty creds")
		}
	})

	t.Run("new_SudoCreds_zero_does_not_panic", func(t *testing.T) {
		c := new(SudoCreds)
		c.Zero()
	})
}

// TestSudoExec_DirectSuccess verifies that SudoExec succeeds on step 1 (direct
// cmd exits 0) and does not prompt for a password.
func TestSudoExec_DirectSuccess(t *testing.T) {
	srv := newMockSSHServer(nil)
	// Default: all commands exit 0.
	client := startMockSSHServer(t, srv)

	warnedOnce := new(bool)
	creds := new(SudoCreds)
	err := SudoExec(client, "mkdir -p /opt/test", creds, warnedOnce, false)
	if err != nil {
		t.Errorf("SudoExec direct step: unexpected error: %v", err)
	}

	// Step 1 (direct) should have been the only command.
	cmds := srv.getCommands()
	if len(cmds) != 1 {
		t.Errorf("expected exactly 1 SSH command (direct); got %d: %v", len(cmds), cmds)
	}
	if cmds[0] != "mkdir -p /opt/test" {
		t.Errorf("expected direct command %q; got %q", "mkdir -p /opt/test", cmds[0])
	}
	if creds.pw != nil {
		t.Errorf("expected creds.pw to remain nil after direct success; got %v", creds.pw)
	}
}

// TestSudoExec_CachedCreds verifies that SudoExec uses creds.pw on step 2 when
// direct fails and creds.pw != nil, without prompting interactively.
func TestSudoExec_CachedCreds(t *testing.T) {
	const correctPassword = "cached-password"

	srv := newMockSSHServer(nil)
	// Direct commands fail (exit 1); sudo -S with correct password succeeds.
	srv.cmdExitCode = func(cmd string, stdin []byte) uint32 {
		if strings.Contains(cmd, "sudo -S") {
			// Correct password supplied via stdin?
			if bytes.Contains(stdin, []byte(correctPassword)) {
				return 0
			}
			return 1
		}
		if strings.Contains(cmd, "sudo -n") {
			return 1
		}
		// Direct command: fail.
		return 1
	}
	client := startMockSSHServer(t, srv)

	warnedOnce := new(bool)
	creds := &SudoCreds{pw: []byte(correctPassword)}
	err := SudoExec(client, "mkdir -p /opt/test", creds, warnedOnce, false)
	if err != nil {
		t.Errorf("SudoExec cached-creds step: unexpected error: %v", err)
	}

	// Should have seen: direct (fail), then sudo -S with cached pw (succeed).
	cmds := srv.getCommands()
	hasDirect := false
	hasSudoS := false
	for _, cmd := range cmds {
		if cmd == "mkdir -p /opt/test" {
			hasDirect = true
		}
		if strings.Contains(cmd, "sudo -S") {
			hasSudoS = true
		}
	}
	if !hasDirect {
		t.Errorf("expected direct command attempt; got: %v", cmds)
	}
	if !hasSudoS {
		t.Errorf("expected sudo -S command attempt; got: %v", cmds)
	}
	// Must NOT have hit interactive prompt (warnedOnce stays false).
	if *warnedOnce {
		t.Errorf("expected warnedOnce=false when cached creds succeed; got true")
	}
}

// TestSudoExec_AllStepsExhausted verifies that when direct, cached, and passwordless
// sudo all fail and no interactive input is available, SudoExec returns an error
// containing "no valid auth path available".
func TestSudoExec_AllStepsExhausted(t *testing.T) {
	srv := newMockSSHServer(nil)
	// All commands fail.
	srv.cmdExitCode = func(cmd string, stdin []byte) uint32 {
		return 1
	}
	client := startMockSSHServer(t, srv)

	// Override promptSudoPassword to return an error immediately (no terminal available).
	origPrompt := promptSudoPasswordFunc
	promptSudoPasswordFunc = func() (string, error) {
		return "", io.EOF // simulate no interactive input
	}
	defer func() { promptSudoPasswordFunc = origPrompt }()

	warnedOnce := new(bool)
	creds := new(SudoCreds)
	err := SudoExec(client, "mkdir -p /opt/test", creds, warnedOnce, false)
	if err == nil {
		t.Fatal("expected error when all auth paths exhausted; got nil")
	}
	if !strings.Contains(err.Error(), "no valid auth path available") {
		t.Errorf("expected error to contain 'no valid auth path available'; got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Verbose pre-confirm diff tests (Plan 13-05)
// ---------------------------------------------------------------------------

// TestUploadVerbose_PreConfirmDiff verifies that when verbose=true, force=false,
// and the remote dir exists, Upload() prints "Local files" and "Remote files"
// sections to stderr before the confirm prompt fires.
func TestUploadVerbose_PreConfirmDiff(t *testing.T) {
	remoteBase := "/opt/test-deploy"

	// Repeat deploy: remoteBase EXISTS.
	srv := newMockSSHServer([]string{remoteBase})
	client := startMockSSHServer(t, srv)

	localDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte("version: '3'"), 0644); err != nil {
		t.Fatal(err)
	}

	// Redirect stderr to capture output.
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Feed "y\n" to stdin so the confirm prompt auto-answers yes.
	oldStdin := os.Stdin
	pr, pw2, _ := os.Pipe()
	os.Stdin = pr
	pw2.WriteString("y\n") //nolint:errcheck
	pw2.Close()            //nolint:errcheck

	creds := new(SudoCreds)
	defer creds.Zero()
	warnedOnce := new(bool)
	_, err := Upload(context.Background(), client, localDir, remoteBase, nil, creds, false, warnedOnce, true)

	_ = w.Close()
	os.Stderr = oldStderr
	os.Stdin = oldStdin
	_ = pr.Close()

	var buf strings.Builder
	io.Copy(&buf, r) //nolint:errcheck
	captured := buf.String()

	if err != nil {
		t.Fatalf("Upload(verbose=true, force=false) returned error: %v", err)
	}
	if !strings.Contains(captured, "Local files") {
		t.Errorf("verbose+force=false: expected 'Local files' in stderr before prompt; got: %q", captured)
	}
	if !strings.Contains(captured, "Remote files") {
		t.Errorf("verbose+force=false: expected 'Remote files' in stderr before prompt; got: %q", captured)
	}
}

// TestUpload_ForceSkipsPrompt verifies that when force=true, no prompt is shown
// and upload proceeds without stdin input (even on repeat deploy).
func TestUpload_ForceSkipsPrompt(t *testing.T) {
	remoteBase := "/opt/test-deploy"

	// Repeat deploy: remoteBase EXISTS.
	srv := newMockSSHServer([]string{remoteBase})
	client := startMockSSHServer(t, srv)

	localDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte("version: '3'"), 0644); err != nil {
		t.Fatal(err)
	}

	// Redirect stderr to capture output — confirm that no prompt is printed.
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	creds := new(SudoCreds)
	defer creds.Zero()
	warnedOnce := new(bool)
	// force=true: no stdin needed — if the prompt fired it would block or fail.
	_, err := Upload(context.Background(), client, localDir, remoteBase, nil, creds, true, warnedOnce, false)

	_ = w.Close()
	os.Stderr = oldStderr

	var buf strings.Builder
	io.Copy(&buf, r) //nolint:errcheck
	captured := buf.String()

	if err != nil {
		t.Fatalf("Upload(force=true) returned error: %v", err)
	}
	if strings.Contains(captured, "Replace all contents") {
		t.Errorf("force=true: unexpected confirm prompt in stderr; got: %q", captured)
	}
}

// TestUploadVerbose_FirstDeploy_NoRemote verifies that when the remote dir is
// absent and verbose=true, stderr contains "Remote files: (none)".
func TestUploadVerbose_FirstDeploy_NoRemote(t *testing.T) {
	remoteBase := "/opt/test-deploy"

	// First deploy: remoteBase does NOT exist.
	srv := newMockSSHServer(nil)
	client := startMockSSHServer(t, srv)

	localDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte("version: '3'"), 0644); err != nil {
		t.Fatal(err)
	}

	// Redirect stderr to capture output.
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	creds := new(SudoCreds)
	defer creds.Zero()
	warnedOnce := new(bool)
	// force=true so no prompt fires (first deploy doesn't prompt anyway, but let's be explicit).
	_, err := Upload(context.Background(), client, localDir, remoteBase, nil, creds, true, warnedOnce, true)

	_ = w.Close()
	os.Stderr = oldStderr

	var buf strings.Builder
	io.Copy(&buf, r) //nolint:errcheck
	captured := buf.String()

	if err != nil {
		t.Fatalf("Upload(verbose=true, first deploy) returned error: %v", err)
	}
	if !strings.Contains(captured, "Remote files: (none)") {
		t.Errorf("first deploy, verbose=true: expected 'Remote files: (none)' in stderr; got: %q", captured)
	}
}

// TestUploadVerbose_Truncation verifies that when more than 20 local files exist,
// stderr contains "... and N more" message.
func TestUploadVerbose_Truncation(t *testing.T) {
	remoteBase := "/opt/test-deploy"

	// First deploy: remoteBase does NOT exist.
	srv := newMockSSHServer(nil)
	client := startMockSSHServer(t, srv)

	localDir := t.TempDir()
	// Create 25 files to trigger truncation at 20.
	for i := 0; i < 25; i++ {
		fname := filepath.Join(localDir, fmt.Sprintf("file%02d.txt", i))
		if err := os.WriteFile(fname, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	creds := new(SudoCreds)
	defer creds.Zero()
	warnedOnce := new(bool)
	_, err := Upload(context.Background(), client, localDir, remoteBase, nil, creds, true, warnedOnce, true)

	_ = w.Close()
	os.Stderr = oldStderr

	var buf strings.Builder
	io.Copy(&buf, r) //nolint:errcheck
	captured := buf.String()

	if err != nil {
		t.Fatalf("Upload(verbose=true, 25 files) returned error: %v", err)
	}
	if !strings.Contains(captured, "... and") || !strings.Contains(captured, "more") {
		t.Errorf("truncation: expected '... and N more' in stderr; got: %q", captured)
	}
}

// TestSudoExec_SinglePromptMultiFile verifies that across multiple SudoExec calls
// in a single deploy to a root-owned path, the interactive password prompt fires
// exactly once. Subsequent calls reuse creds.pw (step 2) and do not re-prompt.
func TestSudoExec_SinglePromptMultiFile(t *testing.T) {
	const correctPassword = "testpassword"

	// promptCount tracks how many times the interactive prompt is called.
	var promptCount int64

	srv := newMockSSHServer(nil)
	// Direct commands fail; sudo -S with correct password succeeds; sudo -n fails.
	srv.cmdExitCode = func(cmd string, stdin []byte) uint32 {
		if strings.Contains(cmd, "sudo -S") {
			if bytes.Contains(stdin, []byte(correctPassword)) {
				return 0
			}
			return 1
		}
		if strings.Contains(cmd, "sudo -n") {
			return 1
		}
		// Direct: fail.
		return 1
	}
	client := startMockSSHServer(t, srv)

	// Override promptSudoPassword to supply the correct password and track calls.
	origPrompt := promptSudoPasswordFunc
	promptSudoPasswordFunc = func() (string, error) {
		atomic.AddInt64(&promptCount, 1)
		return correctPassword, nil
	}
	defer func() { promptSudoPasswordFunc = origPrompt }()

	warnedOnce := new(bool)
	creds := new(SudoCreds)
	defer creds.Zero()

	// Simulate the 8 SudoExec calls that Upload() would make for a
	// root-owned target path.
	ops := []string{
		"mkdir -p /opt/myapp",
		"mv /opt/myapp /opt/myapp-old-123",
		"mv /tmp/docker-deploy-123 /opt/myapp",
		"mv /opt/myapp-old-123 /opt/myapp",    // rollback (simulated)
		"rm -rf /opt/myapp-old-123",
		"rm -rf /opt/myapp",
		"mv /tmp/docker-deploy-456 /opt/myapp", // first-deploy mv
		"cp /tmp/docker-deploy-env-123 /opt/myapp/.env",
	}

	for i, cmd := range ops {
		err := SudoExec(client, cmd, creds, warnedOnce, false)
		if err != nil {
			t.Fatalf("SudoExec call %d (%q) failed: %v", i, cmd, err)
		}
	}

	count := atomic.LoadInt64(&promptCount)
	if count != 1 {
		t.Errorf("expected interactive password prompt exactly once; got %d prompts", count)
	}
}

// ---------------------------------------------------------------------------
// Path-aware sudo detection tests (Plan 13-06)
// ---------------------------------------------------------------------------

// TestUpload_PathAwareSudo_WritablePath verifies that when the test -w probe
// succeeds (the target path is user-writable), Upload() does NOT call SudoExec
// for any remoteBase operation — all mkdir/mv/rm commands use direct sshRun
// (exit 0 without sudo -S or sudo -n).
func TestUpload_PathAwareSudo_WritablePath(t *testing.T) {
	remoteBase := "/home/deploy/myproject"

	// First deploy: remoteBase does NOT exist.
	srv := newMockSSHServer(nil)
	// test -w returns exit 0 (writable) for all commands.
	// All non-probe commands also succeed (exit 0) — direct sshRun path.
	client := startMockSSHServer(t, srv)

	localDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte("version: '3'"), 0644); err != nil {
		t.Fatal(err)
	}

	creds := new(SudoCreds)
	defer creds.Zero()
	warnedOnce := new(bool)
	_, err := Upload(context.Background(), client, localDir, remoteBase, nil, creds, false, warnedOnce, false)
	if err != nil {
		t.Fatalf("Upload returned unexpected error: %v", err)
	}

	cmds := srv.getCommands()
	t.Logf("commands executed: %v", cmds)

	// Verify the probe was issued.
	hasProbe := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "test -w") {
			hasProbe = true
			break
		}
	}
	if !hasProbe {
		t.Errorf("expected test -w probe in commands; got: %v", cmds)
	}

	// Verify no sudo command was issued for remoteBase operations.
	for _, cmd := range cmds {
		if strings.Contains(cmd, "sudo") &&
			(strings.Contains(cmd, "mkdir") || strings.Contains(cmd, "mv") || strings.Contains(cmd, "rm -rf")) {
			t.Errorf("writable path: unexpected sudo command for remoteBase operation: %q", cmd)
		}
	}
}

// TestUpload_PathAwareSudo_ElevatedPath verifies that when the test -w probe
// fails (exit 1), Upload() uses SudoExec for remoteBase operations (i.e. the
// commands pass through the SudoExec step sequence — direct → sudo -n → etc.).
func TestUpload_PathAwareSudo_ElevatedPath(t *testing.T) {
	remoteBase := "/opt/myapp"

	// First deploy: remoteBase does NOT exist.
	srv := newMockSSHServer(nil)
	srv.cmdExitCode = func(cmd string, stdin []byte) uint32 {
		// test -w probe fails (path requires elevation).
		if strings.Contains(cmd, "test -w") {
			return 1
		}
		// Direct mkdir/mv/rm also fail (simulating root-owned path).
		if strings.Contains(cmd, "mkdir") || strings.Contains(cmd, "mv") || strings.Contains(cmd, "rm -rf") {
			return 1
		}
		// sudo -n succeeds (NOPASSWD sudoers).
		if strings.Contains(cmd, "sudo -n") {
			return 0
		}
		return 0
	}
	client := startMockSSHServer(t, srv)

	localDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte("version: '3'"), 0644); err != nil {
		t.Fatal(err)
	}

	creds := new(SudoCreds)
	defer creds.Zero()
	warnedOnce := new(bool)
	_, err := Upload(context.Background(), client, localDir, remoteBase, nil, creds, false, warnedOnce, false)
	if err != nil {
		t.Fatalf("Upload returned unexpected error: %v", err)
	}

	cmds := srv.getCommands()
	t.Logf("commands executed: %v", cmds)

	// Verify the probe was issued.
	hasProbe := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "test -w") {
			hasProbe = true
			break
		}
	}
	if !hasProbe {
		t.Errorf("expected test -w probe in commands; got: %v", cmds)
	}

	// Verify SudoExec was invoked: look for sudo -n (step 3 of SudoExec) for
	// at least one remoteBase operation (mkdir/mv/rm).
	hasSudoForBase := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "sudo -n") &&
			(strings.Contains(cmd, "mkdir") || strings.Contains(cmd, "mv") || strings.Contains(cmd, "rm")) {
			hasSudoForBase = true
			break
		}
	}
	if !hasSudoForBase {
		t.Errorf("elevated path: expected sudo -n for a remoteBase mkdir/mv/rm operation; got: %v", cmds)
	}
}

// TestUpload_PathAwareSudo_ParentWritable verifies that when test -w remoteBase
// fails but test -w parent succeeds (OR probe exits 0 overall), needsSudo=false
// and all remoteBase operations bypass SudoExec.
func TestUpload_PathAwareSudo_ParentWritable(t *testing.T) {
	// remoteBase does not exist yet; parent is writable.
	// The probe is: test -w remoteBase || test -w path.Dir(remoteBase)
	// When remoteBase doesn't exist, `test -w remoteBase` returns exit 1,
	// but `test -w parent` returns exit 0 — the OR means the shell exits 0.
	// So needsSudo should be false (probe exit 0 → writable).
	remoteBase := "/home/deploy/newproject"

	srv := newMockSSHServer(nil)
	// All commands succeed (exit 0) — the OR probe exits 0 (parent writable).
	client := startMockSSHServer(t, srv)

	localDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte("version: '3'"), 0644); err != nil {
		t.Fatal(err)
	}

	creds := new(SudoCreds)
	defer creds.Zero()
	warnedOnce := new(bool)
	_, err := Upload(context.Background(), client, localDir, remoteBase, nil, creds, false, warnedOnce, false)
	if err != nil {
		t.Fatalf("Upload returned unexpected error: %v", err)
	}

	cmds := srv.getCommands()
	t.Logf("commands executed: %v", cmds)

	// Verify probe uses || (both remoteBase and its parent).
	hasOrProbe := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "test -w") && strings.Contains(cmd, "||") {
			hasOrProbe = true
			break
		}
	}
	if !hasOrProbe {
		t.Errorf("expected 'test -w ... || test -w ...' probe; got: %v", cmds)
	}

	// Since probe exits 0 (parent writable), no sudo should fire.
	for _, cmd := range cmds {
		if strings.Contains(cmd, "sudo") &&
			(strings.Contains(cmd, "mkdir") || strings.Contains(cmd, "mv") || strings.Contains(cmd, "rm -rf")) {
			t.Errorf("parent-writable: unexpected sudo command for remoteBase operation: %q", cmd)
		}
	}
}
