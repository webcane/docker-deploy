package completion

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// GenerateZsh writes a zsh completion script for cmd.Root() to w.
// The cmd parameter is the completion subcommand received from RunE;
// cmd.Root() is the deploy command.
func GenerateZsh(cmd *cobra.Command, w io.Writer) error {
	if err := cmd.Root().GenZshCompletion(w); err != nil {
		return fmt.Errorf("generating zsh completion: %w", err)
	}
	return nil
}
