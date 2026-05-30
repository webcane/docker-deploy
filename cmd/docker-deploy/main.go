// Package main is the entry point for the docker-deploy CLI plugin.
// It registers the plugin with the Docker CLI and wires all subcommands.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
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
var gitCommit = "unknown"
var buildTime = "unknown"

// sshDialTimeout is the maximum time to wait for the full SSH handshake
// (TCP dial + protocol negotiation + authentication) to complete.
// Enforced via goroutine + select in internal/ssh.Dial per CLAUDE.md Rule 2.
const sshDialTimeout = 10 * time.Second

func main() {
	plugin.Run(func(_ command.Cli) *cobra.Command {
		return buildDeployCmd()
	}, metadata.Metadata{
		SchemaVersion:    "0.1.0",
		Vendor:           "webcane",
		Version:          version,
		ShortDescription: "Deploy a docker-compose project to a remote host",
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
	var healthcheckTimeout string
	var healthcheckInterval string
	var healthcheckRetries int
	var skipEnv bool
	var verbose bool

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy a docker-compose project to a remote host",
		RunE: func(_ *cobra.Command, _ []string) error {
			if dryRun {
				return runDryRun(host, path, excludes, force, composeFile, healthcheckTimeout, healthcheckInterval, healthcheckRetries, skipEnv, verbose)
			}
			return runDeploy(host, path, excludes, force, composeFile, healthcheckTimeout, healthcheckInterval, healthcheckRetries, skipEnv, verbose)
		},
	}

	cmd.Flags().StringVar(&host, "host", "", "Remote host: ssh://user@host:port URL or SSH config alias (overrides deploy.yaml)")
	cmd.Flags().StringVar(&path, "path", "", "Remote target directory (overrides deploy.yaml and default /opt/<project>)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Verify SSH connectivity and print resolved config; do not deploy")
	cmd.Flags().StringArrayVar(&excludes, "exclude", nil, "Exclude pattern (repeatable); extends built-in defaults")
	cmd.Flags().BoolVar(&force, "force", false, "Skip replace-confirmation on repeat deploy")
	cmd.Flags().StringVar(&composeFile, "compose-file", "", "Compose file name in project root (default: auto-detect compose.yaml or docker-compose.yml)")
	cmd.Flags().StringVar(&healthcheckTimeout, "healthcheck-timeout", "", "max time to wait for containers to become healthy (Docker-style duration, e.g. 10s, 1m30s)")
	cmd.Flags().StringVar(&healthcheckInterval, "healthcheck-interval", "", "interval between health status polls (Docker-style duration, e.g. 10s)")
	cmd.Flags().IntVar(&healthcheckRetries, "healthcheck-retries", 0, "max consecutive unhealthy results before failing the deploy")
	cmd.Flags().BoolVar(&skipEnv, "skip-env", false, "Exclude .env from upload, leaving remote .env unchanged")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Print per-file transfer lines, SSH commands, and pre-flight checklist to stderr")

	cmd.AddCommand(buildVersionCmd())
	cmd.AddCommand(buildValidateCmd())

	return cmd
}

// buildVersionCmd returns a cobra.Command for the "version" subcommand.
// It prints build metadata and exits 0. No flags are registered (D-04).
func buildVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "version",
		Short:        "Print version information",
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runVersion()
		},
	}
}

// runVersion writes version info to os.Stdout.
// Tagged builds (version != "dev" and buildTime populated) include the Built: line (D-01).
// Dev builds omit it (D-03). OS/Arch is derived at runtime.
func runVersion() error {
	return runVersionTo(os.Stdout)
}

// runVersionTo writes version info to w. Extracted for testability.
func runVersionTo(w io.Writer) error {
	osArch := runtime.GOOS + "/" + runtime.GOARCH
	if version != "dev" && buildTime != "unknown" {
		fmt.Fprintf(w, "Docker Deploy Version %s\n", version)
		fmt.Fprintf(w, "  Git commit:  %s\n", gitCommit)
		fmt.Fprintf(w, "  Built:       %s\n", buildTime)
		fmt.Fprintf(w, "  OS/Arch:     %s\n", osArch)
	} else {
		fmt.Fprintf(w, "Docker Deploy Version %s\n", version)
		fmt.Fprintf(w, "  Git commit:  %s\n", gitCommit)
		fmt.Fprintf(w, "  OS/Arch:     %s\n", osArch)
	}
	return nil
}

// buildValidateCmd returns a cobra.Command for the "validate" subcommand.
// It validates deploy.yaml locally without making any SSH connection (D-08).
// SilenceUsage is set to true to suppress the usage block on error (Pitfall 4).
func buildValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "validate",
		Short:        "Validate deploy.yaml configuration without connecting to remote",
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runValidate()
		},
	}
}

