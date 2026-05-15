package filetransfer

import (
	"context"
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

// Upload copies all non-excluded files from localDir to a staging directory
// on the remote, then atomically renames the staging dir to the final target.
//
// remoteBase is the target directory path on the remote (e.g. "/opt/myapp").
// excludes is the list of exclude patterns (from Config.Excludes).
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
// Progress: prints "Uploading N files..." before starting, then "  -> relative/path"
// per file to os.Stdout.
//
// Per CLAUDE.md: sessions are NOT reusable — each SSH exec uses a fresh NewSession().
// SFTP wraps the existing *gossh.Client — no second TCP connection.
func Upload(ctx context.Context, client *gossh.Client, localDir, remoteBase string, excludes []string) error {
	// Step 1: Enumerate files to upload.
	files, err := WalkFiles(localDir, excludes)
	if err != nil {
		return fmt.Errorf("enumerating files in %s: %w", localDir, err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no files to upload: all files excluded from %s", localDir)
	}

	// Step 2: Announce upload count.
	fmt.Fprintf(os.Stdout, "Uploading %d files...\n", len(files))

	// Step 3: Open SFTP session wrapping the existing SSH client.
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("opening SFTP session: %w", err)
	}

	// Step 4: Derive staging directory in the remote /tmp (always writable).
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	stagingDir := "/tmp/docker-deploy-" + timestamp

	// Step 5: Create staging directory.
	if err := sftpClient.MkdirAll(stagingDir); err != nil {
		sftpClient.Close()
		return fmt.Errorf("creating staging directory %s: %w", stagingDir, err)
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

			fmt.Fprintf(os.Stdout, "  -> %s\n", relPath)
		}
		return nil
	}()

	// Step 7: Close SFTP session before running SSH mv/rename commands.
	sftpClient.Close()

	if uploadErr != nil {
		// Upload failed mid-way — staging dir is partial/unusable, clean it up.
		_ = sshExec(client, fmt.Sprintf("rm -rf %s", ShellQuote(stagingDir)))
		return uploadErr
	}

	// Step 8: Ensure target directory exists and is owned by the connecting user.
	// Attempt 1 — without sudo (works if user already owns /opt/<project>).
	sudoPw := ""
	mkdirOK := sshExec(client, fmt.Sprintf("mkdir -p %s", ShellQuote(remoteBase))) == nil

	if !mkdirOK {
		// Attempt 2 — interactive sudo: collect password (up to 3 attempts).
		// Single sh -c pipeline: mkdir + chown + chmod to avoid multiple prompts.
		sudoOK := false
		for attempt := 1; attempt <= 3; attempt++ {
			fmt.Fprintf(os.Stderr, "[sudo] password for remote host: ")
			pw, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Fprintln(os.Stderr)
			if err != nil {
				break
			}
			pwStr := string(pw)
			sudoCmd := fmt.Sprintf(
				"echo %s | sudo -S -p '' sh -c 'mkdir -p %s && chown $(id -un):$(id -gn) %s && chmod 755 %s'",
				ShellQuote(pwStr), ShellQuote(remoteBase), ShellQuote(remoteBase), ShellQuote(remoteBase),
			)
			if err := sshExec(client, sudoCmd); err == nil {
				sudoPw = pwStr
				sudoOK = true
				break
			}
			if attempt < 3 {
				fmt.Fprintln(os.Stderr, "Sorry, try again.")
			}
		}

		if !sudoOK {
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
			return fmt.Errorf("could not create target directory %s", remoteBase)
		}
	}

	// Step 9: Check if remoteBase exists on remote (it should now — this guards the
	// case where mkdir -p succeeded but the dir was removed between steps).
	exists, err := remoteExists(client, remoteBase)
	if err != nil {
		return fmt.Errorf("checking remote target existence: %w", err)
	}

	// Step 10: Atomic swap via SSH exec.
	// All mv/rm operations against /opt use sudo when sudoPw was needed for mkdir.
	// sudoRun builds the sudo-prefixed command if sudoPw is non-empty.
	sudoRun := func(cmd string) error {
		if sudoPw == "" {
			return sshExec(client, cmd)
		}
		return sshExec(client, fmt.Sprintf("echo %s | sudo -S -p '' sh -c %s", ShellQuote(sudoPw), ShellQuote(cmd)))
	}

	if exists {
		// Four-step atomic swap:
		// 1. Move staging to /opt space as remoteBase-new-<ts>  (always writable: staging is in /tmp)
		// 2. Backup existing remoteBase to remoteBase-old-<ts>
		// 3. Move remoteBase-new-<ts> to remoteBase             (place new)
		// 4. Remove backup                                       (clean up)
		newDir := remoteBase + "-new-" + timestamp
		oldDir := remoteBase + "-old-" + timestamp

		// Step 10.1: move staging into /opt space
		if err := sudoRun(fmt.Sprintf("mv %s %s", ShellQuote(stagingDir), ShellQuote(newDir))); err != nil {
			return fmt.Errorf("moving staging dir into target space: %w", err)
		}
		// Step 10.2: backup existing target
		if err := sudoRun(fmt.Sprintf("mv %s %s", ShellQuote(remoteBase), ShellQuote(oldDir))); err != nil {
			// Rollback step 10.1
			_ = sudoRun(fmt.Sprintf("mv %s %s", ShellQuote(newDir), ShellQuote(stagingDir)))
			return fmt.Errorf("renaming existing target to backup: %w", err)
		}
		// Step 10.3: place new version
		if err := sudoRun(fmt.Sprintf("mv %s %s", ShellQuote(newDir), ShellQuote(remoteBase))); err != nil {
			// Rollback step 10.2: restore remoteBase from backup
			_ = sudoRun(fmt.Sprintf("mv %s %s", ShellQuote(oldDir), ShellQuote(remoteBase)))
			return fmt.Errorf("renaming staging dir to target: backup is at %s: %w", oldDir, err)
		}
		// Step 10.4: clean up backup (failure is non-fatal — deployment succeeded)
		if err := sudoRun(fmt.Sprintf("rm -rf %s", ShellQuote(oldDir))); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not remove backup directory %s: %v\n", oldDir, err)
		}
	} else {
		// First deploy: move staging dir directly to target (needs sudo if under /opt).
		if err := sudoRun(fmt.Sprintf("mv %s %s", ShellQuote(stagingDir), ShellQuote(remoteBase))); err != nil {
			return fmt.Errorf("moving staging dir to target: %w", err)
		}
	}

	return nil
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
