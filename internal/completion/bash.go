package completion

import (
	"io"

	"github.com/spf13/cobra"
)

// GenerateBash writes a bash completion script for cmd.Root() to w.
// Uses GenBashCompletionV2 (current standard; requires bash 4.1+).
// false = no descriptions in bash output.
// The cmd parameter is the completion subcommand received from RunE;
// cmd.Root() is the deploy command (confirmed by Pitfall 6 in RESEARCH.md).
func GenerateBash(cmd *cobra.Command, w io.Writer) error {
	return cmd.Root().GenBashCompletionV2(w, false)
}
