package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/cli/cli-plugins/metadata"
	"github.com/docker/cli/cli-plugins/plugin"
	"github.com/docker/cli/cli/command"
	"github.com/spf13/cobra"

	"github.com/mniedre/docker-deploy/internal/config"
	filetransfer "github.com/mniedre/docker-deploy/internal/filetransfer"
	sshpkg "github.com/mniedre/docker-deploy/internal/ssh"
)

var version = "dev"

func main() {
	plugin.Run(func(dockerCli command.Cli) *cobra.Command {
		var host string
		var path string
		var dryRun bool
		var excludes []string
		var force bool

		cmd := &cobra.Command{
			Use:   "deploy",
			Short: "Deploy a docker-compose project to a remote VPS",
			RunE: func(cmd *cobra.Command, args []string) error {
				if dryRun {
					return runDryRun(host, path, excludes, force)
				}
				return runDeploy(host, path, excludes, force)
			},
		}

		cmd.Flags().StringVar(&host, "host", "", "Remote host in ssh://user@host:port format (overrides deploy.yaml)")
		cmd.Flags().StringVar(&path, "path", "", "Remote target directory (overrides deploy.yaml and default /opt/<project>)")
		cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Verify SSH connectivity and print resolved config; do not deploy")
		cmd.Flags().StringArrayVar(&excludes, "exclude", nil, "Exclude pattern (repeatable); extends built-in defaults")
		cmd.Flags().BoolVar(&force, "force", false, "Skip replace-confirmation on repeat deploy")

		return cmd
	}, metadata.Metadata{
		SchemaVersion:    "0.1.0",
		Vendor:           "mniedre",
		Version:          version,
		ShortDescription: "Deploy a docker-compose project to a remote VPS",
	})
}

// runDryRun implements the --dry-run flow: Resolve() -> Dial() -> print summary or error.
func runDryRun(host, path string, excludes []string, force bool) error {
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
	resolved, err := config.Resolve(host, path, excludes, force, fileConfig, projectName)
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

// runDeploy implements the full deploy flow:
// Resolve() -> Dial() -> exists-check -> prompt-or-skip -> Upload() -> success.
func runDeploy(host, path string, excludes []string, force bool) error {
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
	resolved, err := config.Resolve(host, path, excludes, force, fileConfig, projectName)
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

	// 7. Check if the remote target directory already exists.
	// Use a dedicated SSH session per CLAUDE.md (sessions are NOT reusable).
	if !resolved.Force {
		session, err := client.NewSession()
		if err != nil {
			return fmt.Errorf("creating SSH session for existence check: %w", err)
		}
		out, err := session.Output(fmt.Sprintf("test -d %s && echo exists || echo absent", filetransfer.ShellQuote(resolved.Path)))
		session.Close()
		if err != nil {
			return fmt.Errorf("checking remote target existence: %w", err)
		}

		if strings.HasPrefix(strings.TrimSpace(string(out)), "exists") {
			// Target exists — prompt user for confirmation (default No per D-09, T-03-07).
			fmt.Fprintf(os.Stderr, "Target %s exists on %s. Replace all contents? [y/N] ", resolved.Path, resolved.Host.Hostname)
			scanner := bufio.NewScanner(os.Stdin)
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					return fmt.Errorf("reading confirmation: %w", err)
				}
				// EOF on stdin — treat as "No" but inform the user.
				fmt.Fprintln(os.Stderr, "No input received — deploy cancelled.")
				return nil
			}
			answer := strings.TrimSpace(scanner.Text())
			if !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
				// User declined or pressed Enter (default No) — cancel silently.
				return nil
			}
		}
	}

	// 8. Upload files via SFTP with atomic staging.
	// Upload returns the actual count of files transferred (single filesystem walk).
	fileCount, err := filetransfer.Upload(context.Background(), client, cwd, resolved.Path, resolved.Excludes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Deploy failed: %v\n", err)
		return err
	}

	// 9. Print success summary.
	fmt.Fprintf(os.Stdout, "Deploy complete: %d files copied to %s:%s\n", fileCount, resolved.Host.Hostname, resolved.Path)

	return nil
}
