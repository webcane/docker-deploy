package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/cli/cli-plugins/metadata"
	"github.com/docker/cli/cli-plugins/plugin"
	"github.com/docker/cli/cli/command"
	"github.com/spf13/cobra"

	"github.com/webcane/docker-deploy/internal/compose"
	"github.com/webcane/docker-deploy/internal/config"
	filetransfer "github.com/webcane/docker-deploy/internal/filetransfer"
	"github.com/webcane/docker-deploy/internal/health"
	"github.com/webcane/docker-deploy/internal/preflight"
	sshpkg "github.com/webcane/docker-deploy/internal/ssh"
)

var version = "dev"

// sshDialTimeout is the maximum time to wait for an SSH connection to establish.
// This timeout covers the TCP dial phase; SSH protocol negotiation and authentication
// may take additional time (IN-01).
const sshDialTimeout = 10 * time.Second

func main() {
	plugin.Run(func(dockerCli command.Cli) *cobra.Command {
		return buildDeployCmd()
	}, metadata.Metadata{
		SchemaVersion:    "0.1.0",
		Vendor:           "webcane",
		Version:          version,
		ShortDescription: "Deploy a docker-compose project to a remote VPS",
	})
}

// buildDeployCmd constructs the deploy cobra.Command with all flags registered.
// Extracted from main() to allow testing flag registration without starting the plugin.
func buildDeployCmd() *cobra.Command {
	var host string
	var path string
	var dryRun bool
	var excludes []string
	var force bool
	var composeFile string
	var skipEnv bool
	var verbose bool

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy a docker-compose project to a remote VPS",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun {
				return runDryRun(host, path, excludes, force, composeFile, skipEnv, verbose)
			}
			return runDeploy(host, path, excludes, force, composeFile, skipEnv, verbose)
		},
	}

	cmd.Flags().StringVar(&host, "host", "", "Remote host in ssh://user@host:port format (overrides deploy.yaml)")
	cmd.Flags().StringVar(&path, "path", "", "Remote target directory (overrides deploy.yaml and default /opt/<project>)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Verify SSH connectivity and print resolved config; do not deploy")
	cmd.Flags().StringArrayVar(&excludes, "exclude", nil, "Exclude pattern (repeatable); extends built-in defaults")
	cmd.Flags().BoolVar(&force, "force", false, "Skip replace-confirmation on repeat deploy")
	cmd.Flags().StringVar(&composeFile, "compose-file", "", "Compose file name in project root (default: auto-detect compose.yaml or docker-compose.yml)")
	cmd.Flags().BoolVar(&skipEnv, "skip-env", false, "Exclude .env from upload, leaving remote .env unchanged")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Print per-file transfer lines, SSH commands, and pre-flight checklist to stderr")

	return cmd
}

// rollupMsg returns the warning rollup message shown at the end of runDeploy()
// when at least one warning was accumulated. When verbose is true the message
// omits the --verbose hint because details were already printed inline.
func rollupMsg(verbose bool) string {
	if verbose {
		return "WARN: there are some warnings during deployment."
	}
	return "WARN: there are some warnings during deployment. For more details use --verbose flag"
}

// formatCheckResult formats a single CheckResult for verbose preflight output.
// The bracket notation "[STATUS] name: message" is printed to stderr when
// --verbose is set (D-01).
func formatCheckResult(r preflight.CheckResult) string {
	return fmt.Sprintf("  [%s] %s: %s", strings.ToUpper(r.Status), r.Name, r.Message)
}

// formatHostTarget formats the host+path portion of the deploy complete message.
// When port is 0 or 22 (the default SSH port), the colon separator is omitted to
// avoid the confusing "host:/path" appearance (host:port with empty port).
// Custom ports are rendered as "host:PORT/path".
func formatHostTarget(hostname string, port int, path string) string {
	if port == 0 || port == 22 {
		return hostname + path
	}
	return fmt.Sprintf("%s:%d%s", hostname, port, path)
}

