// Package config implements configuration resolution for docker-deploy.
// It supports four-tier precedence: CLI flags > local deploy.yaml > global config > zero value.
package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/webcane/docker-deploy/internal/sshconfig"
)

// defaultExcludes is the built-in exclude list that is always active.
// User-supplied excludes (via deploy.yaml or --exclude flag) extend this list;
// they cannot remove these entries.
var defaultExcludes = []string{
	".git/", "node_modules/", "vendor/", "*.log", ".DS_Store", "__pycache__/",
	".claude/", ".github/", ".planning/", ".idea/", ".vscode/",
	"*.swp", "*.swo", "coverage/", "dist/", ".terraform/",
}

// Host holds a parsed SSH host specification.
type Host struct {
	User     string
	Hostname string
	Port     int
}

// healthcheckYAML is the YAML-parsing form of the target.healthcheck sub-block.
// Duration values are strings (e.g. "10s", "1m30s") and are converted to
// time.Duration by Resolve(). This struct is unexported; callers use HealthcheckConfig.
type healthcheckYAML struct {
	Interval string `yaml:"interval"`
	Timeout  string `yaml:"timeout"`
	Retries  int    `yaml:"retries"`
}

// HealthcheckConfig is the resolved runtime form of the healthcheck configuration.
// Fields are zero values when no healthcheck block is present in any config source,
// which signals that health polling should be skipped entirely (per D-04).
type HealthcheckConfig struct {
	Interval time.Duration
	Timeout  time.Duration
	Retries  int
}

// TargetConfig holds the single-target subsection of deploy.yaml.
// Future phases will add a "targets" (plural) map for named targets.
// The Healthcheck sub-block uses Docker-style duration strings (e.g. "10s", "1m30s").
// Precedence: CLI flags > local deploy.yaml > global config > absent (zero value).
type TargetConfig struct {
	Host        string          `yaml:"host"`
	Path        string          `yaml:"path"`
	Exclude     []string        `yaml:"exclude"`
	Force       bool            `yaml:"force"`
	ComposeFile string          `yaml:"compose_file"`
	Healthcheck healthcheckYAML `yaml:"healthcheck"`
	SkipEnv     bool            `yaml:"skip_env"`
}

// FileConfig is the top-level structure of deploy.yaml.
// Version is used for future schema migration; Target is the single-target config.
type FileConfig struct {
	Version int          `yaml:"version"`
	Target  TargetConfig `yaml:"target"`
}

// Config is the fully resolved runtime configuration.
type Config struct {
	Host        Host
	Path        string
	DryRun      bool
	Excludes    []string          // merged: defaultExcludes + file.Target.Exclude + flagExcludes, deduplicated
	Force       bool              // flag || file.Target.Force (flag > deploy.yaml > false)
	ComposeFile string            // resolved compose filename basename (flag > deploy.yaml > auto-detect)
	Healthcheck HealthcheckConfig // resolved health polling config; zero value means skip polling (per D-04)
	SkipEnv     bool              // opts.SkipEnv || file.Target.SkipEnv; appends .env to Excludes when true
	Verbose     bool              // opts.Verbose; enables detailed output lines
}

// FlagOpts holds all CLI-flag values passed to Resolve().
// It replaces the previous positional-params signature, making it easier to
// add new flags without breaking callers.
// Healthcheck flags use Docker-style duration strings (e.g. "10s", "1m30s") for
// HealthcheckTimeout and HealthcheckInterval, and an integer for HealthcheckRetries.
// Precedence: flag > local deploy.yaml target.healthcheck > global config target.healthcheck > absent (zero value).
type FlagOpts struct {
	Host                   string
	Path                   string
	Excludes               []string
	Force                  bool
	ComposeFile            string
	HealthcheckTimeout     string
	HealthcheckInterval    string
	HealthcheckRetries     int
	HealthcheckRetriesSet  bool // true when --healthcheck-retries was explicitly provided (allows 0 to override file config)
	SkipEnv                bool
	Verbose                bool
}

// isValidUnixUsername reports whether s is a valid Unix username, i.e. it
// consists only of letters, digits, '.', '_', or '-', and is non-empty.
// This is enforced on usernames extracted from host URLs to prevent
// shell-special characters from appearing in operator-facing error messages
// (e.g. the sudoers suggestion in checkSudo).
func isValidUnixUsername(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '.' && r != '_' && r != '-' {
			return false
		}
	}
	return true
}

