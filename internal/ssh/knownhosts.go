// Package ssh implements the SSH transport layer for docker-deploy.
// It provides Dial() with goroutine-wrapped timeout, TOFU known-hosts
// verification, changed-fingerprint hard-fail, and an auth chain
// (SSH agent → ~/.ssh/config key files). No password fallback.
package ssh

import (
	"errors"
	"fmt"
	"net"
	"os"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// UnknownHostError is returned when the remote host is not in known_hosts.
// The caller (Dial) handles the TOFU prompt.
type UnknownHostError struct {
	Hostname    string
	Fingerprint string
}

func (e *UnknownHostError) Error() string {
	return fmt.Sprintf("unknown host %s (fingerprint: %s)", e.Hostname, e.Fingerprint)
}

// KeyMismatchError is returned when the remote host's key has changed relative
// to the stored known_hosts entry. This is a hard fail — never prompt to accept.
type KeyMismatchError struct {
	Hostname       string
	OldFingerprint string
	NewFingerprint string
}

func (e *KeyMismatchError) Error() string {
	return fmt.Sprintf("WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED for %s", e.Hostname)
}

// buildHostKeyCallback returns an ssh.HostKeyCallback that verifies the remote
// host key against knownHostsPath.
//
// If the file does not exist it is created empty (O_CREATE|O_APPEND) so that
// appendKnownHost can write to it later.
//
// On verification:
//   - Known key that matches: returns nil (success).
//   - Known key with wrong fingerprint (changed): returns *KeyMismatchError.
//   - Unknown host (not in file): returns *UnknownHostError.
//   - Any other error: returned as-is.
func buildHostKeyCallback(knownHostsPath string) (gossh.HostKeyCallback, error) {
	// Ensure the file exists so knownhosts.New does not fail on a fresh install.
	f, err := os.OpenFile(knownHostsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600) //nolint:gosec // knownHostsPath is derived from os.UserHomeDir(), a trusted location
	if err != nil {
		return nil, fmt.Errorf("opening known_hosts %s: %w", knownHostsPath, err)
	}
	_ = f.Close()

	base, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("loading known_hosts %s: %w", knownHostsPath, err)
	}

	return func(hostname string, remote net.Addr, key gossh.PublicKey) error {
		err := base(hostname, remote, key)
		if err == nil {
			return nil
		}

		var ke *knownhosts.KeyError
		if !asKeyError(err, &ke) {
			// Not a KeyError — pass through as-is.
			return err
		}

		if len(ke.Want) > 0 {
			// The host is in known_hosts but the key has changed.
			oldFP := ""
			if len(ke.Want) > 0 {
				oldFP = gossh.FingerprintSHA256(ke.Want[0].Key)
			}
			return &KeyMismatchError{
				Hostname:       hostname,
				OldFingerprint: oldFP,
				NewFingerprint: formatFingerprint(key),
			}
		}

		// len(ke.Want) == 0: host not found in known_hosts.
		return &UnknownHostError{
			Hostname:    hostname,
			Fingerprint: formatFingerprint(key),
		}
	}, nil
}

// asKeyError attempts to unwrap err as a *knownhosts.KeyError.
// Returns true and sets target if successful.
func asKeyError(err error, target **knownhosts.KeyError) bool {
	return errors.As(err, target)
}

// appendKnownHost writes a valid known_hosts line for the given host and key
// to knownHostsPath, appending to any existing content.
func appendKnownHost(knownHostsPath string, hostname string, _ net.Addr, key gossh.PublicKey) error {
	f, err := os.OpenFile(knownHostsPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600) //nolint:gosec // knownHostsPath is derived from os.UserHomeDir(), a trusted location
	if err != nil {
		return fmt.Errorf("opening known_hosts for append: %w", err)
	}
	defer f.Close() //nolint:errcheck

	line := knownhosts.Line([]string{hostname}, key) + "\n"
	if _, err := fmt.Fprint(f, line); err != nil {
		return fmt.Errorf("writing known_hosts entry: %w", err)
	}
	return nil
}

// formatFingerprint returns the SHA-256 fingerprint of the given public key
// in the format produced by ssh.FingerprintSHA256 (e.g. "SHA256:...").
func formatFingerprint(key gossh.PublicKey) string {
	return gossh.FingerprintSHA256(key)
}
