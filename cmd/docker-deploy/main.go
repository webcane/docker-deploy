package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/cli/cli-plugins/metadata"
	"github.com/docker/cli/cli-plugins/plugin"
	"github.com/docker/cli/cli/command"
	"github.com/spf13/cobra"

	"github.com/mniedre/docker-deploy/internal/config"
	sshpkg "github.com/mniedre/docker-deploy/internal/ssh"
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
					return runDryRun(host, path)
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

// runDryRun implements the --dry-run flow: Resolve() -> Dial() -> print summary or error.
func runDryRun(host, path string) error {
	// 1. Determine projectName from the working directory basename.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	projectName := filepath.Base(cwd)

	// 2. Load deploy.yaml from the current working directory.
	fileConfig, err := config.LoadFile(cwd)
	if err != nil {
		return fmt.Errorf("loading deploy.yaml: %w", err)
	}

	// 3. Resolve config with flag > file > default precedence.
	// TODO(phase3): wire --exclude and --force flags; for now pass nil/false.
	resolved, err := config.Resolve(host, path, nil, false, fileConfig, projectName)
	if err != nil {
		return fmt.Errorf("resolving config: %w", err)
	}

	// 4. Validate that a host was resolved.
	if resolved.Host.Hostname == "" {
		return fmt.Errorf("no host configured: use --host flag or set target.host in deploy.yaml")
	}

	// 5. Build ssh.DialConfig from the resolved config.
	port := resolved.Host.Port
	if port == 0 {
		port = 22
	}
	dialCfg := sshpkg.DialConfig{
		User:     resolved.Host.User,
		Hostname: resolved.Host.Hostname,
		Port:     port,
		Timeout:  10 * time.Second,
		Stdin:    os.Stdin,
		Stdout:   os.Stderr,
	}

	// 6. Dial the SSH server.
	client, err := sshpkg.Dial(context.Background(), dialCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SSH connection failed: %v\n", err)
		return err
	}
	defer client.Close()

	// 7. Determine auth method indicator (best-effort).
	authMethod := "key file (~/.ssh/config)"
	if os.Getenv("SSH_AUTH_SOCK") != "" {
		authMethod = "ssh-agent"
	}

	// 8. Print dry-run summary to stdout.
	fmt.Fprintf(os.Stdout, "Dry-run: connectivity check passed\n")
	fmt.Fprintf(os.Stdout, "  Host:        %s@%s:%d\n", resolved.Host.User, resolved.Host.Hostname, port)
	fmt.Fprintf(os.Stdout, "  Remote path: %s\n", resolved.Path)
	fmt.Fprintf(os.Stdout, "  Auth method: %s\n", authMethod)
	fmt.Fprintf(os.Stdout, "  Server:      %s\n", string(client.Conn.ServerVersion()))
	fmt.Fprintf(os.Stdout, "  Status:      OK\n")

	return nil
}
