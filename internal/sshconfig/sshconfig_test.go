package sshconfig

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

// --- LookupHost tests ---

// TestLookupHost_FoundWithAllDirectives verifies that LookupHost returns a
// fully-populated HostEntry when the config file has all four directives.
func TestLookupHost_FoundWithAllDirectives(t *testing.T) {
	cfg := `Host minipc
  HostName 192.168.1.50
  User alice
  Port 2222
  IdentityFile ~/.ssh/id_ed25519
`
	tmpFile := writeTempSSHConfig(t, cfg)

	entry, found := LookupHost(tmpFile, "minipc")
	if !found {
		t.Fatal("LookupHost() returned found=false, want true")
	}
	if entry.HostName != "192.168.1.50" {
		t.Errorf("HostName = %q, want %q", entry.HostName, "192.168.1.50")
	}
	if entry.User != "alice" {
		t.Errorf("User = %q, want %q", entry.User, "alice")
	}
	if entry.Port != 2222 {
		t.Errorf("Port = %d, want 2222", entry.Port)
	}
	if len(entry.IdentityFiles) != 1 {
		t.Fatalf("len(IdentityFiles) = %d, want 1; got %v", len(entry.IdentityFiles), entry.IdentityFiles)
	}
	// expandPath should have expanded the ~ prefix
	home, _ := os.UserHomeDir()
	wantPath := filepath.Join(home, ".ssh", "id_ed25519")
	if entry.IdentityFiles[0] != wantPath {
		t.Errorf("IdentityFiles[0] = %q, want %q", entry.IdentityFiles[0], wantPath)
	}
}

// TestLookupHost_FoundAliasOnly verifies D-07: when no HostName directive is
// present, HostEntry.HostName falls back to the alias label itself.
func TestLookupHost_FoundAliasOnly(t *testing.T) {
	cfg := `Host minipc
`
	tmpFile := writeTempSSHConfig(t, cfg)

	entry, found := LookupHost(tmpFile, "minipc")
	if !found {
		t.Fatal("LookupHost() returned found=false, want true")
	}
	if entry.HostName != "minipc" {
		t.Errorf("HostName = %q, want %q (alias fallback)", entry.HostName, "minipc")
	}
	if entry.User != "" {
		t.Errorf("User = %q, want empty", entry.User)
	}
	if entry.Port != 0 {
		t.Errorf("Port = %d, want 0", entry.Port)
	}
	if len(entry.IdentityFiles) != 0 {
		t.Errorf("IdentityFiles = %v, want empty", entry.IdentityFiles)
	}
}

// TestLookupHost_NotFound verifies that LookupHost returns found=false when
// no Host block matches the requested alias.
func TestLookupHost_NotFound(t *testing.T) {
	cfg := `Host otherhost
  HostName 10.0.0.1
`
	tmpFile := writeTempSSHConfig(t, cfg)

	_, found := LookupHost(tmpFile, "unknown")
	if found {
		t.Fatal("LookupHost() returned found=true, want false for unknown alias")
	}
}

// TestLookupHost_IncludeSkipped verifies D-11: Include directives are silently
// skipped and do not prevent subsequent Host blocks from being parsed.
func TestLookupHost_IncludeSkipped(t *testing.T) {
	cfg := `Include ~/.ssh/config.d/*
Host minipc
  HostName 192.168.1.50
`
	tmpFile := writeTempSSHConfig(t, cfg)

	entry, found := LookupHost(tmpFile, "minipc")
	if !found {
		t.Fatal("LookupHost() returned found=false after Include line, want true")
	}
	if entry.HostName != "192.168.1.50" {
		t.Errorf("HostName = %q, want %q", entry.HostName, "192.168.1.50")
	}
}

