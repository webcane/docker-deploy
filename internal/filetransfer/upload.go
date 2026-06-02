package filetransfer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/sftp"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/term"

	"github.com/webcane/docker-deploy/internal/keychain"
)

// ErrDeployCancelled is returned by Upload() when the user declines the
// repeat-deploy confirmation prompt. Callers should treat this as a clean
// exit (print "Deploy cancelled." and return nil), not as an error.
var ErrDeployCancelled = errors.New("deploy cancelled by user")

// SudoCreds holds a sudo password as bytes so it can be zeroed after use.
// The zero value (nil pw) means no password has been captured yet.
//
// Host and User identify the remote target for macOS Keychain operations.
// When both are non-empty, SudoExec will check the Keychain before prompting
// interactively and offer to save a newly entered password after first use.
// Leaving them empty disables Keychain integration and preserves the existing
// behaviour (direct → cached → passwordless sudo → interactive prompt).
type SudoCreds struct {
	pw   []byte
	Host string // remote hostname (e.g. "myserver.example.com")
	User string // SSH/sudo username (e.g. "deploy")
}

// Zero zeroes the password bytes and nils the slice to prevent it lingering
// in process memory after Upload() returns (golang.org/x/crypto convention).
func (c *SudoCreds) Zero() {
	for i := range c.pw {
		c.pw[i] = 0
	}
	c.pw = nil
}

// promptSudoPasswordFunc is the function used to obtain a sudo password
// interactively. It is a package-level variable so tests can replace it
// without needing a real terminal.
var promptSudoPasswordFunc = promptSudoPassword

// promptSudoPassword prompts the user for a sudo password and returns it,
// or an error if the prompt fails or times out.
func promptSudoPassword() (string, error) {
	fmt.Fprintf(os.Stderr, "[sudo] password for remote host: ")
	pw, readErr := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stderr)
	if readErr != nil {
		return "", fmt.Errorf("reading sudo password: %w", readErr)
	}
	return string(pw), nil
}

// sshRun runs a command on the remote via a new SSH session.
//   - pw == nil → session.Run(cmd) with no privilege escalation
//   - pw != nil → sudo with password piped via stdin (-S flag, empty prompt)
//
// Per CLAUDE.md: each SSH exec must use a fresh NewSession() call.
func sshRun(client *gossh.Client, cmd string, pw []byte) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("creating SSH session: %w", err)
	}
	defer session.Close() //nolint:errcheck

	if pw == nil {
		if err := session.Run(cmd); err != nil {
			return fmt.Errorf("running %q: %w", cmd, err)
		}
		return nil
	}

	// Password path: pipe password via stdin — keeps credential out of SSH server
	// logs and /proc process listings (T-13-04-01).
	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("opening stdin pipe: %w", err)
	}

	sudoCmd := fmt.Sprintf("sudo -S -p '' sh -c %s", ShellQuote(cmd))
	if err := session.Start(sudoCmd); err != nil {
		return fmt.Errorf("starting sudo command: %w", err)
	}
	_, _ = stdin.Write(append(pw, '\n'))
	_ = stdin.Close()
	if err := session.Wait(); err != nil {
		return fmt.Errorf("sudo command failed: %w", err)
	}
	return nil
}

// tryKeychainAuth attempts to authenticate using a password stored in the macOS
// Keychain. Returns true and caches the password in creds if the stored
// password succeeds. Returns false (without error) when no entry exists, the
// security binary is absent, or the stored password is rejected — all of which
// should fall through to the interactive prompt.
func tryKeychainAuth(client *gossh.Client, cmd string, creds *SudoCreds, verbose bool) bool {
	stored, kErr := keychain.Lookup(creds.Host, creds.User)
	if kErr != nil || stored == "" {
		return false
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "[ssh] (sudo password cmd redacted — keychain)\n")
	}
	if sshRun(client, cmd, []byte(stored)) != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "  → exit 1 (keychain creds failed — falling back to prompt)\n")
		}
		return false
	}
	creds.pw = []byte(stored)
	if verbose {
		fmt.Fprintf(os.Stderr, "  → exit 0 (keychain creds)\n")
	}
	return true
}

