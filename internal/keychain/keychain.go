// Package keychain provides macOS Keychain integration for storing and
// retrieving remote sudo passwords via /usr/bin/security.
//
// All functions degrade gracefully: if the security binary is unavailable or
// returns an unexpected error, Lookup returns ("", nil) so callers fall back
// to interactive prompting without surfacing a hard error.
package keychain

import (
	"fmt"
	"os/exec"
	"strings"
)

const service = "docker-deploy"

// execSecurityFunc is the function used to run /usr/bin/security subcommands.
// It is a package-level variable so tests can replace it without a real Keychain.
var execSecurityFunc = execSecurity

func execSecurity(args ...string) (string, error) {
	cmd := exec.Command("/usr/bin/security", args...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// account returns the Keychain account identifier: "user@host".
func account(user, host string) string {
	return user + "@" + host
}

// Lookup retrieves a stored sudo password for the given host and user.
// Returns ("", nil) when no entry exists or when the security binary is
// unavailable — callers should fall through to interactive prompting.
func Lookup(host, user string) (string, error) {
	pw, err := execSecurityFunc(
		"find-generic-password",
		"-s", service,
		"-a", account(user, host),
		"-w",
	)
	if err != nil {
		// exit 44 = item not found; any other failure is also non-fatal here —
		// we want interactive fallback, not a hard error.
		return "", nil
	}
	return pw, nil
}

// Store saves a sudo password to the Keychain. Any existing entry for the same
// host+user pair is overwritten (-U flag). Returns an error only if the write
// itself fails (e.g. user denied Keychain access).
func Store(host, user, password string) error {
	_, err := execSecurityFunc(
		"add-generic-password",
		"-U",
		"-s", service,
		"-a", account(user, host),
		"-w", password,
	)
	if err != nil {
		return fmt.Errorf("keychain store: %w", err)
	}
	return nil
}

// Delete removes the stored entry for the given host and user.
// Returns an error if the deletion fails for reasons other than "not found".
func Delete(host, user string) error {
	_, err := execSecurityFunc(
		"delete-generic-password",
		"-s", service,
		"-a", account(user, host),
	)
	if err != nil {
		return fmt.Errorf("keychain delete: %w", err)
	}
	return nil
}