// ParseHost parses an SSH URI of the form ssh://[user@]host[:port].
//
// An empty rawURL is valid and returns a zero Host with no error — the caller
// (Resolve) is responsible for validating that a host was eventually supplied.
//
// Returns an error if the scheme is not "ssh", if the hostname is empty after
// parsing, or if the port is present but not a valid integer.
func ParseHost(rawURL string) (Host, error) {
	if rawURL == "" {
		return Host{}, nil
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return Host{}, fmt.Errorf("invalid host URL %q: %w", rawURL, err)
	}

	if u.Scheme != "ssh" {
		return Host{}, fmt.Errorf("invalid host URL %q: scheme must be \"ssh\", got %q", rawURL, u.Scheme)
	}

	hostname := u.Hostname()
	if hostname == "" {
		return Host{}, fmt.Errorf("invalid host URL %q: hostname is empty", rawURL)
	}

	port := 22
	if portStr := u.Port(); portStr != "" {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return Host{}, fmt.Errorf("invalid host URL %q: port %q is not a valid integer", rawURL, portStr)
		}
		if port < 1 || port > 65535 {
			return Host{}, fmt.Errorf("invalid host URL %q: port %d is out of range (1-65535)", rawURL, port)
		}
	}

	var user string
	if u.User != nil {
		user = u.User.Username()
		if user != "" && !isValidUnixUsername(user) {
			return Host{}, fmt.Errorf("invalid host URL %q: username %q contains disallowed characters (allowed: a-z A-Z 0-9 . _ -)", rawURL, user)
		}
	}

	return Host{
		User:     user,
		Hostname: hostname,
		Port:     port,
	}, nil
}

// LoadFile reads deploy.yaml from dir. The returned bool reports whether the file
// was found on disk (true = file existed, false = file absent or unreadable for
// a non-NotExist reason). A malformed YAML file returns (FileConfig{}, true, error).
// If no deploy.yaml exists, it returns (FileConfig{}, false, nil).
// Non-NotExist read errors (e.g. permission denied) return (FileConfig{}, false, error)
// — treating the file as absent is conservative; callers emit the not-found message
// variant which is less misleading than a partial state message.
func LoadFile(dir string) (FileConfig, bool, error) {
	path := filepath.Join(dir, "deploy.yaml")
	data, err := os.ReadFile(path) //nolint:gosec // path is filepath.Join(dir, "deploy.yaml") where dir comes from os.Getwd(), a trusted location
	if err != nil {
		if os.IsNotExist(err) {
			return FileConfig{}, false, nil
		}
		return FileConfig{}, false, fmt.Errorf("reading deploy.yaml: %w", err)
	}

	var fc FileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return FileConfig{}, true, fmt.Errorf("parsing deploy.yaml: %w", err)
	}

	return fc, true, nil
}

// NoHostError returns a context-specific error for when no SSH host is configured.
// When fileExisted is false (deploy.yaml was not found), the error indicates the
// file is missing and suggests using the --host flag. When fileExisted is true
// (the file was read but target.host is empty), the error points to the deploy.yaml
// field that must be set. The dir parameter is included in the not-found message
// so users know which directory was searched.
func NoHostError(fileExisted bool, dir string) error {
	if !fileExisted {
		return fmt.Errorf("no deploy.yaml found in %s and no --host flag provided", dir)
	}
	return fmt.Errorf("deploy.yaml: target.host is not set")
}

