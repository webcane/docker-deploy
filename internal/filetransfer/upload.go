package filetransfer

import (
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
	"golang.org/x/term"
	gossh "golang.org/x/crypto/ssh"
)

// tryDirectCopy attempts to run a command without privilege escalation.
// Returns true if successful, false otherwise.
func tryDirectCopy(client *gossh.Client, cmd string) bool {
	return sshExec(client, cmd) == nil
}

// tryPasswordlessSudo attempts to run a command with passwordless sudo (sudo -n).
// Returns true if successful, false otherwise.
func tryPasswordlessSudo(client *gossh.Client, cmd string) bool {
	sudoCmd := fmt.Sprintf("sudo -n sh -c %s", ShellQuote(cmd))
	return sshExec(client, sudoCmd) == nil
}

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

// sshExecVerbose wraps sshExec and logs the command and its exit code to stderr
// when verbose=true. The command string is logged before execution as "[ssh] <cmd>",
// and the exit code is logged after as "  → exit 0" or "  → exit N".
func sshExecVerbose(client *gossh.Client, cmd string, verbose bool) error {
	if verbose {
		fmt.Fprintf(os.Stderr, "[ssh] %s\n", cmd)
	}
	err := sshExec(client, cmd)
	if verbose {
		if err == nil {
			fmt.Fprintf(os.Stderr, "  → exit 0\n")
		} else {
			var exitErr *gossh.ExitError
			if errors.As(err, &exitErr) {
				fmt.Fprintf(os.Stderr, "  → exit %d\n", exitErr.ExitStatus())
			} else {
				fmt.Fprintf(os.Stderr, "  → exit ?\n")
			}
		}
	}
	return err
}

