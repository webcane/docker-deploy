// Package completion_test provides external tests for the completion package.
package completion_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/webcane/docker-deploy/internal/completion"
)

// TestGenerateBash_OutputContainsBashHeader verifies that GenerateBash writes a
// non-empty bash completion script with a comment header.
func TestGenerateBash_OutputContainsBashHeader(t *testing.T) {
	cmd := &cobra.Command{Use: "testroot"}
	var buf bytes.Buffer

	if err := completion.GenerateBash(cmd, &buf); err != nil {
		t.Fatalf("GenerateBash returned error: %v", err)
	}

	out := buf.String()
	if out == "" {
		t.Fatal("GenerateBash output is empty")
	}
	if !strings.Contains(out, "#") {
		t.Errorf("GenerateBash output does not contain '#'; got: %q", out[:min(100, len(out))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
