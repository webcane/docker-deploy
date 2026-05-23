package ssh

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	sshconfig "github.com/webcane/docker-deploy/internal/sshconfig"
)

// DialConfig holds the parameters for opening an SSH connection.
type DialConfig struct {
	// User is the SSH username.
	User string

	// Hostname is the remote host (without port).
	Hostname string

	// Port is the SSH port; defaults to 22 if zero.
	Port int

	// KnownHostsPath is the path to the known_hosts file.
	// If empty, ~/.ssh/known_hosts is used.
	KnownHostsPath string

	// Timeout is the maximum duration to wait for the SSH handshake to complete.
	// If zero, 10 seconds is used. Note: ClientConfig.Timeout only covers TCP;
	// the goroutine+select pattern here enforces the full SSH handshake timeout
	// per CLAUDE.md Rule 2.
	Timeout time.Duration

	// Stdin is used to read user responses to TOFU prompts.
	// Pass os.Stdin from callers; tests pass a *strings.Reader.
	Stdin io.Reader

	// Stdout is used to print TOFU prompts and key-mismatch warnings.
	// Pass os.Stderr from callers; tests may capture this.
	Stdout io.Writer
}

// Dial opens an authenticated SSH connection to the host described by cfg.
//
// Security properties (per CLAUDE.md):
//   - Host key always verified against known_hosts (no trust-on-first-use bypass).
//   - Enforces full SSH handshake timeout via goroutine + select (not just ClientConfig.Timeout).
//   - Auth chain: SSH agent first, then key files from ~/.ssh/config; no password fallback.
//   - Unknown host: TOFU prompt — show fingerprint, ask user, append on "yes".
//   - Changed fingerprint: hard fail with loud warning + ssh-keygen -R command.
func Dial(ctx context.Context, cfg DialConfig) (*gossh.Client, error) {
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	if cfg.Stdin == nil {
		cfg.Stdin = os.Stdin
	}
	if cfg.Stdout == nil {
		cfg.Stdout = os.Stderr
	}

	// Step 1: Build auth methods (agent → config keys; no password).
	// We do not short-circuit here on an empty list: attempting the TCP
	// connection first is required so that (a) the timeout path fires for
	// unreachable hosts and (b) TOFU host-key acceptance runs before auth.
	// The "no methods" case is handled by formatDialError after gossh returns.
	authMethods, err := buildAuthMethods(cfg.Hostname, cfg.User)
	if err != nil {
		return nil, err
	}

	// Step 2: Build the known-hosts callback with TOFU and hard-fail support.
	knownHostsPath := cfg.KnownHostsPath
	if knownHostsPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolving home dir for known_hosts: %w", err)
		}
		knownHostsPath = filepath.Join(home, ".ssh", "known_hosts")
	}

	baseCallback, err := buildHostKeyCallback(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("loading known_hosts: %w", err)
	}

	// Wrap the base callback to intercept typed errors and handle TOFU / hard-fail.
	wrappedCallback := func(hostname string, remote net.Addr, key gossh.PublicKey) error {
		cbErr := baseCallback(hostname, remote, key)
		if cbErr == nil {
			return nil
		}

		switch typed := cbErr.(type) {
		case *UnknownHostError:
			return handleTOFU(cfg.Stdin, cfg.Stdout, knownHostsPath, hostname, remote, key, typed)

		case *KeyMismatchError:
			return handleKeyMismatch(cfg.Stdout, cfg.Hostname, typed)

		default:
			return cbErr
		}
	}

	// Step 3: Dial with goroutine + select to enforce full SSH handshake timeout.
	addr := fmt.Sprintf("%s:%d", cfg.Hostname, cfg.Port)

	clientCfg := &gossh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: wrappedCallback,
		// Set 100ms longer than our select timeout so time.After always fires
		// first, guaranteeing the deterministic "SSH connection timed out after X"
		// message rather than an OS-level "i/o timeout" from the net stack.
		Timeout: timeout + 100*time.Millisecond,
	}

	type result struct {
		client *gossh.Client
		err    error
	}
	ch := make(chan result, 1)
	go func() {
		c, dialErr := gossh.Dial("tcp", addr, clientCfg)
		ch <- result{c, dialErr}
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("SSH connection cancelled: %w", ctx.Err())
	case <-time.After(timeout):
		return nil, fmt.Errorf("SSH connection timed out after %v", timeout)
	case r := <-ch:
		if r.err != nil {
			return nil, formatDialError(r.err, cfg.Hostname)
		}
		return r.client, nil
	}
}