// runValidate validates deploy.yaml from the current working directory without
// making any SSH connection (D-08). It reuses the same cwd-relative config loading
// sequence as runDeploy/runDryRun. On success it prints "✓ deploy.yaml is valid"
// to stdout (D-09). On error it prints the error to stderr and returns it (D-07).
func runValidate() error {
	// 1. Determine cwd.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// 2. Check that deploy.yaml exists before calling LoadFile (D-07).
	// os.Stat is used rather than LoadFile so we can distinguish "missing" from
	// "malformed" and emit the exact "deploy.yaml not found" message.
	// Note: do NOT print the error here; cobra's RunE handler prints the returned
	// error automatically (WR-01: avoid double-printing).
	if _, err := os.Stat(filepath.Join(cwd, "deploy.yaml")); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("deploy.yaml not found")
	}

	// 3. Derive project name from cwd basename.
	projectName := filepath.Base(cwd)

	// 4. Load deploy.yaml.
	fileConfig, _, err := config.LoadFile(cwd)
	if err != nil {
		return fmt.Errorf("loading deploy.yaml: %w", err)
	}

	// 5. Load global config (missing file is OK — treated as empty FileConfig).
	globalCfg, err := loadGlobalConfig()
	if err != nil {
		return fmt.Errorf("loading global config: %w", err)
	}

	// 6. Resolve config with zero FlagOpts — validate flag values only come from the file.
	_, err = config.Resolve(config.FlagOpts{}, fileConfig, globalCfg, projectName, cwd)
	if err != nil {
		return fmt.Errorf("resolving config: %w", err)
	}

	// 7. All checks passed — print success (D-09).
	fmt.Fprintln(os.Stdout, "✓ deploy.yaml is valid")
	return nil
}

// loadGlobalConfig loads the global deploy config from ~/.docker/cli-plugins/deploy.yaml.
// A missing file is treated as an empty FileConfig (not an error). A malformed file is fatal.
// Per D-06: global config is the third tier in four-tier precedence (flag > local > global > zero).
func loadGlobalConfig() (config.FileConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return config.FileConfig{}, fmt.Errorf("cannot determine home directory for global config: %w", err)
	}
	// config.LoadFile expects a directory and appends "deploy.yaml" internally.
	globalDir := filepath.Join(home, ".docker", "cli-plugins")
	globalCfg, _, err := config.LoadFile(globalDir)
	if err != nil {
		return config.FileConfig{}, fmt.Errorf("global config %s: %w", filepath.Join(globalDir, "deploy.yaml"), err)
	}
	return globalCfg, nil
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
func runDryRun(host, path string, excludes []string, force bool, _ string, healthcheckTimeout, healthcheckInterval string, healthcheckRetries int, skipEnv bool, verbose bool) error {
	// 1. Determine projectName from the working directory basename.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	projectName := filepath.Base(cwd)

	// 2. Load deploy.yaml from the current working directory.
	fileConfig, fileExisted, err := config.LoadFile(cwd)
	if err != nil {
		return fmt.Errorf("loading deploy.yaml: %w", err)
	}

	// 3. Load global config (~/.docker/cli-plugins/deploy.yaml); missing file is OK.
	globalCfg, err := loadGlobalConfig()
	if err != nil {
		return fmt.Errorf("loading global config: %w", err)
	}

	// 4. Resolve config with four-tier precedence: flag > local file > global file > zero.
	// A sentinel composeFile value is passed to skip auto-detection for dry-run.
	resolved, err := config.Resolve(config.FlagOpts{
		Host:                host,
		Path:                path,
		Excludes:            excludes,
		Force:               force,
		ComposeFile:         "docker-compose.yml", // sentinel: skips auto-detect; value is unused in dry-run
		HealthcheckTimeout:  healthcheckTimeout,
		HealthcheckInterval: healthcheckInterval,
		HealthcheckRetries:  healthcheckRetries,
		SkipEnv:             skipEnv,
		Verbose:             verbose,
	}, fileConfig, globalCfg, projectName, cwd)
	if err != nil {
		return fmt.Errorf("resolving config: %w", err)
	}

	// 5. Validate that a host was resolved.
	if resolved.Host.Hostname == "" {
		return fmt.Errorf("%w", config.NoHostError(fileExisted, cwd))
	}

	// 6. Build ssh.DialConfig from the resolved config.
	port := resolved.Host.Port
	if port == 0 {
		port = 22
	}
	dialCfg := sshpkg.DialConfig{
		User:       resolved.Host.User,
		Hostname:   resolved.Host.Hostname,
		Port:       port,
		Timeout:    sshDialTimeout,
		Stdin:      os.Stdin,
		UserOutput: os.Stderr,
	}

	// 7. Dial the SSH server.
	client, err := sshpkg.Dial(context.Background(), dialCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SSH connection failed: %v\n", err)
		return fmt.Errorf("SSH dial: %w", err)
	}
	defer client.Close() //nolint:errcheck

	// 8. Determine auth method indicator (best-effort).
	authMethod := "key file (~/.ssh/config)"
	if os.Getenv("SSH_AUTH_SOCK") != "" {
		authMethod = "ssh-agent"
	}

	// 9. Print dry-run summary to stdout.
	fmt.Fprintf(os.Stdout, "Dry-run: connectivity check passed\n")
	fmt.Fprintf(os.Stdout, "  Host:        %s@%s:%d\n", resolved.Host.User, resolved.Host.Hostname, port)
	fmt.Fprintf(os.Stdout, "  Remote path: %s\n", resolved.Path)
	fmt.Fprintf(os.Stdout, "  Auth method: %s\n", authMethod)
	fmt.Fprintf(os.Stdout, "  Server:      %s\n", string(client.ServerVersion()))
	fmt.Fprintf(os.Stdout, "  Status:      OK\n")

	return nil
}

