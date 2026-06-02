// Package completion provides shell completion script generation.
package completion

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// GenerateZsh writes a zsh completion script for root to w.
// root must be a standalone cobra.Command named "docker-deploy" so the
// generated script contains the correct command name in its header and
// function names (D-05: static scripts committed to contrib/).
func GenerateZsh(root *cobra.Command, w io.Writer) error {
	if err := root.GenZshCompletion(w); err != nil {
		return fmt.Errorf("generating zsh completion: %w", err)
	}
	return nil
}
