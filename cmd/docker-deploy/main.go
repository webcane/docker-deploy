package main

import (
	"github.com/docker/cli/cli-plugins/metadata"
	"github.com/docker/cli/cli-plugins/plugin"
	"github.com/docker/cli/cli/command"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	plugin.Run(func(dockerCli command.Cli) *cobra.Command {
		cmd := &cobra.Command{
			Use:   "deploy",
			Short: "Deploy a docker-compose project to a remote VPS",
			RunE: func(cmd *cobra.Command, args []string) error {
				return nil
			},
		}
		return cmd
	}, metadata.Metadata{
		SchemaVersion:    "0.1.0",
		Vendor:           "mniedre",
		Version:          version,
		ShortDescription: "Deploy a docker-compose project to a remote VPS",
	})
}