// runDeploy implements the full deploy flow:
// Resolve() -> Dial() -> exists-check -> prompt-or-skip -> Upload() -> RunCompose() -> success.
func runDeploy(host, path string, excludes []string, force bool, composeFile string, healthcheckTimeout, healthcheckInterval string, healthcheckRetries int, skipEnv bool, verbose bool) error { //nolint:gocognit // orchestrates full deploy pipeline (resolve→dial→preflight→upload→compose→health) with verbose branching and warning collection
	// 1. Determine projectName from the working directory basename.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	projectName := filepath.Base(cwd)

	// 2. Load deploy.yaml from the current working directory.
	fileConfig, fileExisted, err := config.LoadFile(cwd)
	if err != nil {
		return fmt.Errorf("loading deploy.yaml: %w", err)
	}

	// 3. Load global config (~/.docker/cli-plugins/deploy.yaml); missing file is OK.
	globalCfg, err := loadGlobalConfig()
	if err != nil {
		return fmt.Errorf("loading global config: %w", err)
	}

	// 4. Resolve config with four-tier precedence: flag > local file > global file > zero.
	resolved, err := config.Resolve(config.FlagOpts{
		Host:                host,
		Path:                path,
		Excludes:            excludes,
		Force:               force,
		ComposeFile:         composeFile,
		HealthcheckTimeout:  healthcheckTimeout,
		HealthcheckInterval: healthcheckInterval,
		HealthcheckRetries:  healthcheckRetries,
		SkipEnv:             skipEnv,
		Verbose:             verbose,
	}, fileConfig, globalCfg, projectName, cwd)
	if err != nil {
		return fmt.Errorf("resolving config: %w", err)
	}

	// 5. Validate that a host was resolved.
	if resolved.Host.Hostname == "" {
		return fmt.Errorf("%w", config.NoHostError(fileExisted, cwd))
	}

	// 5b. Validate that ComposeFile contains no path separators (T-04-03-01).
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
		User:       resolved.Host.User,
		Hostname:   resolved.Host.Hostname,
		Port:       port,
		Timeout:    sshDialTimeout,
		Stdin:      os.Stdin,
		UserOutput: os.Stderr,
	}

	// 6. Dial the SSH server.
	client, err := sshpkg.Dial(context.Background(), dialCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SSH connection failed: %v\n", err)
		return fmt.Errorf("SSH dial: %w", err)
	}
	defer client.Close() //nolint:errcheck

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
		return fmt.Errorf("pre-flight checks: %w", err)
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

	// 7. (confirm prompt is now inside Upload() — see force bool parameter)

	// 8. Upload files via SFTP with atomic staging.
	// Upload returns the actual count of files transferred (single filesystem walk).
	// creds (SudoCreds) captures the sudo password if the interactive fallback fires and
	// reuses it across all SudoExec calls — single prompt per deploy (SC-6).
	// defer creds.Zero() ensures the password bytes are wiped from memory after Upload returns.
	// warnedOnce is set to true by Upload when passwordless sudo was unavailable;
	// in non-verbose mode Upload suppresses the inline print so we add it to the rollup here.
	creds := new(filetransfer.SudoCreds) // SudoCreds captures sudo password for reuse across ops
	defer creds.Zero()
	warnedOnce := new(bool)
	fileCount, err := filetransfer.Upload(context.Background(), client, cwd, resolved.Path, resolved.Excludes, creds, resolved.Force, warnedOnce, resolved.Verbose)
	if errors.Is(err, filetransfer.ErrDeployCancelled) {
		fmt.Fprintln(os.Stderr, "Deploy cancelled.")
		return nil
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Deploy failed: %v\n", err)
		return fmt.Errorf("upload: %w", err)
	}
	if *warnedOnce {
		warnings = append(warnings, "WARNING: passwordless sudo not configured; you may be prompted for a password")
	}

	// 9. Execute docker compose up on the remote host, streaming output locally.
	// RunCompose() writes the failure line to os.Stderr on non-zero exit; no
	// additional wrapping is needed here.
	if err := compose.RunCompose(context.Background(), client, resolved.Path, resolved.ComposeFile, resolved.Verbose); err != nil {
		return fmt.Errorf("compose up: %w", err)
	}

	// 9b. Poll container health after compose up completes.
	if err := health.PollHealth(context.Background(), client, projectName, resolved); err != nil {
		return fmt.Errorf("health poll: %w", err)
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
