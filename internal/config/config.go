// Package config implements configuration resolution for docker-deploy.
// It supports three-tier precedence: CLI flags > deploy.yaml > built-in defaults.
package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// defaultExcludes is the built-in exclude list that is always active.
// User-supplied excludes (via deploy.yaml or --exclude flag) extend this list;
// they cannot remove these entries.
var defaultExcludes = []string{
	".git/", "node_modules/", "vendor/", "*.log", ".DS_Store", "__pycache__/",
}

// Host holds a parsed SSH host specification.
type Host struct {
	User     string
	Hostname string
	Port     int
}

// TargetConfig holds the single-target subsection of deploy.yaml.
// Future phases will add a "targets" (plural) map for named targets.
type TargetConfig struct {
	Host           string   `yaml:"host"`
	Path           string   `yaml:"path"`
	Exclude        []string `yaml:"exclude"`
	Force          bool     `yaml:"force"`
	ComposeFile    string   `yaml:"compose_file"`
	HealthTimeout  int      `yaml:"health_timeout"`
	HealthInterval int      `yaml:"health_interval"`
}

// FileConfig is the top-level structure of deploy.yaml.
// Version is used for future schema migration; Target is the single-target config.
type FileConfig struct {
	Version int          `yaml:"version"`
	Target  TargetConfig `yaml:"target"`
}

// Config is the fully resolved runtime configuration.
type Config struct {
	Host           Host
	Path           string
	DryRun         bool
	Excludes       []string // merged: defaultExcludes + file.Target.Exclude + flagExcludes, deduplicated
	Force          bool     // flag || file.Target.Force (flag > deploy.yaml > false)
	ComposeFile    string   // resolved compose filename basename (flag > deploy.yaml > auto-detect)
	HealthTimeout  int      // seconds to wait for health check; flag > deploy.yaml > 60
	HealthInterval int      // seconds between health check polls; flag > deploy.yaml > 5
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
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-') {
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

// LoadFile reads deploy.yaml from dir. If no deploy.yaml exists, it returns a
// zero FileConfig with no error. A malformed YAML file returns an error rather
// than panicking (T-02-01).
func LoadFile(dir string) (FileConfig, error) {
	path := filepath.Join(dir, "deploy.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return FileConfig{}, nil
		}
		return FileConfig{}, fmt.Errorf("reading deploy.yaml: %w", err)
	}

	var fc FileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return FileConfig{}, fmt.Errorf("parsing deploy.yaml: %w", err)
	}

	return fc, nil
}

// mergeExcludes builds the final exclude list by starting with the built-in
// defaults, then appending file-level excludes, then flag-level excludes.
// Deduplication is by string equality, preserving insertion order; later
// duplicates are dropped.
func mergeExcludes(fileExcludes, flagExcludes []string) []string {
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
	return result
}

// Resolve applies three-tier precedence (flag > deploy.yaml > default) to
// produce a fully resolved Config.
//
// Parameters:
//   - flagHost: value from the --host flag (empty string = not set)
//   - flagPath: value from the --path flag (empty string = not set)
//   - flagExcludes: values from the repeatable --exclude flag (nil is safe)
//   - flagForce: value from the --force flag
//   - flagComposeFile: value from the --compose-file flag (empty string = not set)
//   - flagHealthTimeout: seconds from a future --health-timeout flag (0 = not set)
//   - flagHealthInterval: seconds from a future --health-interval flag (0 = not set)
//   - file: parsed deploy.yaml content (zero value is safe when no file present)
//   - projectName: basename of the local project directory
//   - localDir: absolute path to the local project directory (used for auto-detect)
//
// Host precedence: flagHost > file.Target.Host > zero value (caller validates).
// Path precedence: flagPath > file.Target.Path > "/opt/" + projectName.
// Excludes: defaultExcludes + file.Target.Exclude + flagExcludes (deduped, order preserved).
// Force: flagForce || file.Target.Force (flag > deploy.yaml > false).
// ComposeFile: flagComposeFile > file.Target.ComposeFile > auto-detect (compose.yaml, docker-compose.yml).
// HealthTimeout: flagHealthTimeout > file.Target.HealthTimeout > 60.
// HealthInterval: flagHealthInterval > file.Target.HealthInterval > 5.
//
// NOTE: flagHealthTimeout and flagHealthInterval are not registered as CLI flags in
// Phase 5 (health flags via deploy.yaml only per D-03). Callers pass 0 for both.
// The parameters exist for future flag registration without a signature change.
//
// T-02-02: invalid host URLs (non-ssh scheme, empty hostname) are rejected
// here via ParseHost.
// T-04-01-01: ComposeFile is stored as supplied (basename); Plan 02 validates
// filepath.Base(ComposeFile) == ComposeFile before constructing remote commands.
// T-05-01-01: Zero and negative values for health fields are treated as "not set"
// (> 0 check gates both flag and file values), so defaults always apply.
func Resolve(flagHost, flagPath string, flagExcludes []string, flagForce bool, flagComposeFile string, flagHealthTimeout, flagHealthInterval int, file FileConfig, projectName string, localDir string) (Config, error) {
	var cfg Config

	switch {
	case flagHost != "":
		h, err := ParseHost(flagHost)
		if err != nil {
			return Config{}, fmt.Errorf("--host flag: %w", err)
		}
		cfg.Host = h
	case file.Target.Host != "":
		h, err := ParseHost(file.Target.Host)
		if err != nil {
			return Config{}, fmt.Errorf("deploy.yaml target.host: %w", err)
		}
		cfg.Host = h
	}
	// else: zero Host — caller must validate before dialing

	switch {
	case flagPath != "":
		cfg.Path = flagPath
	case file.Target.Path != "":
		cfg.Path = file.Target.Path
	default:
		cfg.Path = "/opt/" + projectName
	}

	cfg.Excludes = mergeExcludes(file.Target.Exclude, flagExcludes)
	cfg.Force = flagForce || file.Target.Force

	// ComposeFile resolution: flag > deploy.yaml > auto-detect (D-07, D-08, D-09).
	switch {
	case flagComposeFile != "":
		cfg.ComposeFile = flagComposeFile
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

	// Validate deploy.yaml health values: negative integers are rejected with an
	// explicit error so the user is not silently surprised by default values.
	if file.Target.HealthTimeout < 0 {
		return Config{}, fmt.Errorf("deploy.yaml: health_timeout must be >= 0, got %d", file.Target.HealthTimeout)
	}
	if file.Target.HealthInterval < 0 {
		return Config{}, fmt.Errorf("deploy.yaml: health_interval must be >= 0, got %d", file.Target.HealthInterval)
	}

	// HealthTimeout resolution: flag > deploy.yaml > default 60.
	// Zero is treated as "not set" for both flag and file values (T-05-01-01).
	switch {
	case flagHealthTimeout > 0:
		cfg.HealthTimeout = flagHealthTimeout
	case file.Target.HealthTimeout > 0:
		cfg.HealthTimeout = file.Target.HealthTimeout
	default:
		cfg.HealthTimeout = 60
	}

	// HealthInterval resolution: flag > deploy.yaml > default 5.
	// Zero is treated as "not set" for both flag and file values (T-05-01-01).
	switch {
	case flagHealthInterval > 0:
		cfg.HealthInterval = flagHealthInterval
	case file.Target.HealthInterval > 0:
		cfg.HealthInterval = file.Target.HealthInterval
	default:
		cfg.HealthInterval = 5
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