// Upload copies all non-excluded files from localDir to a staging directory
// on the remote, then atomically renames the staging dir to the final target.
//
// remoteBase is the target directory path on the remote (e.g. "/opt/myapp").
// excludes is the list of exclude patterns (from Config.Excludes).
// verbose controls per-file output and SSH command logging:
//   - When verbose=true: per-file lines "  -> relative/path" are written to
//     os.Stderr; each SSH command (mkdir, mv, rm) and its exit code are logged
//     to os.Stderr before/after execution; warnedOnce is never set to true so
//     every sudo warning prints.
//   - When verbose=false: per-file lines are suppressed; SSH command lines are
//     suppressed; warnedOnce behavior is unchanged (first warning prints, rest
//     are suppressed).
//
// Atomic swap strategy:
//  1. Create staging dir: /tmp/docker-deploy-<unixTimestamp> (always writable on remote)
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
func Upload(ctx context.Context, client *gossh.Client, localDir, remoteBase string, excludes []string, sudoPw *string, warnedOnce *bool, verbose bool) (int, error) {
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
	defer sftpClient.Close()

	// Step 4: Derive staging directory in the remote /tmp (always writable).
	// Use nanosecond precision to avoid collisions in concurrent deployments
	// to the same remote in the same second (IN-03).
	timestamp := fmt.Sprintf("%d", time.Now().UnixNano())
	stagingDir := "/tmp/docker-deploy-" + timestamp

	// Step 5: Create staging directory.
	if err := sftpClient.MkdirAll(stagingDir); err != nil {
		return 0, fmt.Errorf("creating staging directory %s: %w", stagingDir, err)
	}

	// Step 6: Upload each file into the staging directory.
	// On any upload error: clean up the partial staging dir (unusable) and return.
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
			localFile, err := os.Open(localPath)
			if err != nil {
				return fmt.Errorf("opening local file %s: %w", localPath, err)
			}

			// Create remote file.
			remoteFile, err := sftpClient.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
			if err != nil {
				localFile.Close()
				return fmt.Errorf("creating remote file %s: %w", remotePath, err)
			}

			// Copy contents.
			if _, err := io.Copy(remoteFile, localFile); err != nil {
				remoteFile.Close()
				localFile.Close()
				return fmt.Errorf("copying %s to remote: %w", relPath, err)
			}

			remoteFile.Close()
			localFile.Close()

			// Preserve source file permissions (e.g. executable bit for scripts).
			if err := sftpClient.Chmod(remotePath, localInfo.Mode().Perm()); err != nil {
				return fmt.Errorf("setting permissions on remote file %s: %w", remotePath, err)
			}

			// Per-file line: written to stderr only when verbose=true (D-01, D-03).
			if verbose {
				fmt.Fprintf(os.Stderr, "  -> %s\n", relPath)
			}
		}
		return nil
	}()

	if uploadErr != nil {
		// Upload failed mid-way — staging dir is partial/unusable, clean it up.
		// Silent best-effort cleanup: discard error.
		_ = sshExec(client, fmt.Sprintf("rm -rf %s", ShellQuote(stagingDir)))
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
		fmt.Fprintf(os.Stderr, "  → exit 0\n")
	}
	if err != nil {
		return 0, fmt.Errorf("checking remote target existence: %w", err)
	}

	// If .env is excluded from upload and the remote target already has a .env file,
	// back it up before the atomic swap. The swap replaces the entire directory, which
	// would silently delete the remote .env even though the operator excluded it
	// intentionally (e.g. via --skip-env). The backup is restored after the swap.
	envBackupPath := ""
	if existsBefore {
		for _, exc := range excludes {
			if exc == ".env" {
				envPath := path.Join(remoteBase, ".env")
				out, checkErr := sshExecOutput(client, fmt.Sprintf("test -f %s && echo exists || echo absent", ShellQuote(envPath)))
				if checkErr == nil && strings.HasPrefix(strings.TrimSpace(out), "exists") {
					envBackupPath = "/tmp/docker-deploy-env-" + timestamp
					if cpErr := sshExec(client, fmt.Sprintf("cp %s %s", ShellQuote(envPath), ShellQuote(envBackupPath))); cpErr != nil {
						fmt.Fprintf(os.Stderr, "WARNING: could not backup remote .env: %v; .env will not be preserved after deploy\n", cpErr)
						envBackupPath = ""
					}
				}
				break
			}
		}
	}

	// sudoRunWithFallback implements the structured auth fallback sequence.
	// It reuses sudoPw across multiple commands to avoid prompting multiple times.
	// The verbose param is captured from the outer Upload() scope.
	sudoRunWithFallback := func(cmd string) error {
		// Step 1: Try direct copy (no privilege escalation).
		if verbose {
			fmt.Fprintf(os.Stderr, "[ssh] %s\n", cmd)
		}
		ok := tryDirectCopy(client, cmd)
		if verbose && ok {
			fmt.Fprintf(os.Stderr, "  → exit 0\n")
		}
		if ok {
			return nil
		}

		// Step 2: Try passwordless sudo.
		sudoCmd := fmt.Sprintf("sudo -n sh -c %s", ShellQuote(cmd))
		if verbose {
			fmt.Fprintf(os.Stderr, "[ssh] %s\n", sudoCmd)
		}
		ok = tryPasswordlessSudo(client, cmd)
		if verbose && ok {
			fmt.Fprintf(os.Stderr, "  → exit 0\n")
		}
		if ok {
			return nil
		}

		// Step 3: Prompt for sudo password interactively (up to 3 attempts).
		// When verbose=true: the warnedOnce guard is bypassed — every warning prints
		// and *warnedOnce is never set to true (D-01).
		// When verbose=false: existing behavior — first warning prints, rest suppressed.
		if verbose || !*warnedOnce {
			fmt.Fprintf(os.Stderr, "WARNING: passwordless sudo not configured; you may be prompted for a password\n")
			if !verbose {
				*warnedOnce = true
			}
		}
		for attempt := 1; attempt <= 3; attempt++ {
			pw, readErr := promptSudoPassword()
			if readErr != nil {
				return readErr
			}
			// Redact the interactive sudo command in verbose output — it contains
			// the literal password (T-07-02-05).
			if verbose {
				fmt.Fprintf(os.Stderr, "[ssh] (sudo password cmd redacted)\n")
			}
			sudoCmd := fmt.Sprintf("echo %s | sudo -S -p '' sh -c %s", ShellQuote(pw), ShellQuote(cmd))
			if sshExec(client, sudoCmd) == nil {
				*sudoPw = pw
				if verbose {
					fmt.Fprintf(os.Stderr, "  → exit 0\n")
				}
				return nil
			}
			if attempt < 3 {
				fmt.Fprintln(os.Stderr, "Sorry, try again.")
			}
		}

		// Step 4: All paths exhausted.
		return fmt.Errorf("could not write to target directory — no valid auth path available (tried direct copy, passwordless sudo, interactive password)")
	}

	// Step 8: Ensure target directory exists.
	if err := sudoRunWithFallback(fmt.Sprintf("mkdir -p %s", ShellQuote(remoteBase))); err != nil {
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
	if existsBefore {
		// Repeat deploy — three-step atomic swap:
		//   1. mv remoteBase to backup
		//   2. mv staging to remoteBase
		//   3. rm -rf backup (non-fatal on failure)
		oldDir := remoteBase + "-old-" + timestamp

		if err := sudoRunWithFallback(fmt.Sprintf("mv %s %s", ShellQuote(remoteBase), ShellQuote(oldDir))); err != nil {
			if envBackupPath != "" {
				_ = sshExec(client, fmt.Sprintf("rm -f %s", ShellQuote(envBackupPath)))
			}
			return 0, fmt.Errorf("renaming existing target to backup: %w", err)
		}
		if err := sudoRunWithFallback(fmt.Sprintf("mv %s %s", ShellQuote(stagingDir), ShellQuote(remoteBase))); err != nil {
			// Rollback: best-effort restore using the same sudoRunWithFallback closure
			// so the mv succeeds even when /opt/… requires elevated privileges.
			rollbackErr := sudoRunWithFallback(fmt.Sprintf("mv %s %s", ShellQuote(oldDir), ShellQuote(remoteBase)))
			// Clean up .env backup on all error paths — the original remoteBase is
			// being restored, so its .env is already present.
			if envBackupPath != "" {
				_ = sshExec(client, fmt.Sprintf("rm -f %s", ShellQuote(envBackupPath)))
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
		if err := sudoRunWithFallback(fmt.Sprintf("rm -rf %s", ShellQuote(oldDir))); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not remove backup directory %s: %v\n", oldDir, err)
		}
	} else {
		// First deploy — move staging directly to target.
		// Step 8 (mkdir -p) created remoteBase as an empty directory. If we now
		// run `mv stagingDir remoteBase`, Unix mv moves stagingDir *inside*
		// remoteBase (because the destination exists) instead of renaming it.
		// Remove the empty placeholder first so that mv performs a clean rename.
		if err := sudoRunWithFallback(fmt.Sprintf("rm -rf %s", ShellQuote(remoteBase))); err != nil {
			return 0, fmt.Errorf("removing target placeholder before first deploy: %w", err)
		}
		if err := sudoRunWithFallback(fmt.Sprintf("mv %s %s", ShellQuote(stagingDir), ShellQuote(remoteBase))); err != nil {
			return 0, fmt.Errorf("moving staging dir to target: %w", err)
		}
	}

	// Restore backed-up .env into the new remoteBase now that the swap is complete.
	// sudoRunWithFallback is used for the copy because remoteBase may require elevated
	// permissions (the same path that handled the mv/rm steps above).
	if envBackupPath != "" {
		envDest := path.Join(remoteBase, ".env")
		if restoreErr := sudoRunWithFallback(fmt.Sprintf("cp %s %s", ShellQuote(envBackupPath), ShellQuote(envDest))); restoreErr != nil {
			fmt.Fprintf(os.Stderr, "WARNING: deploy succeeded but could not restore remote .env: %v\n", restoreErr)
		}
		_ = sshExec(client, fmt.Sprintf("rm -f %s", ShellQuote(envBackupPath)))
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

// sshExec runs a command on the remote via a new SSH session.
// Per CLAUDE.md: each SSH exec must use a fresh NewSession() call.
func sshExec(client *gossh.Client, cmd string) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("creating SSH session: %w", err)
	}
	defer session.Close()

	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("running %q: %w", cmd, err)
	}
	return nil
}

// sshExecOutput runs a command and returns its combined stdout as a string.
func sshExecOutput(client *gossh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("creating SSH session: %w", err)
	}
	defer session.Close()

	out, err := session.Output(cmd)
	if err != nil {
		return "", fmt.Errorf("running %q: %w", cmd, err)
	}
	return string(out), nil
}

// ShellQuote wraps s in single quotes for safe use in shell commands,
// escaping any embedded single quotes using the '\'' technique.
// This handles paths derived from validated config values (remoteBase is from
// Resolve() which validates via ParseHost; staging dir name uses only
// alphanumerics + timestamp integer — T-03-05).
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