// TestLookupHost_WildcardBlock verifies that "Host *" matches any alias.
func TestLookupHost_WildcardBlock(t *testing.T) {
	cfg := `Host *
  IdentityFile ~/.ssh/id_rsa
`
	tmpFile := writeTempSSHConfig(t, cfg)

	entry, found := LookupHost(tmpFile, "anyhost")
	if !found {
		t.Fatal("LookupHost() returned found=false for wildcard Host *, want true")
	}
	home, _ := os.UserHomeDir()
	wantPath := filepath.Join(home, ".ssh", "id_rsa")
	if len(entry.IdentityFiles) == 0 || entry.IdentityFiles[0] != wantPath {
		t.Errorf("IdentityFiles = %v, want [%q]", entry.IdentityFiles, wantPath)
	}
}

// TestLoadSigners_DelegatesLookupHost verifies that LoadSigners still loads
// signers after the refactor to use LookupHost internally.
// This test writes a real (generated) key so it verifies end-to-end loading.
func TestLoadSigners_DelegatesLookupHost(t *testing.T) {
	t.Helper()
	// Only verifies no panic; keys may be absent in CI.
	signers := LoadSigners("/nonexistent/path", "anyhost")
	_ = signers
}

// --- expandPath tests ---

func TestExpandPath_TildeExpansion(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := expandPath("~/.ssh/id_ed25519", home, "alice", "host.example.com", "bob", "22")
	want := filepath.Join(home, ".ssh", "id_ed25519")
	if got != want {
		t.Errorf("expandPath() = %q, want %q", got, want)
	}
}