// runDryRun implements the --dry-run flow: Resolve() -> Dial() -> print summary or error.
// The composeFile parameter is accepted for API symmetry with runDeploy but is not
// used during dry-run, since dry-run only verifies SSH connectivity and config resolution
// (IN-02).
func runDryRun(host, path string, excludes []string, force bool, composeFile string, skipEnv bool, verbose bool) error {
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
	// A sentinel composeFile value is passed to skip auto-detection for dry-run.
	// HealthTimeout/HealthInterval are 0 — not registered as CLI flags (deploy.yaml only, Phase 5).
	resolved, err := config.Resolve(config.FlagOpts{
		Host:        host,
		Path:        path,
		Excludes:    excludes,
		Force:       force,
		ComposeFile: "docker-compose.yml", // sentinel: skips auto-detect; value is unused in dry-run
		SkipEnv:     skipEnv,
		Verbose:     verbose,
	}, fileConfig, projectName, cwd)
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
		Timeout:  sshDialTimeout,
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
// Resolve() -> Dial() -> exists-check -> prompt-or-skip -> Upload() -> RunCompose() -> success.
func runDeploy(host, path string, excludes []string, force bool, composeFile string, skipEnv bool, verbose bool) error {
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
	// HealthTimeout/HealthInterval are 0 — not registered as CLI flags (deploy.yaml only, Phase 5).
	resolved, err := config.Resolve(config.FlagOpts{
		Host:        host,
		Path:        path,
		Excludes:    excludes,
		Force:       force,
		ComposeFile: composeFile,
		SkipEnv:     skipEnv,
		Verbose:     verbose,
	}, fileConfig, projectName, cwd)
	if err != nil {
		return fmt.Errorf("resolving config: %w", err)
	}

	// 4. Validate that a host was resolved.
	if resolved.Host.Hostname == "" {
		return fmt.Errorf("no host configured: use --host flag or set target.host in deploy.yaml")
	}

	// 4b. Validate that ComposeFile contains no path separators (T-04-03-01).
	// filepath.Base() strips any leading directory components, so if the result
	// differs from the input the value contains a path separator ('/' on POSIX).
	// This prevents path-traversal (e.g. "../../etc/passwd") but does NOT strip
	// shell metacharacters — that protection is provided by the allowlist check
	// and ShellQuote() inside compose.RunCompose().
	if filepath.Base(resolved.ComposeFile) != resolved.ComposeFile {
		return fmt.Errorf("compose file must be a filename, not a path: %q", resolved.ComposeFile)
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
		Timeout:  sshDialTimeout,
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

	// 6a. Initialize the warning collector (D-02, T-07-02-02).
	// Non-blocking warnings are accumulated here. At the end of runDeploy():
	//   - If verbose=true: each warning is printed inline as it occurs.
	//   - If verbose=false and len(warnings)>0: single rollup message is printed.
	var warnings []string

	// 6b. Skip-env warning (D-09, T-07-02-01).
	// When SkipEnv is true, .env is already excluded via cfg.Excludes (config.Resolve).
	// Emit the warning inline when verbose, or collect for rollup when not verbose.
	if resolved.SkipEnv {
		msg := "WARNING: .env not uploaded — remote .env left unchanged"
		if resolved.Verbose {
			fmt.Fprintln(os.Stderr, msg)
		} else {
			warnings = append(warnings, msg)
		}
	}

	// 6c. Run pre-flight checks before any file operations.
	results, err := preflight.RunPreflightChecks(context.Background(), preflight.NewSSHRunner(client), resolved)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Pre-flight failed: %v\n", err)
		return err
	}

	// 6d. Render pre-flight checklist and collect pre-flight warnings.
	if resolved.Verbose {
		// Verbose: print full checklist to stderr (D-01, Phase 5 deferral fulfilled).
		for _, r := range results {
			fmt.Fprintln(os.Stderr, formatCheckResult(r))
		}
	} else {
		// Non-verbose: collect warn-status results for rollup (D-02).
		for _, r := range results {
			if r.Status == "warn" {
				warnings = append(warnings, fmt.Sprintf("pre-flight: %s: %s", r.Name, r.Message))
			}
		}
	}

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
			// Use bufio.NewReader.ReadString rather than bufio.Scanner to avoid
			// the 64 KB default token-size limit and to be semantically precise
			// for a single-line prompt (WR-02).
			reader := bufio.NewReader(os.Stdin)
			answer, readErr := reader.ReadString('\n')
			if readErr != nil && readErr != io.EOF {
				return fmt.Errorf("reading confirmation: %w", readErr)
			}
			if readErr == io.EOF && strings.TrimSpace(answer) == "" {
				// EOF on stdin with no content — treat as "No" but inform the user.
				fmt.Fprintln(os.Stderr, "No input received — deploy cancelled.")
				return nil
			}
			answer = strings.TrimSpace(answer)
			if !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
				// User declined or pressed Enter (default No) — cancel silently.
				return nil
			}
		}
	}

	// 8. Upload files via SFTP with atomic staging.
	// Upload returns the actual count of files transferred (single filesystem walk).
	// sudoPw is populated during interactive auth fallback and reused across operations.
	// warnedOnce is set to true by Upload when passwordless sudo was unavailable;
	// in non-verbose mode Upload suppresses the inline print so we add it to the rollup here.
	var sudoPw *string
	sudoPw = new(string)
	*sudoPw = ""
	var warnedOnce *bool
	warnedOnce = new(bool)
	*warnedOnce = false
	fileCount, err := filetransfer.Upload(context.Background(), client, cwd, resolved.Path, resolved.Excludes, sudoPw, warnedOnce, resolved.Verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Deploy failed: %v\n", err)
		return err
	}
	if *warnedOnce {
		warnings = append(warnings, "WARNING: passwordless sudo not configured; you may be prompted for a password")
	}

	// 9. Execute docker compose up on the remote host, streaming output locally.
	// RunCompose() writes the failure line to os.Stderr on non-zero exit; no
	// additional wrapping is needed here.
	if err := compose.RunCompose(context.Background(), client, resolved.Path, resolved.ComposeFile, resolved.Verbose); err != nil {
		return err
	}

	// 9b. Poll container health after compose up completes.
	if err := health.PollHealth(context.Background(), client, projectName, resolved); err != nil {
		return err
	}

	// 10. Warning rollup (D-02, T-07-02-02).
	// Always print when warnings occurred. In non-verbose mode adds the --verbose hint
	// since details were suppressed; in verbose mode details were already printed inline.
	if len(warnings) > 0 {
		fmt.Fprintln(os.Stderr, rollupMsg(resolved.Verbose))
	}

	// 11. Print success summary after compose completes successfully.
	fmt.Fprintf(os.Stdout, "Deploy complete: %d files copied to %s\n", fileCount, formatHostTarget(resolved.Host.Hostname, port, resolved.Path))

	return nil
}
