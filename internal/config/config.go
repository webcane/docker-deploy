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

// Host holds a parsed SSH host specification.
type Host struct {
	User     string
	Hostname string
	Port     int
}

// TargetConfig holds the single-target subsection of deploy.yaml.
// Future phases will add a "targets" (plural) map for named targets.
type TargetConfig struct {
	Host string `yaml:"host"`
	Path string `yaml:"path"`
}

// FileConfig is the top-level structure of deploy.yaml.
// Version is used for future schema migration; Target is the single-target config.
type FileConfig struct {
	Version int          `yaml:"version"`
	Target  TargetConfig `yaml:"target"`
}

// Config is the fully resolved runtime configuration.
type Config struct {
	Host    Host
	Path    string
	DryRun  bool
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
	}

	var user string
	if u.User != nil {
		user = u.User.Username()
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

// Resolve applies three-tier precedence (flag > deploy.yaml > default) to
// produce a fully resolved Config.
//
// Host precedence: flagHost > file.Target.Host > zero value (caller validates).
// Path precedence: flagPath > file.Target.Path > "/opt/" + projectName.
//
// T-02-02: invalid host URLs (non-ssh scheme, empty hostname) are rejected
// here via ParseHost.
func Resolve(flagHost, flagPath string, file FileConfig, projectName string) (Config, error) {
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

	return cfg, nil
}