// mergeExcludes builds the final exclude list by starting with the built-in
// defaults, then appending file-level excludes, then flag-level excludes.
// Deduplication is by string equality, preserving insertion order; later
// duplicates are dropped.
// When skipEnv is true, ".env" is appended to the result (deduplicated).
func mergeExcludes(fileExcludes, flagExcludes []string, skipEnv bool) []string {
	seen := make(map[string]struct{}, len(defaultExcludes)+len(fileExcludes)+len(flagExcludes))
	result := make([]string, 0, len(defaultExcludes)+len(fileExcludes)+len(flagExcludes))

	for _, e := range defaultExcludes {
		if _, ok := seen[e]; !ok {
			seen[e] = struct{}{}
			result = append(result, e)
		}
	}
	for _, e := range fileExcludes {
		if _, ok := seen[e]; !ok {
			seen[e] = struct{}{}
			result = append(result, e)
		}
	}
	for _, e := range flagExcludes {
		if _, ok := seen[e]; !ok {
			seen[e] = struct{}{}
			result = append(result, e)
		}
	}
	if skipEnv {
		if _, ok := seen[".env"]; !ok {
			seen[".env"] = struct{}{}
			result = append(result, ".env")
		}
	}
	return result
}

// sshConfigPath returns the canonical path to the user's SSH config file.
// If os.UserHomeDir() fails (rare in practice), falls back to "~/.ssh/config"
// as a literal string so that error paths still produce readable messages.
func sshConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.ssh/config"
	}
	return filepath.Join(home, ".ssh", "config")
}

// resolveHostString resolves a raw host value (either a bare alias or a full
// ssh:// URL) to a Host struct.
//
// If raw has an ssh:// prefix, it is passed directly to ParseHost (existing
// behaviour — D-03 fallthrough).
//
// Otherwise, raw is treated as an SSH config alias (D-01): LookupHost is
// called with configPath; if no block matches, an error is returned with the
// message: alias %q not found in <configPath> (D-03, D-04). If a block is
// found, a synthetic ssh://[user@]hostname[:port] URL is constructed and
// passed to ParseHost (D-03).
//
// Per D-12: HostEntry.HostName (the real hostname) is used as Hostname, not
// the alias label, so known_hosts verification uses the correct key.
func resolveHostString(raw, configPath string) (Host, error) {
	if strings.HasPrefix(raw, "ssh://") {
		return ParseHost(raw)
	}

	// Bare alias — look up in ssh config.
	entry, found := sshconfig.LookupHost(configPath, raw)
	if !found {
		return Host{}, fmt.Errorf("alias %q not found in %s", raw, configPath)
	}

	// Build Host directly from HostEntry fields — bypass URL construction to
	// avoid re-validating the User value through isValidUnixUsername (WR-02)
	// and to correctly handle IPv6 HostName values without bracket-wrapping (CR-01).
	// Host.Hostname is used as a direct TCP dial target, not re-parsed as a URL.
	port := entry.Port
	if port == 0 {
		port = 22
	}
	return Host{
		User:     entry.User,
		Hostname: entry.HostName,
		Port:     port,
	}, nil
}