// offerKeychainSave prompts the user to save pw to the macOS Keychain. Called
// after a successful interactive sudo prompt when creds.Host and creds.User are
// set. Prints a warning to stderr if the save fails but does not return an error
// — the deploy has already succeeded at this point.
func offerKeychainSave(creds *SudoCreds, pw string) {
	fmt.Fprintf(os.Stderr, "Save password to macOS Keychain for %s@%s? [y/N] ", creds.User, creds.Host)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)
	if strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes") {
		if saveErr := keychain.Store(creds.Host, creds.User, pw); saveErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save to Keychain: %v\n", saveErr)
		}
	}
}

// SudoExec runs cmd on the remote with automatic privilege escalation fallback.
//
// Step order (D-11):
//  1. Direct: sshRun(client, cmd, nil) — no sudo
//  2. Cached: if creds.pw != nil, sshRun with creds.pw
//  3. Passwordless: sudo -n sh -c <cmd> (NOPASSWD sudoers)
//     3b. Keychain: if creds.Host/User are set, check macOS Keychain for stored password
//  4. Interactive: prompt up to 3 times; on success cache password in creds.pw;
//     offer to save to macOS Keychain if creds.Host/User are set
//
// creds and warnedOnce are in/out params shared across all SudoExec calls
// within a single Upload() — ensures the interactive prompt fires at most once
// per deploy (SC-6, D-12).
func SudoExec(client *gossh.Client, cmd string, creds *SudoCreds, warnedOnce *bool, verbose bool) error { //nolint:gocognit // 5-step fallback chain (direct→cached→sudo-n→keychain→interactive) with credential caching — splitting steps into helper functions would require threading creds/warnedOnce through each
	// Step 1: Direct (no privilege escalation).
	if verbose {
		fmt.Fprintf(os.Stderr, "[ssh] %s\n", cmd)
	}
	if err := sshRun(client, cmd, nil); err == nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "  → exit 0\n")
		}
		return nil
	}
	if verbose {
		// Use a generic message — the actual failure reason may not be exit 1
		// (e.g. connection reset, session limit). WR-06: avoid hardcoding "exit 1".
		fmt.Fprintf(os.Stderr, "  → direct failed, trying sudo\n")
	}

	// Step 2: Cached password from a previous interactive prompt.
	if creds.pw != nil {
		if err := sshRun(client, cmd, creds.pw); err == nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "  → exit 0 (cached creds)\n")
			}
			return nil
		}
		if verbose {
			fmt.Fprintf(os.Stderr, "  → exit 1 (cached creds failed)\n")
		}
	}

	// Step 3: Passwordless sudo (sudo -n, NOPASSWD sudoers entry).
	sudoNCmd := fmt.Sprintf("sudo -n sh -c %s", ShellQuote(cmd))
	if verbose {
		fmt.Fprintf(os.Stderr, "[ssh] %s\n", sudoNCmd)
	}
	if err := sshRun(client, sudoNCmd, nil); err == nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "  → exit 0 (passwordless sudo)\n")
		}
		return nil
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "  → exit 1 (passwordless sudo failed)\n")
	}

	// Step 3b: macOS Keychain — try a stored password before prompting interactively.
	// Only runs when creds.Host and creds.User are both set (wired from main.go).
	// tryKeychainAuth returns false on any error so the fallback to step 4 is silent.
	if creds.Host != "" && creds.User != "" && tryKeychainAuth(client, cmd, creds, verbose) {
		return nil
	}

	// Step 4: Interactive password prompt — up to 3 attempts.
	// Print the warning once regardless of verbose; in non-verbose mode it is
	// surfaced via the rollup in main.go (D-01, D-02).
	if !*warnedOnce {
		*warnedOnce = true
		if verbose {
			fmt.Fprintf(os.Stderr, "WARNING: passwordless sudo not configured; you may be prompted for a password\n")
		}
	}
	for attempt := 1; attempt <= 3; attempt++ {
		pw, readErr := promptSudoPasswordFunc()
		if readErr != nil {
			// Prompt failed (no terminal or EOF) — fall through to final error.
			break
		}
		if verbose {
			fmt.Fprintf(os.Stderr, "[ssh] (sudo password cmd redacted)\n")
		}
		if sshRun(client, cmd, []byte(pw)) == nil {
			creds.pw = []byte(pw)
			if verbose {
				fmt.Fprintf(os.Stderr, "  → exit 0\n")
			}
			// Offer to save the newly entered password to the macOS Keychain so
			// future deploys skip this prompt entirely (step 3b).
			if creds.Host != "" && creds.User != "" {
				offerKeychainSave(creds, pw)
			}
			return nil
		}
		if attempt < 3 {
			fmt.Fprintln(os.Stderr, "Sorry, try again.")
		}
	}

	return fmt.Errorf("could not write to target directory — no valid auth path available (tried direct copy, passwordless sudo, interactive password)")
}

