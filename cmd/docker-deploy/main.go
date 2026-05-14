package main

import (
	"fmt"

	"github.com/docker/cli/cli-plugins/metadata"
	"github.com/docker/cli/cli-plugins/plugin"
	"github.com/docker/cli/cli/command"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	plugin.Run(func(dockerCli command.Cli) *cobra.Command {
		var host string
		var path string
		var dryRun bool

		cmd := &cobra.Command{
			Use:   "deploy",
			Short: "Deploy a docker-compose project to a remote VPS",
			RunE: func(cmd *cobra.Command, args []string) error {
				if dryRun {
					// Connectivity verification only — wired in plan 02-03
					_ = host
					_ = path
					return fmt.Errorf("--dry-run: not wired yet")
				}
				_ = host
				_ = path
				return fmt.Errorf("deploy not implemented yet")
			},
		}

		cmd.Flags().StringVar(&host, "host", "", "Remote host in ssh://user@host:port format (overrides deploy.yaml)")
		cmd.Flags().StringVar(&path, "path", "", "Remote target directory (overrides deploy.yaml and default /opt/<project>)")
		cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Verify SSH connectivity and print resolved config; do not deploy")

		return cmd
	}, metadata.Metadata{
		SchemaVersion:    "0.1.0",
		Vendor:           "mniedre",
		Version:          version,
		ShortDescription: "Deploy a docker-compose project to a remote VPS",
	})
}
