// Package completion_test provides external tests for the completion package.
package completion_test

import (
	"os"
	"testing"

	"github.com/spf13/cobra"

	"github.com/webcane/docker-deploy/internal/completion"
)

// TestRegister_SetsHostFlagAnnotation verifies that Register() registers a completion
// function for the "host" flag (verifiable via GetFlagCompletionFunc).
func TestRegister_SetsHostFlagAnnotation(t *testing.T) {
	cmd := &cobra.Command{Use: "testcmd"}
	var host string
	cmd.Flags().StringVar(&host, "host", "", "Remote host")

	completion.Register(cmd)

	if _, ok := cmd.GetFlagCompletionFunc("host"); !ok {
		t.Error("host flag has no completion function registered; expected completion.Register to register one")
	}
}

// TestRegister_SetsPathFlagAnnotation verifies that Register() registers a completion
// function for the "path" flag.
func TestRegister_SetsPathFlagAnnotation(t *testing.T) {
	cmd := &cobra.Command{Use: "testcmd"}
	var path string
	cmd.Flags().StringVar(&path, "path", "", "Remote path")

	completion.Register(cmd)

	if _, ok := cmd.GetFlagCompletionFunc("path"); !ok {
		t.Error("path flag has no completion function registered; expected completion.Register to register one")
	}
}

// TestRegister_SetsComposeFileFlagAnnotation verifies that Register() registers a completion
// function for the "compose-file" flag.
func TestRegister_SetsComposeFileFlagAnnotation(t *testing.T) {
	cmd := &cobra.Command{Use: "testcmd"}
	var cf string
	cmd.Flags().StringVar(&cf, "compose-file", "", "Compose file")

	completion.Register(cmd)

	if _, ok := cmd.GetFlagCompletionFunc("compose-file"); !ok {
		t.Error("compose-file flag has no completion function registered; expected completion.Register to register one")
	}
}

// TestHostCompletionFunc_SilentOnMissingFiles verifies that HostCompletionFunc
// returns without panicking even when deploy.yaml and ~/.ssh/config do not exist.
// D-03: silent failure contract.
func TestHostCompletionFunc_SilentOnMissingFiles(t *testing.T) {
	// Change to an empty tmpdir so deploy.yaml does not exist.
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) }) //nolint:errcheck

	_, directive := completion.HostCompletionFunc(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("HostCompletionFunc directive = %v; want ShellCompDirectiveNoFileComp", directive)
	}
}

// TestPathCompletionFunc_ReturnsPrefixOptSlash verifies that PathCompletionFunc
// always returns at least one candidate starting with "/opt/" (D-06).
func TestPathCompletionFunc_ReturnsPrefixOptSlash(t *testing.T) {
	candidates, directive := completion.PathCompletionFunc(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("PathCompletionFunc directive = %v; want ShellCompDirectiveNoFileComp", directive)
	}
	if len(candidates) == 0 {
		t.Fatal("PathCompletionFunc returned no candidates")
	}
	first := string(candidates[0])
	if len(first) < 5 || first[:5] != "/opt/" {
		t.Errorf("PathCompletionFunc first candidate = %q; want prefix /opt/", first)
	}
}

// TestComposeFileCompletionFunc_EmptyWhenNoneExist verifies that ComposeFileCompletionFunc
// returns an empty slice when neither compose.yaml nor docker-compose.yml exist in cwd.
func TestComposeFileCompletionFunc_EmptyWhenNoneExist(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) }) //nolint:errcheck

	candidates, _ := completion.ComposeFileCompletionFunc(nil, nil, "")
	if len(candidates) != 0 {
		t.Errorf("ComposeFileCompletionFunc got %d candidates; want 0 (no compose files in tmpdir)", len(candidates))
	}
}

// TestComposeFileCompletionFunc_SuggestsWhenPresent verifies that ComposeFileCompletionFunc
// suggests "compose.yaml" when it exists in cwd (D-07).
func TestComposeFileCompletionFunc_SuggestsWhenPresent(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) }) //nolint:errcheck

	// Create compose.yaml in the tmpdir.
	if err := os.WriteFile("compose.yaml", []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}

	candidates, _ := completion.ComposeFileCompletionFunc(nil, nil, "")
	found := false
	for _, c := range candidates {
		if string(c) == "compose.yaml" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ComposeFileCompletionFunc candidates = %v; want to contain 'compose.yaml'", candidates)
	}
}
