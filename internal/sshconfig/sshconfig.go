// Package sshconfig parses ~/.ssh/config to extract IdentityFile entries
// for a given host, then loads the corresponding private key signers.
package sshconfig

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	gossh "golang.org/x/crypto/ssh"
)

// HostEntry holds the resolved fields from a matching Host block in ~/.ssh/config.
// Zero values indicate the directive was absent; callers apply their own defaults.
//   - HostName: resolved hostname (alias label if HostName directive absent — D-07)
//   - User: empty string if absent (caller uses OS user or deploy.yaml user — D-09)
//   - Port: 0 if absent (caller defaults to 22 — D-08)
//   - IdentityFiles: expanded paths collected from IdentityFile directives
type HostEntry struct {
	HostName      string
	User          string
	Port          int
	IdentityFiles []string
}

// LookupHost reads configPath (typically ~/.ssh/config), finds the first Host
// block whose pattern(s) match alias, and returns the resolved HostEntry plus
// found=true. If no matching block is found, or the file cannot be opened,
// it returns HostEntry{}, false.
//
// Per D-07: when HostName directive is absent, HostEntry.HostName is set to
// the alias label itself (matches OpenSSH behaviour).
// Per D-08: Port 0 means "not set"; caller defaults to 22.
// Per D-09: User "" means "not set"; caller inherits OS username.
// Per D-11: Include directives are silently skipped.
// TODO: Include directives not implemented — only the named config file is parsed.
func LookupHost(configPath, alias string) (HostEntry, bool) { //nolint:gocognit // line-by-line ssh config parser with Host block tracking — complexity is inherent to the format
	f, err := os.Open(configPath) //nolint:gosec // configPath is ~/.ssh/config, a user-controlled trusted path
	if err != nil {
		return HostEntry{}, false
	}
	defer f.Close() //nolint:errcheck

	var (
		entry            HostEntry
		active           bool
		found            bool
		rawIdentityFiles []string
		scanner          = bufio.NewScanner(f)
	)

scan:
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip blank lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 1 {
			continue
		}
		keyword := strings.ToLower(parts[0])

		// Per D-11: skip Include directives silently.
		// TODO: Include directives not implemented — only the named config file is parsed.
		if keyword == "include" {
			continue
		}

		if len(parts) < 2 {
			continue
		}
		value := parts[1]

		switch keyword {
		case "host":
			// A new Host block starts. If the previous block was active and not yet
			// recorded, mark it found now (the block has ended).
			if active && !found {
				found = true
			}
			if found {
				break scan // exit the for loop — bare break would only exit the switch
			}
			// SSH config allows multiple patterns: "Host a b *.c"
			active = false
			for _, pattern := range parts[1:] {
				if hostMatches(pattern, alias) {
					active = true
					break
				}
			}

		case "hostname":
			if active && !found {
				entry.HostName = value
			}
		case "user":
			if active && !found {
				entry.User = value
			}
		case "port":
			if active && !found {
				if p, err := strconv.Atoi(value); err == nil {
					entry.Port = p
				}
			}
		case "identityfile":
			if active && !found {
				rawIdentityFiles = append(rawIdentityFiles, value)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return HostEntry{}, false
	}

	// Handle the last block in the file (no subsequent "host" keyword to trigger found).
	if active && !found {
		found = true
	}

	if !found {
		return HostEntry{}, false
	}

	// Per D-07: if no HostName directive was found, use the alias label itself.
	if entry.HostName == "" {
		entry.HostName = alias
	}

	// Expand IdentityFile paths now that HostName and User are fully resolved.
	home, _ := os.UserHomeDir()
	localUser := ""
	if u, err := user.Current(); err == nil {
		localUser = u.Username
	}
	portStr := "22"
	if entry.Port != 0 {
		portStr = strconv.Itoa(entry.Port)
	}
	for _, raw := range rawIdentityFiles {
		entry.IdentityFiles = append(entry.IdentityFiles, expandPath(raw, home, localUser, entry.HostName, entry.User, portStr))
	}

	return entry, true
}

// ListHosts reads configPath (typically ~/.ssh/config) and returns all
// non-wildcard Host block aliases in file order. Wildcard patterns (containing
// '*' or '?') are excluded. Returns nil if the file cannot be opened, is empty,
// or if the scanner encounters an error.
func ListHosts(configPath string) []string {
	f, err := os.Open(configPath) //nolint:gosec // configPath is ~/.ssh/config, a user-controlled trusted path
	if err != nil {
		return nil
	}
	defer f.Close() //nolint:errcheck

	var (
		aliases []string
		scanner = bufio.NewScanner(f)
	)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip blank lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		keyword := strings.ToLower(parts[0])

		if keyword == "host" {
			for _, pattern := range parts[1:] {
				if !strings.ContainsAny(pattern, "*?") {
					aliases = append(aliases, pattern)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil
	}

	return aliases
}

// LoadSigners reads configPath (typically ~/.ssh/config), finds the Host
// block(s) that match hostname, and returns a []gossh.Signer for each
// successfully-loaded IdentityFile.
//
// Keys that cannot be loaded (wrong passphrase, missing file, etc.) are
// silently skipped — the caller should treat an empty result as "no keys
// available from config".
func LoadSigners(configPath, hostname string) []gossh.Signer {
	// Delegate to LookupHost (D-10): LoadSigners is a thin wrapper.
	entry, found := LookupHost(configPath, hostname)
	var identityFiles []string
	if found && len(entry.IdentityFiles) > 0 {
		identityFiles = entry.IdentityFiles
	} else {
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

// hostMatches reports whether the Host pattern in an SSH config file matches
// the given hostname. Supports "*" wildcard.
func hostMatches(pattern, hostname string) bool {
	if pattern == "*" {
		return true
	}
	matched, err := filepath.Match(pattern, hostname)
	return err == nil && matched
}

// expandPath expands ~ and OpenSSH %-tokens in an SSH config path.
// Tokens: %d=homeDir, %u=localUser, %h=hostname, %r=remoteUser, %p=port, %%=literal %.
func expandPath(path, homeDir, localUser, hostname, remoteUser, port string) string {
	if strings.HasPrefix(path, "~/") {
		path = filepath.Join(homeDir, path[2:])
	}
	r := strings.NewReplacer(
		"%%", "%",
		"%d", homeDir,
		"%u", localUser,
		"%h", hostname,
		"%r", remoteUser,
		"%p", port,
	)
	return r.Replace(path)
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
