//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/webcane/docker-deploy/internal/filetransfer"
)

// buildLargeLocalDir creates a temp directory with n files named file-000.txt through
// file-N.txt, each containing sizeBytes of content. Used to ensure the upload is
// large enough that context cancellation fires mid-transfer.
func buildLargeLocalDir(t *testing.T, n int, sizeBytes int) string {
	t.Helper()
	dir := t.TempDir()
	content := strings.Repeat("x", sizeBytes)
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("file-%03d.txt", i)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("write test file %s: %v", name, err)
		}
	}
	return dir
}

// TestUpload_HappyPath verifies that Upload() transfers files successfully and
// returns n > 0 files transferred with a nil error.
func TestUpload_HappyPath(t *testing.T) {
	client := dialContainer(t, "sshuser")

	localDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte("services: {}\n"), 0644); err != nil {
		t.Fatalf("write compose.yaml: %v", err)
	}

	sudoPw := ""
	warned := false
	n, err := filetransfer.Upload(
		context.Background(),
		client,
		localDir,
		"/opt/testapp-upload-happy",
		[]string{},
		&sudoPw,
		&warned,
		false,
	)
	if err != nil {
		t.Fatalf("Upload returned unexpected error: %v", err)
	}
	if n == 0 {
		t.Error("expected at least 1 file transferred, got 0")
	}

	t.Cleanup(func() {
		// /opt is root-owned; Upload creates the dir via sudo, so removal needs sudo too.
		sshExecHelper(t, client, "sudo rm -rf /opt/testapp-upload-happy")
	})
}

// TestUpload_AtomicCancel verifies SC-5: a context-cancelled mid-transfer leaves the
// sentinel file at /opt/testapp-atomic/sentinel-before-deploy.txt with content "original",
// and no /tmp/docker-deploy-* staging directory remains after the cancelled upload.
func TestUpload_AtomicCancel(t *testing.T) {
	client := dialContainer(t, "sshuser")

	remoteBase := "/opt/testapp-atomic"

	// Register cleanup before any t.Skip — ensures remote dir is always removed.
	// /opt is root-owned; the pre-seed and Upload create the dir via sudo, so removal
	// requires sudo as well (sshuser lacks write permission to /opt itself).
	t.Cleanup(func() {
		sshExecHelper(t, client, "sudo rm -rf /opt/testapp-atomic")
	})

	// Pre-seed sentinel file using sudo — sshuser cannot create dirs in /opt directly.
	sshExecHelper(t, client, "sudo bash -c 'mkdir -p /opt/testapp-atomic && echo original > /opt/testapp-atomic/sentinel-before-deploy.txt'")

	// Create a large enough local dir to ensure context cancellation fires mid-transfer.
	localDir := buildLargeLocalDir(t, 100, 1024)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // ensure cancel is always called even on t.Skip or early return
	// Cancel after 100ms to fire mid-transfer.
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	sudoPw := ""
	warned := false
	_, err := filetransfer.Upload(ctx, client, localDir, remoteBase, []string{}, &sudoPw, &warned, false)

	// If Upload returns nil, the cancel fired too late (all files transferred before context was cancelled).
	if err == nil {
		t.Skip("cancel fired too late — increase file count or delay")
	}

	// Assert sentinel content is still original.
	out := sshExecOutputHelper(t, client, "cat /opt/testapp-atomic/sentinel-before-deploy.txt")
	if strings.TrimSpace(out) != "original" {
		t.Errorf("sentinel file corrupted after cancelled upload; got %q", out)
	}

	// Assert no staging dir remains.
	// The staging dir is co-located with the target (/opt/.deploy-tmp-*) per CLAUDE.md Rule 3
	// to ensure the final mv is an atomic same-filesystem rename(2).
	out2 := sshExecOutputHelper(t, client, "ls /opt/.deploy-tmp-* 2>/dev/null && echo found || echo none")
	if !strings.Contains(out2, "none") {
		t.Errorf("staging dir not cleaned up after cancel; found: %q", out2)
	}

}

// TestUpload_SkipEnv verifies D-04: a pre-seeded .env on the remote is unchanged
// after re-deploy with .env in the excludes list.
func TestUpload_SkipEnv(t *testing.T) {
	client := dialContainer(t, "sshuser")

	remoteBase := "/opt/testapp-skipenv"

	// Pre-seed .env on remote using sudo — sshuser cannot create dirs in /opt directly.
	sshExecHelper(t, client, "sudo bash -c 'mkdir -p /opt/testapp-skipenv && echo SECRET=original > /opt/testapp-skipenv/.env'")

	// Create local dir with only compose.yaml — no .env in localDir.
	localDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte("services: {}\n"), 0644); err != nil {
		t.Fatalf("write compose.yaml: %v", err)
	}

	sudoPw := ""
	warned := false
	_, err := filetransfer.Upload(
		context.Background(),
		client,
		localDir,
		remoteBase,
		[]string{".env"},
		&sudoPw,
		&warned,
		false,
	)
	if err != nil {
		t.Fatalf("Upload with skip-env: %v", err)
	}

	// Assert remote .env is unchanged.
	out := sshExecOutputHelper(t, client, "cat /opt/testapp-skipenv/.env")
	if strings.TrimSpace(out) != "SECRET=original" {
		t.Errorf("remote .env was changed; got %q", out)
	}

	t.Cleanup(func() {
		// /opt is root-owned; removal requires sudo.
		sshExecHelper(t, client, "sudo rm -rf /opt/testapp-skipenv")
	})
}
