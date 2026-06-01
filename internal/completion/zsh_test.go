// Package completion_test provides external tests for the completion package.
package completion_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/webcane/docker-deploy/internal/completion"
)

// TestGenerateZsh_OutputContainsCompdef verifies that GenerateZsh writes a
// zsh completion script containing the "#compdef" header.
func TestGenerateZsh_OutputContainsCompdef(t *testing.T) {
	cmd := &cobra.Command{Use: "testroot"}
	var buf bytes.Buffer

	if err := completion.GenerateZsh(cmd, &buf); err != nil {
		t.Fatalf("GenerateZsh returned error: %v", err)
	}

	out := buf.String()
	if out == "" {
		t.Fatal("GenerateZsh output is empty")
	}
	if !strings.Contains(out, "#compdef") {
		t.Errorf("GenerateZsh output does not contain '#compdef'; got start: %q", out[:min(100, len(out))])
	}
}
