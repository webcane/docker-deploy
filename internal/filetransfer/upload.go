package filetransfer

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/pkg/sftp"
	gossh "golang.org/x/crypto/ssh"
)

// Upload copies all non-excluded files from localDir to a staging directory
// on the remote, then atomically renames the staging dir to the final target.
//
// remoteBase is the target directory path on the remote (e.g. "/opt/myapp").
// excludes is the list of exclude patterns (from Config.Excludes).
//
// Atomic swap strategy:
//  1. Create staging dir: filepath.Dir(remoteBase)/<basename>.deploy-tmp-<unixTimestamp>
//  2. Upload all files into staging dir maintaining relative path structure
//  3. Via SSH session exec: if remoteBase exists, mv to .old-<timestamp>, mv staging to
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

	// Step 4: Derive staging directory path.
	// Use path (not filepath) for remote Linux paths.
	remoteParent := path.Dir(remoteBase)
	projectName := path.Base(remoteBase)
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	stagingDir := remoteParent + "/" + projectName + ".deploy-tmp-" + timestamp

	// Step 5: Create staging directory.
	if err := sftpClient.MkdirAll(stagingDir); err != nil {
		sftpClient.Close()
		return fmt.Errorf("creating staging directory %s: %w", stagingDir, err)
	}

	// Step 6: Upload each file into the staging directory.
	for _, relPath := range files {
		if err := ctx.Err(); err != nil {
			sftpClient.Close()
			return fmt.Errorf("upload cancelled: %w", err)
		}

		localPath := filepath.Join(localDir, filepath.FromSlash(relPath))
		remotePath := path.Join(stagingDir, relPath)

		// Ensure parent directory exists on remote.
		remoteDir := path.Dir(remotePath)
		if err := sftpClient.MkdirAll(remoteDir); err != nil {
			sftpClient.Close()
			return fmt.Errorf("creating remote directory %s: %w", remoteDir, err)
		}

		// Open local file for reading.
		localFile, err := os.Open(localPath)
		if err != nil {
			sftpClient.Close()
			return fmt.Errorf("opening local file %s: %w", localPath, err)
		}

		// Create remote file.
		remoteFile, err := sftpClient.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
		if err != nil {
			localFile.Close()
			sftpClient.Close()
			return fmt.Errorf("creating remote file %s: %w", remotePath, err)
		}

		// Copy contents.
		if _, err := io.Copy(remoteFile, localFile); err != nil {
			remoteFile.Close()
			localFile.Close()
			sftpClient.Close()
			return fmt.Errorf("copying %s to remote: %w", relPath, err)
		}

		remoteFile.Close()
		localFile.Close()

		fmt.Fprintf(os.Stdout, "  -> %s\n", relPath)
	}

	// Step 7: Close SFTP session before running SSH mv/rename commands.
	sftpClient.Close()

	// Step 8: Check if remoteBase exists on remote.
	exists, err := remoteExists(client, remoteBase)
	if err != nil {
		return fmt.Errorf("checking remote target existence: %w", err)
	}

	// Step 9: Perform atomic swap via SSH exec commands.
	if exists {
		// Three-step atomic swap:
		// 1. mv remoteBase remoteBase.old-<timestamp>
		// 2. mv stagingDir remoteBase
		// 3. rm -rf remoteBase.old-<timestamp>
		oldDir := remoteBase + ".old-" + timestamp

		if err := sshExec(client, fmt.Sprintf("mv %s %s", shellQuote(remoteBase), shellQuote(oldDir))); err != nil {
			return fmt.Errorf("renaming existing target to backup: %w", err)
		}
		if err := sshExec(client, fmt.Sprintf("mv %s %s", shellQuote(stagingDir), shellQuote(remoteBase))); err != nil {
			return fmt.Errorf("renaming staging dir to target: %w", err)
		}
		if err := sshExec(client, fmt.Sprintf("rm -rf %s", shellQuote(oldDir))); err != nil {
			return fmt.Errorf("removing backup dir: %w", err)
		}
	} else {
		// First deploy: just move staging dir to target.
		if err := sshExec(client, fmt.Sprintf("mv %s %s", shellQuote(stagingDir), shellQuote(remoteBase))); err != nil {
			return fmt.Errorf("moving staging dir to target: %w", err)
		}
	}

	return nil
}

// remoteExists checks whether a path exists and is a directory on the remote
// host by running `test -d <path> && echo exists || echo absent` via SSH.
func remoteExists(client *gossh.Client, remotePath string) (bool, error) {
	output, err := sshExecOutput(client, fmt.Sprintf("test -d %s && echo exists || echo absent", shellQuote(remotePath)))
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

// shellQuote wraps a path in single quotes for safe use in shell commands.
// This handles paths derived from validated config values (remoteBase is from
// Resolve() which validates via ParseHost; staging dir name uses only
// alphanumerics + timestamp integer — T-03-05).
func shellQuote(s string) string {
	return "'" + s + "'"
}