// Resolve applies four-tier precedence (flag > local deploy.yaml > global config > zero value) to
// produce a fully resolved Config.
//
// Parameters:
//   - opts: FlagOpts struct containing all CLI-flag values; zero values mean "not set"
//   - file: parsed local deploy.yaml content (zero value is safe when no file present)
//   - globalFile: parsed global config (~/.docker/cli-plugins/deploy.yaml); pass FileConfig{}
//     when the global config does not exist (a missing global file is not an error)
//   - projectName: basename of the local project directory
//   - localDir: absolute path to the local project directory (used for auto-detect)
//
// Host precedence: opts.Host > file.Target.Host > zero value (caller validates).
// Path precedence: opts.Path > file.Target.Path > "/opt/" + projectName.
// Excludes: defaultExcludes + file.Target.Exclude + opts.Excludes (deduped, order preserved).
// Force: opts.Force || file.Target.Force (flag > deploy.yaml > false).
// ComposeFile: opts.ComposeFile > file.Target.ComposeFile > auto-detect (compose.yaml, docker-compose.yml).
// Healthcheck.Interval: --healthcheck-interval > local deploy.yaml > global config > 0 (zero = skip).
// Healthcheck.Timeout: --healthcheck-timeout > local deploy.yaml > global config > 0 (zero = skip).
// Healthcheck.Retries: --healthcheck-retries (if opts.HealthcheckRetriesSet) > local deploy.yaml (> 0) > global config (> 0) > 0 (zero = skip).
//   HealthcheckRetriesSet must be true for the flag value (including 0) to override file config.
// SkipEnv: opts.SkipEnv || file.Target.SkipEnv; when true, ".env" is appended to Excludes.
// Verbose: opts.Verbose; enables detailed output lines to stderr.
//
// T-02-02: invalid host URLs (non-ssh scheme, empty hostname) are rejected
// here via ParseHost.
// T-04-01-01: ComposeFile is stored as supplied (basename); Plan 02 validates
// filepath.Base(ComposeFile) == ComposeFile before constructing remote commands.
func Resolve(opts FlagOpts, file FileConfig, globalFile FileConfig, projectName string, localDir string) (Config, error) { //nolint:gocognit // complexity from layered flag>file>default precedence for 10+ config fields — splitting by field group would require multiple return values and hurt readability
	var cfg Config

	switch {
	case opts.Host != "":
		h, err := resolveHostString(opts.Host, sshConfigPath())
		if err != nil {
			return Config{}, fmt.Errorf("--host flag: %w", err)
		}
		cfg.Host = h
	case file.Target.Host != "":
		h, err := resolveHostString(file.Target.Host, sshConfigPath())
		if err != nil {
			return Config{}, fmt.Errorf("deploy.yaml target.host: %w", err)
		}
		cfg.Host = h
	}
	// else: zero Host — caller must validate before dialing

	switch {
	case opts.Path != "":
		cfg.Path = opts.Path
	case file.Target.Path != "":
		cfg.Path = file.Target.Path
	default:
		cfg.Path = "/opt/" + projectName
	}

	cfg.SkipEnv = opts.SkipEnv || file.Target.SkipEnv
	cfg.Verbose = opts.Verbose
	cfg.Excludes = mergeExcludes(file.Target.Exclude, opts.Excludes, cfg.SkipEnv)
	cfg.Force = opts.Force || file.Target.Force

	// ComposeFile resolution: flag > deploy.yaml > auto-detect (D-07, D-08, D-09).
	switch {
	case opts.ComposeFile != "":
		cfg.ComposeFile = opts.ComposeFile
	case file.Target.ComposeFile != "":
		cfg.ComposeFile = file.Target.ComposeFile
	default:
		// Auto-detect: try compose.yaml first, then docker-compose.yml.
		for _, candidate := range []string{"compose.yaml", "docker-compose.yml"} {
			if _, err := os.Stat(filepath.Join(localDir, candidate)); err == nil {
				cfg.ComposeFile = candidate
				break
			}
		}
		if cfg.ComposeFile == "" {
			return Config{}, fmt.Errorf("no compose file found; use --compose-file to specify one")
		}
	}

	// Healthcheck resolution: four-tier precedence (flag > local file > global file > zero).
	// Duration strings are parsed via time.ParseDuration; negative durations are rejected.
	// Invalid duration strings return an error naming the source (--healthcheck-X, deploy.yaml, or global config).
	// No hardcoded defaults — absent block in all tiers produces zero HealthcheckConfig (health polling skipped per D-04).

	// 1. Validate negative retries up front across all three file/flag tiers.
	if file.Target.Healthcheck.Retries < 0 {
		return Config{}, fmt.Errorf("deploy.yaml: healthcheck.retries must be >= 0, got %d", file.Target.Healthcheck.Retries)
	}
	if globalFile.Target.Healthcheck.Retries < 0 {
		return Config{}, fmt.Errorf("global config: healthcheck.retries must be >= 0, got %d", globalFile.Target.Healthcheck.Retries)
	}
	if opts.HealthcheckRetries < 0 {
		return Config{}, fmt.Errorf("--healthcheck-retries: must be >= 0, got %d", opts.HealthcheckRetries)
	}

	// 2. Resolve Interval: flag > local file > global file > zero.
	switch {
	case opts.HealthcheckInterval != "":
		d, err := time.ParseDuration(opts.HealthcheckInterval)
		if err != nil {
			return Config{}, fmt.Errorf("--healthcheck-interval: invalid duration %q: %w", opts.HealthcheckInterval, err)
		}
		if d < 0 {
			return Config{}, fmt.Errorf("--healthcheck-interval: duration must be >= 0, got %s", opts.HealthcheckInterval)
		}
		cfg.Healthcheck.Interval = d
	case file.Target.Healthcheck.Interval != "":
		d, err := time.ParseDuration(file.Target.Healthcheck.Interval)
		if err != nil {
			return Config{}, fmt.Errorf("deploy.yaml: healthcheck.interval: invalid duration %q: %w", file.Target.Healthcheck.Interval, err)
		}
		if d < 0 {
			return Config{}, fmt.Errorf("deploy.yaml: healthcheck.interval: duration must be >= 0, got %s", file.Target.Healthcheck.Interval)
		}
		cfg.Healthcheck.Interval = d
	case globalFile.Target.Healthcheck.Interval != "":
		d, err := time.ParseDuration(globalFile.Target.Healthcheck.Interval)
		if err != nil {
			return Config{}, fmt.Errorf("global config: healthcheck.interval: invalid duration %q: %w", globalFile.Target.Healthcheck.Interval, err)
		}
		if d < 0 {
			return Config{}, fmt.Errorf("global config: healthcheck.interval: duration must be >= 0, got %s", globalFile.Target.Healthcheck.Interval)
		}
		cfg.Healthcheck.Interval = d
		// else: leave cfg.Healthcheck.Interval at zero (no hardcoded default per D-04)
	}

	// 3. Resolve Timeout: flag > local file > global file > zero.
	switch {
	case opts.HealthcheckTimeout != "":
		d, err := time.ParseDuration(opts.HealthcheckTimeout)
		if err != nil {
			return Config{}, fmt.Errorf("--healthcheck-timeout: invalid duration %q: %w", opts.HealthcheckTimeout, err)
		}
		if d < 0 {
			return Config{}, fmt.Errorf("--healthcheck-timeout: duration must be >= 0, got %s", opts.HealthcheckTimeout)
		}
		cfg.Healthcheck.Timeout = d
	case file.Target.Healthcheck.Timeout != "":
		d, err := time.ParseDuration(file.Target.Healthcheck.Timeout)
		if err != nil {
			return Config{}, fmt.Errorf("deploy.yaml: healthcheck.timeout: invalid duration %q: %w", file.Target.Healthcheck.Timeout, err)
		}
		if d < 0 {
			return Config{}, fmt.Errorf("deploy.yaml: healthcheck.timeout: duration must be >= 0, got %s", file.Target.Healthcheck.Timeout)
		}
		cfg.Healthcheck.Timeout = d
	case globalFile.Target.Healthcheck.Timeout != "":
		d, err := time.ParseDuration(globalFile.Target.Healthcheck.Timeout)
		if err != nil {
			return Config{}, fmt.Errorf("global config: healthcheck.timeout: invalid duration %q: %w", globalFile.Target.Healthcheck.Timeout, err)
		}
		if d < 0 {
			return Config{}, fmt.Errorf("global config: healthcheck.timeout: duration must be >= 0, got %s", globalFile.Target.Healthcheck.Timeout)
		}
		cfg.Healthcheck.Timeout = d
		// else: leave cfg.Healthcheck.Timeout at zero (no hardcoded default per D-04)
	}

	// 4. Resolve Retries: flag (if explicitly set) > local file (> 0) > global file (> 0) > zero.
	// HealthcheckRetriesSet distinguishes --healthcheck-retries=0 (immediate-fail) from "flag not provided".
	switch {
	case opts.HealthcheckRetriesSet:
		cfg.Healthcheck.Retries = opts.HealthcheckRetries
	case file.Target.Healthcheck.Retries > 0:
		cfg.Healthcheck.Retries = file.Target.Healthcheck.Retries
	case globalFile.Target.Healthcheck.Retries > 0:
		cfg.Healthcheck.Retries = globalFile.Target.Healthcheck.Retries
		// else: leave cfg.Healthcheck.Retries at zero (no hardcoded default per D-04)
	}

	// Validate that the remote path is absolute (WR-03).
	// ShellQuote prevents the shell from interpreting the path as a command, but
	// it does not prevent filesystem-level traversal if the path is relative
	// (e.g. "../../../etc"). Requiring a leading '/' ensures the path is anchored
	// to the filesystem root and cannot escape the intended deploy root.
	if cfg.Path != "" && !filepath.IsAbs(cfg.Path) {
		return Config{}, fmt.Errorf("remote path must be absolute (start with /), got: %q", cfg.Path)
	}

	return cfg, nil
}
