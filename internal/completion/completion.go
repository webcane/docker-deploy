// Package completion provides shell tab-completion support for docker-deploy.
// It registers dynamic flag completion functions and exposes them for use by
// cmd/docker-deploy/main.go. All completion logic lives here per D-08.
package completion

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/webcane/docker-deploy/internal/config"
	"github.com/webcane/docker-deploy/internal/sshconfig"
)

// Register wires the three RegisterFlagCompletionFunc hooks onto cmd.
// Errors from RegisterFlagCompletionFunc are discarded (flags must be defined first).
func Register(cmd *cobra.Command) {
	_ = cmd.RegisterFlagCompletionFunc("host", HostCompletionFunc)
	_ = cmd.RegisterFlagCompletionFunc("path", PathCompletionFunc)
	_ = cmd.RegisterFlagCompletionFunc("compose-file", ComposeFileCompletionFunc)
}

// HostCompletionFunc returns completion candidates for the --host flag.
// It merges the deploy.yaml host value and ~/.ssh/config alias names,
// deduplicated in order. Returns empty on any error (D-03).
func HostCompletionFunc(_ *cobra.Command, _ []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var hosts []string

	// Read deploy.yaml host value (silent failure on error — D-03).
	if fc, _, err := config.LoadFile(cwd); err == nil && fc.Target.Host != "" {
		hosts = append(hosts, fc.Target.Host)
	}

	// Read ~/.ssh/config aliases (silent failure on error — D-03).
	if home, err := os.UserHomeDir(); err == nil {
		aliases := sshconfig.ListHosts(filepath.Join(home, ".ssh", "config"))
		hosts = append(hosts, aliases...)
	}

	return dedupStrings(hosts), cobra.ShellCompDirectiveNoFileComp
}

// PathCompletionFunc returns a single completion candidate for the --path flag:
// "/opt/<cwd-basename>" (D-06). Returns empty on any error (D-03).
func PathCompletionFunc(_ *cobra.Command, _ []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return []cobra.Completion{"/opt/" + filepath.Base(cwd)}, cobra.ShellCompDirectiveNoFileComp
}

// ComposeFileCompletionFunc returns completion candidates for the --compose-file flag.
// It suggests "compose.yaml" and/or "docker-compose.yml" if they exist in cwd (D-07).
// Returns empty when neither file is present.
func ComposeFileCompletionFunc(_ *cobra.Command, _ []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
	var suggestions []cobra.Completion
	for _, name := range []string{"compose.yaml", "docker-compose.yml"} {
		if _, err := os.Stat(name); err == nil {
			suggestions = append(suggestions, cobra.Completion(name))
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

// dedupStrings deduplicates a string slice, preserving first-occurrence order.
// Returns a []cobra.Completion (cobra.Completion = string).
func dedupStrings(in []string) []cobra.Completion {
	seen := make(map[string]bool, len(in))
	out := make([]cobra.Completion, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, cobra.Completion(s))
		}
	}
	return out
}