// buildAuthMethods constructs the SSH auth chain:
// 1. SSH agent (if $SSH_AUTH_SOCK is set and accessible)
// 2. Key files from ~/.ssh/config matching the target host
// No password auth is added.
func buildAuthMethods(hostname, user string) ([]gossh.AuthMethod, error) {
	var methods []gossh.AuthMethod

	// Try SSH agent first.
	if agentSock := os.Getenv("SSH_AUTH_SOCK"); agentSock != "" {
		conn, err := net.Dial("unix", agentSock)
		if err == nil {
			agentClient := agent.NewClient(conn)
			methods = append(methods, gossh.PublicKeysCallback(agentClient.Signers))
		}
		// Silently skip if agent unavailable.
	}

	// Then load key files from ~/.ssh/config.
	signers := loadSSHConfigKeys(hostname, user)
	if len(signers) > 0 {
		methods = append(methods, gossh.PublicKeys(signers...))
	}

	return methods, nil
}

// loadSSHConfigKeys reads ~/.ssh/config, finds the block matching hostname,
// and loads each IdentityFile key. Keys that fail to load (e.g. wrong passphrase)
// are silently skipped.
func loadSSHConfigKeys(hostname, _ string) []gossh.Signer {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	configPath := filepath.Join(home, ".ssh", "config")
	return sshconfig.LoadSigners(configPath, hostname)
}

// handleTOFU prints the host fingerprint, prompts the user, and appends the
// host entry to known_hosts on confirmation.
func handleTOFU(
	stdin io.Reader,
	stdout io.Writer,
	knownHostsPath string,
	hostname string,
	remote net.Addr,
	key gossh.PublicKey,
	_ *UnknownHostError,
) error {
	fp := formatFingerprint(key)
	fmt.Fprintf(stdout, "The authenticity of host '%s' cannot be established.\n", hostname)
	fmt.Fprintf(stdout, "Key fingerprint is %s.\n", fp)
	fmt.Fprintf(stdout, "Are you sure you want to continue connecting? [yes/no]: ")

	reader := bufio.NewReader(stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)

	if answer != "yes" {
		return fmt.Errorf("host key verification rejected by user")
	}

	if err := appendKnownHost(knownHostsPath, hostname, remote, key); err != nil {
		return fmt.Errorf("appending host key to known_hosts: %w", err)
	}
	fmt.Fprintf(stdout, "Warning: Permanently added '%s' to the list of known hosts.\n", hostname)
	return nil
}

// handleKeyMismatch prints a loud warning and the remediation command, then
// returns the error. The caller is NOT asked to confirm — this is a hard fail.
func handleKeyMismatch(stdout io.Writer, hostname string, e *KeyMismatchError) error {
	fmt.Fprintln(stdout, "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@")
	fmt.Fprintln(stdout, "@    WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!     @")
	fmt.Fprintln(stdout, "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@")
	fmt.Fprintf(stdout, "IT IS POSSIBLE THAT SOMEONE IS DOING SOMETHING NASTY!\n")
	fmt.Fprintf(stdout, "Someone could be eavesdropping on you right now (man-in-the-middle attack)!\n")
	fmt.Fprintf(stdout, "Host: %s\n", hostname)
	fmt.Fprintf(stdout, "Previously recorded fingerprint: %s\n", e.OldFingerprint)
	fmt.Fprintf(stdout, "New fingerprint:                 %s\n", e.NewFingerprint)
	fmt.Fprintf(stdout, "\nTo remove the old key, run:\n")
	fmt.Fprintf(stdout, "  ssh-keygen -R %s\n\n", hostname)
	return e
}

// formatDialError wraps raw SSH dial errors with human-readable messages where
// appropriate (D-03: auth failure message).
func formatDialError(err error, hostname string) error {
	msg := err.Error()
	if strings.Contains(msg, "unable to authenticate") ||
		strings.Contains(msg, "no supported methods") ||
		strings.Contains(msg, "handshake failed") && strings.Contains(msg, "unable to authenticate") {
		return fmt.Errorf(
			"SSH auth failed: ensure your key is loaded in ssh-agent or configured in ~/.ssh/config for host %s",
			hostname,
		)
	}
	return err
}