// Upload copies all non-excluded files from localDir to a staging directory
// on the remote, then atomically renames the staging dir to the final target.
//
// remoteBase is the target directory path on the remote (e.g. "/opt/myapp").
// excludes is the list of exclude patterns (from Config.Excludes).
// force skips the replace-confirmation prompt on repeat deploys.
// verbose controls per-file output and SSH command logging:
//   - When verbose=true: per-file lines "  -> relative/path" are written to
//     os.Stderr; each SSH command (mkdir, mv, rm) and its exit code are logged
//     to os.Stderr before/after execution; warnedOnce is never set to true so
//     every sudo warning prints. When !force and existsBefore, the pre-confirm
//     diff block prints local and remote file lists (truncated at 20) before the
//     "Replace all contents?" prompt.
//   - When verbose=false: per-file lines are suppressed; SSH command lines are
//     suppressed; warnedOnce behavior is unchanged (first warning prints, rest
//     are suppressed).
//
// Atomic swap strategy:
//  1. Create staging dir: /tmp/docker-deploy-<unixNanoTimestamp>
//     (/tmp is always writable by the SSH user via SFTP, even when target is root-owned)
//  2. Upload all files into staging dir maintaining relative path structure
//  3. Ensure remoteBase exists (mkdir -p, falling back to interactive sudo with up to 3
//     password attempts). If target cannot be created: warn, leave staged files, return error.
//  4. Via SSH session exec: if remoteBase exists, mv to .old-<timestamp>, mv staging to
//     remoteBase, rm -rf the .old dir; if absent, just mv staging to remoteBase
//
// If WalkFiles returns 0 files, Upload returns an error.
//
// Progress: prints "Uploading N files..." before starting to os.Stdout.
// Per-file lines go to os.Stderr only when verbose=true.
//
// Per CLAUDE.md: sessions are NOT reusable — each SSH exec uses a fresh NewSession().
// SFTP wraps the existing *gossh.Client — no second TCP connection.
//
// Returns the number of files actually transferred on success.
func Upload(ctx context.Context, client *gossh.Client, localDir, remoteBase string, excludes []string, creds *SudoCreds, force bool, warnedOnce *bool, verbose bool) (int, error) { //nolint:gocognit // orchestrates 8+ atomic steps (probe, stage, copy, swap, rollback) sharing sftp client — each step is tightly coupled via common error path and cleanup logic
	// Step 1: Enumerate files to upload.
	files, err := WalkFiles(localDir, excludes)
	if err != nil {
		return 0, fmt.Errorf("enumerating files in %s: %w", localDir, err)
	}
	if len(files) == 0 {
		return 0, fmt.Errorf("no files to upload: all files excluded from %s", localDir)
	}

	// Step 2: Announce upload count.
	fmt.Fprintf(os.Stdout, "Uploading %d files...\n", len(files))

	// Step 3: Open SFTP session wrapping the existing SSH client.
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return 0, fmt.Errorf("opening SFTP session: %w", err)
	}
	defer sftpClient.Close() //nolint:errcheck

	// Step 3b: Probe whether the target path requires elevation.
	// Check ONLY the parent directory: mkdir, mv, and rm all operate on entries
	// WITHIN the parent — they require the parent to be writable, not the target
	// dir itself. Checking remoteBase directly is wrong: a user-owned /opt/myapp
	// would pass "test -w /opt/myapp" but "mv /opt/myapp /opt/myapp-old" still
	// fails because /opt (the parent) is root-owned.
	// path.Dir is used (not filepath.Dir) because the remote is always Linux.
	// sshRun with nil password: probe is read-only and never needs elevation.
	// Exit 0 → parent writable → needsSudo=false; exit 1 → needsSudo=true.
	probeCmd := fmt.Sprintf("test -w %s", ShellQuote(path.Dir(remoteBase)))
	if verbose {
		fmt.Fprintf(os.Stderr, "[ssh] %s\n", probeCmd)
	}
	needsSudo := sshRun(client, probeCmd, nil) != nil
	if verbose {
		if needsSudo {
			fmt.Fprintf(os.Stderr, "  → exit 1 (path requires elevation)\n")
		} else {
			fmt.Fprintf(os.Stderr, "  → exit 0 (path is user-writable)\n")
		}
	}

	// execCmd dispatches to SudoExec (elevated path) or sshRun with nil
	// password (user-writable path) for all remoteBase operations.
	execCmd := func(cmd string) error {
		if needsSudo {
			return SudoExec(client, cmd, creds, warnedOnce, verbose)
		}
		if verbose {
			fmt.Fprintf(os.Stderr, "[ssh] %s\n", cmd)
		}
		if err := sshRun(client, cmd, nil); err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "  → exit 1\n")
			}
			return err
		}
		if verbose {
			fmt.Fprintf(os.Stderr, "  → exit 0\n")
		}
		return nil
	}

	// Step 4: Derive staging directory in /tmp.
	// /tmp is world-writable so SFTP (which runs as the SSH user with no sudo)
	// can always create it, even when the target dir (e.g. /opt/…) is root-owned.
	// The final mv is a sudo mv so partial state is never left at the target path
	// (CLAUDE.md Rule 3). Use nanosecond precision to avoid collisions on concurrent
	// deployments to the same remote in the same second.
	timestamp := fmt.Sprintf("%d", time.Now().UnixNano())
	stagingDir := "/tmp/docker-deploy-" + timestamp

	// Step 5: Create staging directory.
	if err := sftpClient.MkdirAll(stagingDir); err != nil {
		return 0, fmt.Errorf("creating staging directory %s: %w", stagingDir, err)
	}

	// Step 6: Upload each file into the staging directory.
	// On any upload error: clean up the partial staging dir (unusable) and return.
	if verbose {
		fmt.Fprintf(os.Stderr, "Uploading %d files...\n", len(files))
	}
	uploadErr := func() error {
		for _, relPath := range files {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("upload cancelled: %w", err)
			}

			localPath := filepath.Join(localDir, filepath.FromSlash(relPath))
			remotePath := path.Join(stagingDir, relPath)

			// Ensure parent directory exists on remote.
			remoteDir := path.Dir(remotePath)
			if err := sftpClient.MkdirAll(remoteDir); err != nil {
				return fmt.Errorf("creating remote directory %s: %w", remoteDir, err)
			}

			// Stat local file to capture permissions before opening it.
			localInfo, err := os.Stat(localPath)
			if err != nil {
				return fmt.Errorf("stat local file %s: %w", localPath, err)
			}

			// Open local file for reading.
			localFile, err := os.Open(localPath) //nolint:gosec // localPath is constructed from localDir (os.Getwd()) + relative path from WalkFiles, a controlled source
			if err != nil {
				return fmt.Errorf("opening local file %s: %w", localPath, err)
			}

			// Create remote file.
			remoteFile, err := sftpClient.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
			if err != nil {
				_ = localFile.Close()
				return fmt.Errorf("creating remote file %s: %w", remotePath, err)
			}

			// Copy contents.
			if _, err := io.Copy(remoteFile, localFile); err != nil {
				_ = remoteFile.Close()
				_ = localFile.Close()
				return fmt.Errorf("copying %s to remote: %w", relPath, err)
			}

			if err := remoteFile.Close(); err != nil {
				_ = localFile.Close()
				return fmt.Errorf("flushing remote file %s: %w", remotePath, err)
			}
			_ = localFile.Close()

			// Preserve source file permissions (e.g. executable bit for scripts).
			if err := sftpClient.Chmod(remotePath, localInfo.Mode().Perm()); err != nil {
				return fmt.Errorf("setting permissions on remote file %s: %w", remotePath, err)
			}

		}
		return nil
	}()

	if uploadErr != nil {
		// Upload failed mid-way — staging dir is partial/unusable, clean it up.
		// Direct rm of /tmp staging dir — no elevated privileges needed.
		_ = sshRun(client, fmt.Sprintf("rm -rf %s", ShellQuote(stagingDir)), nil)
		return 0, uploadErr
	}

	// Check whether target already exists BEFORE we create it, so we can choose
	// the right swap path in step 10.
	// Log the remoteExists check when verbose.
	if verbose {
		fmt.Fprintf(os.Stderr, "[ssh] test -d %s\n", ShellQuote(remoteBase))
	}
	existsBefore, err := remoteExists(client, remoteBase)
	if verbose && err == nil {
		if existsBefore {
			fmt.Fprintf(os.Stderr, "  → exists\n")
		} else {
			fmt.Fprintf(os.Stderr, "  → absent\n")
		}
	}
	if err != nil {
		return 0, fmt.Errorf("checking remote target existence: %w", err)
	}

	// Pre-confirm diff + confirm prompt.
	// Runs only when !force and the remote target already exists (repeat deploy).
	// On first deploy no prompt is shown (nothing to overwrite).
	if !force && existsBefore { //nolint:nestif // confirm prompt logic requires shared access to files, remote listing, and force flag — extracting as a helper would require passing all these
		// Verbose pre-confirm diff block (D-17, D-18, D-19).
		// Print local and remote file lists to stderr before the prompt so the
		// operator can verify what will change.
		if verbose {
			// Local file list (already computed in step 1).
			const maxDisplay = 20
			fmt.Fprintf(os.Stderr, "Local files (%d):\n", len(files))
			for i, f := range files {
				if i >= maxDisplay {
					fmt.Fprintf(os.Stderr, "  ... and %d more\n", len(files)-maxDisplay)
					break
				}
				fmt.Fprintf(os.Stderr, "  %s\n", f)
			}

			// Remote file list via SFTP ReadDir (sftpClient opened at step 3).
			remoteEntries, rdErr := sftpClient.ReadDir(remoteBase)
			if rdErr != nil {
				fmt.Fprintf(os.Stderr, "Remote files: (unable to list: %v)\n", rdErr)
			} else {
				fmt.Fprintf(os.Stderr, "Remote files (%d):\n", len(remoteEntries))
				for i, e := range remoteEntries {
					if i >= maxDisplay {
						fmt.Fprintf(os.Stderr, "  ... and %d more\n", len(remoteEntries)-maxDisplay)
						break
					}
					fmt.Fprintf(os.Stderr, "  %s\n", e.Name())
				}
			}
		}

		// Confirm prompt.
		fmt.Fprintf(os.Stderr, "Target %s exists on remote. Replace all contents? [y/N] ", remoteBase)
		reader := bufio.NewReader(os.Stdin)
		answer, readErr := reader.ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			return 0, fmt.Errorf("reading confirmation: %w", readErr)
		}
		if readErr == io.EOF && strings.TrimSpace(answer) == "" {
			fmt.Fprintln(os.Stderr, "No input received — deploy cancelled.")
			_ = sshRun(client, "rm -rf "+ShellQuote(stagingDir), nil)
			return 0, ErrDeployCancelled
		}
		answer = strings.TrimSpace(answer)
		if !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
			fmt.Fprintln(os.Stderr, "Deploy cancelled.")
			_ = sshRun(client, "rm -rf "+ShellQuote(stagingDir), nil)
			return 0, ErrDeployCancelled
		}
	}

	// Verbose diff: first-deploy case — show "Remote files: (none)" when verbose.
	if verbose && !existsBefore {
		fmt.Fprintf(os.Stderr, "Local files (%d):\n", len(files))
		const maxDisplay = 20
		for i, f := range files {
			if i >= maxDisplay {
				fmt.Fprintf(os.Stderr, "  ... and %d more\n", len(files)-maxDisplay)
				break
			}
			fmt.Fprintf(os.Stderr, "  %s\n", f)
		}
		fmt.Fprintf(os.Stderr, "Remote files: (none)\n")
	}

	// If .env is excluded from upload and the remote target already has a .env file,
	// back it up before the atomic swap. The swap replaces the entire directory, which
	// would silently delete the remote .env even though the operator excluded it
	// intentionally (e.g. via --skip-env). The backup is restored after the swap.
	// Use ShouldExclude rather than an exact-string scan so that glob patterns
	// (e.g. ".env*") also trigger the backup (WR-03). ShouldExclude matches the
	// same patterns that WalkFiles uses to exclude files, keeping the two paths
	// consistent.
	envBackupPath := ""
	if existsBefore && ShouldExclude(".env", excludes) { //nolint:nestif // .env backup logic requires checking remote file existence and conditionally copying — splitting into a helper would need the same parameters
		envPath := path.Join(remoteBase, ".env")
		out, checkErr := sshExecOutput(client, fmt.Sprintf("test -f %s && echo exists || echo absent", ShellQuote(envPath)))
		if checkErr == nil && strings.HasPrefix(strings.TrimSpace(out), "exists") {
			envBackupPath = "/tmp/docker-deploy-env-" + timestamp
			if cpErr := sshRun(client, fmt.Sprintf("cp %s %s", ShellQuote(envPath), ShellQuote(envBackupPath)), nil); cpErr != nil {
				fmt.Fprintf(os.Stderr, "WARNING: could not backup remote .env: %v; .env will not be preserved after deploy\n", cpErr)
				envBackupPath = ""
			}
		}
	}

	// Step 8: Ensure target directory exists.
	// D-15: execCmd — dispatches to SudoExec or sshRun based on needsSudo probe.
	if err := execCmd(fmt.Sprintf("mkdir -p %s", ShellQuote(remoteBase))); err != nil {
		remoteHost := client.RemoteAddr().String()
		fmt.Fprintf(os.Stderr,
			"Warning: could not create target directory %s.\n"+
				"Uploaded files are staged at %s on the remote server.\n"+
				"To deploy manually:\n"+
				"  ssh %s 'sudo mv %s %s'\n"+
				"Or re-run after granting access:\n"+
				"  ssh %s 'sudo mkdir -p %s && sudo chown <user> %s'\n",
			remoteBase,
			stagingDir,
			remoteHost, stagingDir, remoteBase,
			remoteHost, remoteBase, remoteBase,
		)
		return 0, fmt.Errorf("could not create target directory %s", remoteBase)
	}

	// Step 9: Atomic swap via SSH exec.
	// D-15: ALL mv/rm operations on remoteBase use SudoExec — remoteBase may be root-owned.
	if existsBefore { //nolint:nestif // atomic swap with rollback is inherently branchy — first-deploy and repeat-deploy share the same staging dir and cleanup
		// Repeat deploy — three-step atomic swap:
		//   1. mv remoteBase to backup
		//   2. mv staging to remoteBase
		//   3. rm -rf backup (non-fatal on failure)
		oldDir := remoteBase + "-old-" + timestamp

		if err := execCmd(fmt.Sprintf("mv %s %s", ShellQuote(remoteBase), ShellQuote(oldDir))); err != nil {
			if envBackupPath != "" {
				_ = sshRun(client, fmt.Sprintf("rm -f %s", ShellQuote(envBackupPath)), nil)
			}
			return 0, fmt.Errorf("renaming existing target to backup: %w", err)
		}
		if err := execCmd(fmt.Sprintf("mv %s %s", ShellQuote(stagingDir), ShellQuote(remoteBase))); err != nil {
			// Rollback: best-effort restore using execCmd so the mv succeeds even
			// when /opt/… requires elevated privileges (D-15, feedback_sudo_rollback.md).
			rollbackErr := execCmd(fmt.Sprintf("mv %s %s", ShellQuote(oldDir), ShellQuote(remoteBase)))
			// Clean up .env backup on all error paths — the original remoteBase is
			// being restored, so its .env is already present.
			if envBackupPath != "" {
				_ = sshRun(client, fmt.Sprintf("rm -f %s", ShellQuote(envBackupPath)), nil)
			}
			if rollbackErr != nil {
				return 0, fmt.Errorf(
					"placing new version failed and rollback also failed (%v).\n"+
						"Restore manually:\n"+
						"  ssh %s 'sudo mv %s %s'\n"+
						"Original error: %w",
					rollbackErr, client.RemoteAddr().String(), ShellQuote(oldDir), ShellQuote(remoteBase), err)
			}
			return 0, fmt.Errorf(
				"placing new version failed (rolled back successfully).\n"+
					"Original error: %w",
				err)
		}
		if err := execCmd(fmt.Sprintf("rm -rf %s", ShellQuote(oldDir))); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not remove backup directory %s: %v\n", oldDir, err)
		}
	} else {
		// First deploy — move staging directly to target.
		// Step 8 (mkdir -p) created remoteBase as an empty directory. If we now
		// run `mv stagingDir remoteBase`, Unix mv moves stagingDir *inside*
		// remoteBase (because the destination exists) instead of renaming it.
		// Remove the empty placeholder first so that mv performs a clean rename.
		if err := execCmd(fmt.Sprintf("rm -rf %s", ShellQuote(remoteBase))); err != nil {
			return 0, fmt.Errorf("removing target placeholder before first deploy: %w", err)
		}
		if err := execCmd(fmt.Sprintf("mv %s %s", ShellQuote(stagingDir), ShellQuote(remoteBase))); err != nil {
			return 0, fmt.Errorf("moving staging dir to target: %w", err)
		}
	}

	// Restore backed-up .env into the new remoteBase now that the swap is complete.
	// execCmd is used for the copy because remoteBase may require elevated
	// permissions (the same path that handled the mv/rm steps above) — D-15.
	if envBackupPath != "" {
		envDest := path.Join(remoteBase, ".env")
		if restoreErr := execCmd(fmt.Sprintf("cp %s %s", ShellQuote(envBackupPath), ShellQuote(envDest))); restoreErr != nil {
			fmt.Fprintf(os.Stderr, "WARNING: deploy succeeded but could not restore remote .env: %v\n", restoreErr)
		}
		_ = sshRun(client, fmt.Sprintf("rm -f %s", ShellQuote(envBackupPath)), nil)
	}

	return len(files), nil
}

// remoteExists checks whether a path exists and is a directory on the remote
// host by running `test -d <path> && echo exists || echo absent` via SSH.
func remoteExists(client *gossh.Client, remotePath string) (bool, error) {
	output, err := sshExecOutput(client, fmt.Sprintf("test -d %s && echo exists || echo absent", ShellQuote(remotePath)))
	if err != nil {
		return false, err
	}
	switch {
	case len(output) >= 6 && output[:6] == "exists":
		return true, nil
	default:
		return false, nil
	}
}

// sshExecOutput runs a command and returns its combined stdout as a string.
func sshExecOutput(client *gossh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("creating SSH session: %w", err)
	}
	defer session.Close() //nolint:errcheck

	out, err := session.Output(cmd)
	if err != nil {
		return "", fmt.Errorf("running %q: %w", cmd, err)
	}
	return string(out), nil
}

// ShellQuote wraps s in single quotes for safe use in shell commands,
// escaping any embedded single quotes using the '\" technique.
// This handles paths derived from validated config values (remoteBase is from
// Resolve() which validates via ParseHost; staging dir name uses only
// alphanumerics + timestamp integer — T-03-05).
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
