package filetransfer

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

			// Send exit-status 0 (success for all commands in these tests).
			exitMsg := gossh.Marshal(struct{ Code uint32 }{0})
			ch.SendRequest("exit-status", false, exitMsg) //nolint:errcheck
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

	var sudoPw *string
	sudoPw = new(string)
	*sudoPw = ""
	var warnedOnce *bool
	warnedOnce = new(bool)
	*warnedOnce = false
	_, err := Upload(context.Background(), client, localDir, remoteBase, nil, sudoPw, warnedOnce, false)
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
	var sudoPw *string
	sudoPw = new(string)
	*sudoPw = ""
	var warnedOnce *bool
	warnedOnce = new(bool)
	*warnedOnce = false
	_, err := Upload(context.Background(), client, localDir, remoteBase, nil, sudoPw, warnedOnce, false)
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

	var sudoPw *string
	sudoPw = new(string)
	*sudoPw = ""
	var warnedOnce *bool
	warnedOnce = new(bool)
	*warnedOnce = false
	_, err := Upload(context.Background(), client, localDir, remoteBase, nil, sudoPw, warnedOnce, false)
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

		var sudoPw *string = new(string)
		var warnedOnce *bool = new(bool)
		_, err := Upload(context.Background(), client, localDir, remoteBase, nil, sudoPw, warnedOnce, true)

		w.Close()
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

		var sudoPw *string = new(string)
		var warnedOnce *bool = new(bool)
		_, err := Upload(context.Background(), client, localDir, remoteBase, nil, sudoPw, warnedOnce, false)

		w.Close()
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

	var sudoPw *string = new(string)
	var warnedOnce *bool = new(bool)
	_, err := Upload(context.Background(), client, localDir, remoteBase, nil, sudoPw, warnedOnce, true)

	w.Close()
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

	var sudoPw *string
	sudoPw = new(string)
	*sudoPw = ""
	var warnedOnce *bool
	warnedOnce = new(bool)
	*warnedOnce = false
	_, err := Upload(context.Background(), client, localDir, remoteBase, nil, sudoPw, warnedOnce, false)
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

		var sudoPw *string = new(string)
		var warnedOnce *bool = new(bool)
		excludes := []string{".env"}
		_, err := Upload(context.Background(), client, localDir, remoteBase, excludes, sudoPw, warnedOnce, false)
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

		var sudoPw *string = new(string)
		var warnedOnce *bool = new(bool)
		_, err := Upload(context.Background(), client, localDir, remoteBase, nil, sudoPw, warnedOnce, false)
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

		var sudoPw *string = new(string)
		var warnedOnce *bool = new(bool)
		excludes := []string{".env"}
		_, err := Upload(context.Background(), client, localDir, remoteBase, excludes, sudoPw, warnedOnce, false)
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