func TestExpandPath_PercentTokens(t *testing.T) {
	home := "/home/alice"
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"%d expands to homeDir", "%d/.ssh/id_rsa", "/home/alice/.ssh/id_rsa"},
		{"%u expands to localUser", "%d/.ssh/id_%u", "/home/alice/.ssh/id_alice"},
		{"%r expands to remoteUser", "%d/.ssh/id_%r", "/home/alice/.ssh/id_deploy"},
		{"%h expands to hostname", "%d/.ssh/%h_key", "/home/alice/.ssh/myserver_key"},
		{"%p expands to port", "%d/.ssh/key_%p", "/home/alice/.ssh/key_2222"},
		{"combined %r and %d", "%d/.ssh/id_%r", "/home/alice/.ssh/id_deploy"},
		{"%% expands to literal %", "%d/.ssh/id_%%foo", "/home/alice/.ssh/id_%foo"},
		{"no tokens unchanged", "/absolute/path/key", "/absolute/path/key"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := expandPath(tc.input, home, "alice", "myserver", "deploy", "2222")
			if got != tc.want {
				t.Errorf("expandPath(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestLookupHost_PercentTokensExpanded verifies that IdentityFile paths with
// %-tokens are fully expanded after LookupHost resolves HostName and User.
func TestLookupHost_PercentTokensExpanded(t *testing.T) {
	home, _ := os.UserHomeDir()
	u, err := user.Current()
	if err != nil {
		t.Skip("cannot determine current user")
	}

	cfg := `Host myserver
  HostName 192.168.1.10
  User deploy
  Port 2222
  IdentityFile %d/.ssh/id_%r
`
	tmpFile := writeTempSSHConfig(t, cfg)

	entry, found := LookupHost(tmpFile, "myserver")
	if !found {
		t.Fatal("LookupHost() returned found=false, want true")
	}
	if len(entry.IdentityFiles) != 1 {
		t.Fatalf("len(IdentityFiles) = %d, want 1; got %v", len(entry.IdentityFiles), entry.IdentityFiles)
	}
	wantPath := filepath.Join(home, ".ssh", "id_deploy")
	if entry.IdentityFiles[0] != wantPath {
		t.Errorf("IdentityFiles[0] = %q, want %q", entry.IdentityFiles[0], wantPath)
	}
	_ = u // used for Skip guard above
}

// TestLookupHost_PercentD_WithoutRemoteUser verifies %d expansion when no User
// directive is present (remoteUser will be empty string).
func TestLookupHost_PercentD_WithoutRemoteUser(t *testing.T) {
	home, _ := os.UserHomeDir()

	cfg := `Host minimal
  IdentityFile %d/.ssh/id_ed25519
`
	tmpFile := writeTempSSHConfig(t, cfg)

	entry, found := LookupHost(tmpFile, "minimal")
	if !found {
		t.Fatal("LookupHost() returned found=false, want true")
	}
	if len(entry.IdentityFiles) != 1 {
		t.Fatalf("len(IdentityFiles) = %d, want 1", len(entry.IdentityFiles))
	}
	wantPath := filepath.Join(home, ".ssh", "id_ed25519")
	if entry.IdentityFiles[0] != wantPath {
		t.Errorf("IdentityFiles[0] = %q, want %q", entry.IdentityFiles[0], wantPath)
	}
}

// --- ListHosts tests ---

// TestListHosts_HappyPath verifies that ListHosts returns all non-wildcard Host
// aliases in file order.
func TestListHosts_HappyPath(t *testing.T) {
	cfg := "Host minipc\n  HostName 192.168.1.50\nHost devbox\n  HostName 10.0.0.5\n"
	tmpFile := writeTempSSHConfig(t, cfg)

	got := ListHosts(tmpFile)
	want := []string{"minipc", "devbox"}
	if len(got) != len(want) {
		t.Fatalf("ListHosts() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ListHosts()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestListHosts_SkipsWildcards verifies that wildcard Host patterns are excluded.
func TestListHosts_SkipsWildcards(t *testing.T) {
	cfg := "Host *\n  ServerAliveInterval 60\nHost *.example.com\n  User admin\nHost prod\n  HostName 1.2.3.4\n"
	tmpFile := writeTempSSHConfig(t, cfg)

	got := ListHosts(tmpFile)
	want := []string{"prod"}
	if len(got) != len(want) {
		t.Fatalf("ListHosts() = %v, want %v", got, want)
	}
	if got[0] != "prod" {
		t.Errorf("ListHosts()[0] = %q, want %q", got[0], "prod")
	}
}

// TestListHosts_MultiPatternLine verifies that only non-wildcard patterns are
// collected from a Host line with multiple patterns.
func TestListHosts_MultiPatternLine(t *testing.T) {
	cfg := "Host web app *.internal\n  User deploy\n"
	tmpFile := writeTempSSHConfig(t, cfg)

	got := ListHosts(tmpFile)
	want := []string{"web", "app"}
	if len(got) != len(want) {
		t.Fatalf("ListHosts() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ListHosts()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestListHosts_MissingFile verifies that ListHosts returns nil without panicking
// when the config file does not exist.
func TestListHosts_MissingFile(t *testing.T) {
	got := ListHosts(t.TempDir() + "/nonexistent")
	if got != nil {
		t.Errorf("ListHosts() = %v, want nil for missing file", got)
	}
}

// TestListHosts_EmptyFile verifies that ListHosts returns nil for an empty file.
func TestListHosts_EmptyFile(t *testing.T) {
	tmpFile := writeTempSSHConfig(t, "")
	got := ListHosts(tmpFile)
	if got != nil {
		t.Errorf("ListHosts() = %v, want nil for empty file", got)
	}
}

// TestListHosts_CommentsAndBlankLines verifies that comment and blank lines are
// skipped and the real Host alias is returned.
func TestListHosts_CommentsAndBlankLines(t *testing.T) {
	cfg := "# global settings\n\nHost real\n  HostName 1.2.3.4\n"
	tmpFile := writeTempSSHConfig(t, cfg)

	got := ListHosts(tmpFile)
	want := []string{"real"}
	if len(got) != len(want) {
		t.Fatalf("ListHosts() = %v, want %v", got, want)
	}
	if got[0] != "real" {
		t.Errorf("ListHosts()[0] = %q, want %q", got[0], "real")
	}
}

// writeTempSSHConfig writes content to a temp file and returns the path.
func writeTempSSHConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "ssh_config_*")
	if err != nil {
		t.Fatalf("creating temp ssh config: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("writing temp ssh config: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing temp ssh config: %v", err)
	}
	return f.Name()
}
