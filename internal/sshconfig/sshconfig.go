// Package sshconfig parses ~/.ssh/config to extract IdentityFile entries
// for a given host, then loads the corresponding private key signers.
package sshconfig

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gossh "golang.org/x/crypto/ssh"
)

// LoadSigners reads configPath (typically ~/.ssh/config), finds the Host
// block(s) that match hostname, and returns a []gossh.Signer for each
// successfully-loaded IdentityFile.
//
// Keys that cannot be loaded (wrong passphrase, missing file, etc.) are
// silently skipped — the caller should treat an empty result as "no keys
// available from config".
func LoadSigners(configPath, hostname string) []gossh.Signer {
	identityFiles := parseIdentityFiles(configPath, hostname)
	if len(identityFiles) == 0 {
		// Fall back to well-known default key locations.
		identityFiles = defaultIdentityFiles()
	}

	var signers []gossh.Signer
	for _, path := range identityFiles {
		s, err := loadSigner(path)
		if err != nil {
			continue // silently skip unloadable keys
		}
		signers = append(signers, s)
	}
	return signers
}

// parseIdentityFiles returns the IdentityFile paths declared in configPath
// for the matching Host blocks. It handles both exact hostname matches and
// wildcard patterns (e.g. "Host *").
func parseIdentityFiles(configPath, hostname string) []string { //nolint:gocognit // line-by-line ssh config parser with Host block tracking and wildcard matching — complexity is inherent to the format
	f, err := os.Open(configPath) //nolint:gosec // configPath is ~/.ssh/config, a user-controlled trusted path
	if err != nil {
		return nil
	}
	defer f.Close() //nolint:errcheck

	var (
		result  []string
		active  bool
		scanner = bufio.NewScanner(f)
	)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and blank lines.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		keyword := strings.ToLower(parts[0])
		value := parts[1]

		switch keyword {
		case "host":
			// SSH config allows multiple patterns on one Host line (e.g. "Host a b *.c").
			active = false
			for _, pattern := range parts[1:] {
				if hostMatches(pattern, hostname) {
					active = true
					break
				}
			}

		case "identityfile":
			if active {
				result = append(result, expandPath(value))
			}
		}
	}
	return result
}

// hostMatches reports whether the Host pattern in an SSH config matches
// the given hostname. Supports "*" wildcard.
func hostMatches(pattern, hostname string) bool {
	if pattern == "*" {
		return true
	}
	matched, err := filepath.Match(pattern, hostname)
	return err == nil && matched
}

// expandPath expands ~ to the user home directory in SSH config paths.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// defaultIdentityFiles returns the default SSH private key paths that OpenSSH
// tries when no explicit IdentityFile is configured.
func defaultIdentityFiles() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "id_rsa"),
		filepath.Join(home, ".ssh", "id_ecdsa"),
	}
}

// loadSigner reads a PEM-encoded private key from path and returns an ssh.Signer.
// Returns an error if the file cannot be read or the key cannot be parsed.
func loadSigner(path string) (gossh.Signer, error) {
	pemBytes, err := os.ReadFile(path) //nolint:gosec // path is from user-controlled SSH config, acceptable
	if err != nil {
		return nil, fmt.Errorf("reading private key %q: %w", path, err)
	}
	signer, err := gossh.ParsePrivateKey(pemBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing private key %q: %w", path, err)
	}
	return signer, nil
}
