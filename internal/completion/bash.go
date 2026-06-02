package completion

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// GenerateBash writes a bash completion script for root to w.
// Uses GenBashCompletionV2 (current standard; requires bash 4.1+).
// false = no descriptions in bash output.
// root must be a standalone cobra.Command named "docker-deploy" so the
// generated script contains the correct command name in its header and
// function names (D-05: static scripts committed to contrib/).
func GenerateBash(root *cobra.Command, w io.Writer) error {
	if err := root.GenBashCompletionV2(w, false); err != nil {
		return fmt.Errorf("generating bash completion: %w", err)
	}
	return nil
}
