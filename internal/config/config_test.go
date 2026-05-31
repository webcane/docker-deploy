package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- ParseHost tests ---

func TestParseHost(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Host
		wantErr bool
	}{
		{
			name:  "full URL with user and port",
			input: "ssh://alice@myhost.example.com:22",
			want:  Host{User: "alice", Hostname: "myhost.example.com", Port: 22},
		},
		{
			name:  "no port defaults to 22",
			input: "ssh://bob@myhost.example.com",
			want:  Host{User: "bob", Hostname: "myhost.example.com", Port: 22},
		},
		{
			name:  "non-standard port",
			input: "ssh://alice@myhost.example.com:2222",
			want:  Host{User: "alice", Hostname: "myhost.example.com", Port: 2222},
		},
		{
			name:  "no user",
			input: "ssh://myhost.example.com:22",
			want:  Host{User: "", Hostname: "myhost.example.com", Port: 22},
		},
		{
			name:  "empty string returns zero Host no error",
			input: "",
			want:  Host{},
		},
		{
			name:    "wrong scheme rejected",
			input:   "http://myhost.example.com:22",
			wantErr: true,
		},
		{
			name:    "not a URL returns error",
			input:   "not-a-url",
			wantErr: true,
		},
		{
			name:    "scheme only no hostname",
			input:   "ssh://",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseHost(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseHost(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseHost(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseHost(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

// --- Resolve tests ---

func TestResolveHostPrecedence(t *testing.T) {
	flagHost := "ssh://flaguser@flaghost.example.com:22"
	fileHost := "ssh://fileuser@filehost.example.com:22"

	tests := []struct {
		name         string
		flagHost     string
		fileHost     string
		wantHostname string
		wantUser     string
	}{
		{
			name:         "flag overrides file",
			flagHost:     flagHost,
			fileHost:     fileHost,
			wantHostname: "flaghost.example.com",
			wantUser:     "flaguser",
		},
		{
			name:         "file used when flag empty",
			flagHost:     "",
			fileHost:     fileHost,
			wantHostname: "filehost.example.com",
			wantUser:     "fileuser",
		},
		{
			name:         "zero Host when both empty",
			flagHost:     "",
			fileHost:     "",
			wantHostname: "",
			wantUser:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := FileConfig{
				Target: TargetConfig{Host: tt.fileHost},
			}
			cfg, err := Resolve(FlagOpts{Host: tt.flagHost, ComposeFile: "compose.yaml"}, file, FileConfig{}, "myproject", "")
			if err != nil {
				t.Fatalf("Resolve() unexpected error: %v", err)
			}
			if cfg.Host.Hostname != tt.wantHostname {
				t.Errorf("Host.Hostname = %q, want %q", cfg.Host.Hostname, tt.wantHostname)
			}
			if cfg.Host.User != tt.wantUser {
				t.Errorf("Host.User = %q, want %q", cfg.Host.User, tt.wantUser)
			}
		})
	}
}

func TestResolvePathPrecedence(t *testing.T) {
	tests := []struct {
		name        string
		flagPath    string
		filePath    string
		projectName string
		wantPath    string
	}{
		{
			name:        "flag overrides file and default",
			flagPath:    "/custom/flag",
			filePath:    "/from-file",
			projectName: "myproject",
			wantPath:    "/custom/flag",
		},
		{
			name:        "file overrides default",
			flagPath:    "",
			filePath:    "/from-file",
			projectName: "myproject",
			wantPath:    "/from-file",
		},
		{
			name:        "default when both empty",
			flagPath:    "",
			filePath:    "",
			projectName: "myproject",
			wantPath:    "/opt/myproject",
		},
		{
			name:        "default uses project name",
			flagPath:    "",
			filePath:    "",
			projectName: "myapp",
			wantPath:    "/opt/myapp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := FileConfig{
				Target: TargetConfig{Path: tt.filePath},
			}
			cfg, err := Resolve(FlagOpts{Path: tt.flagPath, ComposeFile: "compose.yaml"}, file, FileConfig{}, tt.projectName, "")
			if err != nil {
				t.Fatalf("Resolve() unexpected error: %v", err)
			}
			if cfg.Path != tt.wantPath {
				t.Errorf("Path = %q, want %q", cfg.Path, tt.wantPath)
			}
		})
	}
}

func TestResolveInvalidHostReturnsError(t *testing.T) {
	_, err := Resolve(FlagOpts{Host: "http://not-ssh.example.com", ComposeFile: "compose.yaml"}, FileConfig{}, FileConfig{}, "proj", "")
	if err == nil {
		t.Fatal("Resolve() with non-ssh scheme should return error")
	}
}

// TestResolveExcludes verifies that Config.Excludes follows the
// defaultExcludes + file.Target.Exclude + flagExcludes merge model with
// deduplication (insertion-order preserved, later duplicates dropped).
func TestResolveExcludes(t *testing.T) { //nolint:gocognit // tests exclude merge logic across many cases (flag-only, file-only, merged, dedup) — each assertion is load-bearing
	// builtInPatterns is the expected set of 16 built-in defaults (original 6 + 10 new).
	// We do not access the unexported defaultExcludes var directly;
	// instead we verify through Resolve() output.
	builtInPatterns := []string{
		".git/", "node_modules/", "vendor/", "*.log", ".DS_Store", "__pycache__/",
		".claude/", ".github/", ".planning/", ".idea/", ".vscode/",
		"*.swp", "*.swo", "coverage/", "dist/", ".terraform/",
	}

	containsAll := func(t *testing.T, got []string, want []string) {
		t.Helper()
		set := make(map[string]struct{}, len(got))
		for _, g := range got {
			set[g] = struct{}{}
		}
		for _, w := range want {
			if _, ok := set[w]; !ok {
				t.Errorf("Excludes missing %q; got %v", w, got)
			}
		}
	}

	countOf := func(got []string, target string) int {
		n := 0
		for _, g := range got {
			if g == target {
				n++
			}
		}
		return n
	}

	tests := []struct {
		name         string
		fileExclude  []string
		flagExcludes []string
		wantLen      int
		wantContains []string // must all be present
		wantDedupOf  string   // if non-empty, must appear exactly once
	}{
		{
			name:         "defaults_when_no_input",
			fileExclude:  nil,
			flagExcludes: nil,
			wantLen:      16,
			wantContains: builtInPatterns,
		},
		{
			name:         "file_extends_defaults",
			fileExclude:  []string{"*.tmp"},
			flagExcludes: nil,
			wantLen:      17,
			wantContains: append(append([]string{}, builtInPatterns...), "*.tmp"),
		},
		{
			name:         "flag_extends_file_and_defaults",
			fileExclude:  []string{"*.tmp"},
			flagExcludes: []string{"logs/"},
			wantLen:      18,
			wantContains: append(append([]string{}, builtInPatterns...), "*.tmp", "logs/"),
		},
		{
			name:         "flag_deduplicates",
			fileExclude:  nil,
			flagExcludes: []string{".git/"}, // already in defaults
			wantLen:      16,
			wantContains: builtInPatterns,
			wantDedupOf:  ".git/",
		},
		{
			name:         "file_deduplicates",
			fileExclude:  []string{".git/"}, // already in defaults
			flagExcludes: nil,
			wantLen:      16,
			wantContains: builtInPatterns,
			wantDedupOf:  ".git/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := FileConfig{
				Target: TargetConfig{Exclude: tt.fileExclude},
			}
			cfg, err := Resolve(FlagOpts{Excludes: tt.flagExcludes, ComposeFile: "compose.yaml"}, file, FileConfig{}, "proj", "")
			if err != nil {
				t.Fatalf("Resolve() unexpected error: %v", err)
			}
			if len(cfg.Excludes) != tt.wantLen {
				t.Errorf("len(Excludes) = %d, want %d; got %v", len(cfg.Excludes), tt.wantLen, cfg.Excludes)
			}
			containsAll(t, cfg.Excludes, tt.wantContains)
			if tt.wantDedupOf != "" {
				if c := countOf(cfg.Excludes, tt.wantDedupOf); c != 1 {
					t.Errorf("%q appears %d times in Excludes, want exactly 1; got %v", tt.wantDedupOf, c, cfg.Excludes)
				}
			}
		})
	}
}

// TestResolveSkipEnv verifies that SkipEnv (from flag or file) appends ".env"
// to cfg.Excludes exactly once, and that ".env" is not present when SkipEnv is false.
func TestResolveSkipEnv(t *testing.T) { //nolint:gocognit // exercises skip-env interaction with excludes across flag/file/both/neither combinations
	countOf := func(got []string, target string) int {
		n := 0
		for _, g := range got {
			if g == target {
				n++
			}
		}
		return n
	}

	t.Run("flag_skip_env_appends_dot_env", func(t *testing.T) {
		cfg, err := Resolve(FlagOpts{SkipEnv: true, ComposeFile: "compose.yaml"}, FileConfig{}, FileConfig{}, "proj", "")
		if err != nil {
			t.Fatalf("Resolve() unexpected error: %v", err)
		}
		if c := countOf(cfg.Excludes, ".env"); c != 1 {
			t.Errorf(".env appears %d times in Excludes, want exactly 1; got %v", c, cfg.Excludes)
		}
		if !cfg.SkipEnv {
			t.Errorf("cfg.SkipEnv = false, want true")
		}
	})

	t.Run("file_skip_env_appends_dot_env", func(t *testing.T) {
		file := FileConfig{Target: TargetConfig{SkipEnv: true}}
		cfg, err := Resolve(FlagOpts{ComposeFile: "compose.yaml"}, file, FileConfig{}, "proj", "")
		if err != nil {
			t.Fatalf("Resolve() unexpected error: %v", err)
		}
		if c := countOf(cfg.Excludes, ".env"); c != 1 {
			t.Errorf(".env appears %d times in Excludes, want exactly 1; got %v", c, cfg.Excludes)
		}
		if !cfg.SkipEnv {
			t.Errorf("cfg.SkipEnv = false, want true")
		}
	})

	t.Run("flag_overrides_file_skip_env_false", func(t *testing.T) {
		// Flag SkipEnv=true overrides file SkipEnv=false — .env must be present.
		file := FileConfig{Target: TargetConfig{SkipEnv: false}}
		cfg, err := Resolve(FlagOpts{SkipEnv: true, ComposeFile: "compose.yaml"}, file, FileConfig{}, "proj", "")
		if err != nil {
			t.Fatalf("Resolve() unexpected error: %v", err)
		}
		if c := countOf(cfg.Excludes, ".env"); c != 1 {
			t.Errorf(".env appears %d times in Excludes, want exactly 1; got %v", c, cfg.Excludes)
		}
		if !cfg.SkipEnv {
			t.Errorf("cfg.SkipEnv = false, want true")
		}
	})

	t.Run("skip_env_deduplicates_if_already_in_flag_excludes", func(t *testing.T) {
		// User also listed ".env" in --exclude; SkipEnv should not add it again.
		cfg, err := Resolve(FlagOpts{SkipEnv: true, Excludes: []string{".env"}, ComposeFile: "compose.yaml"}, FileConfig{}, FileConfig{}, "proj", "")
		if err != nil {
			t.Fatalf("Resolve() unexpected error: %v", err)
		}
		if c := countOf(cfg.Excludes, ".env"); c != 1 {
			t.Errorf(".env appears %d times in Excludes, want exactly 1; got %v", c, cfg.Excludes)
		}
	})

	t.Run("no_skip_env_dot_env_absent", func(t *testing.T) {
		// Neither flag nor file enables SkipEnv — .env must not appear.
		cfg, err := Resolve(FlagOpts{ComposeFile: "compose.yaml"}, FileConfig{}, FileConfig{}, "proj", "")
		if err != nil {
			t.Fatalf("Resolve() unexpected error: %v", err)
		}
		if c := countOf(cfg.Excludes, ".env"); c != 0 {
			t.Errorf(".env appears %d times in Excludes, want 0; got %v", c, cfg.Excludes)
		}
		if cfg.SkipEnv {
			t.Errorf("cfg.SkipEnv = true, want false")
		}
	})
}

// TestResolveVerbose verifies that cfg.Verbose reflects opts.Verbose.
func TestResolveVerbose(t *testing.T) {
	t.Run("verbose_true", func(t *testing.T) {
		cfg, err := Resolve(FlagOpts{Verbose: true, ComposeFile: "compose.yaml"}, FileConfig{}, FileConfig{}, "proj", "")
		if err != nil {
			t.Fatalf("Resolve() unexpected error: %v", err)
		}
		if !cfg.Verbose {
			t.Errorf("cfg.Verbose = false, want true")
		}
	})

	t.Run("verbose_false_default", func(t *testing.T) {
		cfg, err := Resolve(FlagOpts{ComposeFile: "compose.yaml"}, FileConfig{}, FileConfig{}, "proj", "")
		if err != nil {
			t.Fatalf("Resolve() unexpected error: %v", err)
		}
		if cfg.Verbose {
			t.Errorf("cfg.Verbose = true, want false")
		}
	})
}

// TestResolveExpandedDefaults verifies that all 10 new dev-tooling entries
// from defaultExcludes are present in cfg.Excludes when no user excludes are provided.
func TestResolveExpandedDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating compose.yaml: %v", err)
	}

	cfg, err := Resolve(FlagOpts{}, FileConfig{}, FileConfig{}, "proj", dir)
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}

	newDefaults := []string{
		".claude/", ".github/", ".planning/", ".idea/", ".vscode/",
		"*.swp", "*.swo", "coverage/", "dist/", ".terraform/",
	}

	excludeSet := make(map[string]struct{}, len(cfg.Excludes))
	for _, e := range cfg.Excludes {
		excludeSet[e] = struct{}{}
	}
	for _, want := range newDefaults {
		if _, ok := excludeSet[want]; !ok {
			t.Errorf("defaultExcludes missing %q; got %v", want, cfg.Excludes)
		}
	}
}

// TestResolveForce verifies that Config.Force follows flag > file > false precedence.
func TestResolveForce(t *testing.T) {
	tests := []struct {
		name      string
		fileForce bool
		flagForce bool
		wantForce bool
	}{
		{
			name:      "default_false",
			fileForce: false,
			flagForce: false,
			wantForce: false,
		},
		{
			name:      "file_sets_true",
			fileForce: true,
			flagForce: false,
			wantForce: true,
		},
		{
			name:      "flag_overrides",
			fileForce: false,
			flagForce: true,
			wantForce: true,
		},
		{
			name:      "both_true",
			fileForce: true,
			flagForce: true,
			wantForce: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := FileConfig{
				Target: TargetConfig{Force: tt.fileForce},
			}
			cfg, err := Resolve(FlagOpts{Force: tt.flagForce, ComposeFile: "compose.yaml"}, file, FileConfig{}, "proj", "")
			if err != nil {
				t.Fatalf("Resolve() unexpected error: %v", err)
			}
			if cfg.Force != tt.wantForce {
				t.Errorf("Force = %v, want %v", cfg.Force, tt.wantForce)
			}
		})
	}
}

// --- TestResolveComposeFile tests (TDD: 04-01) ---

// TestResolveComposeFile_FlagWins verifies that the --compose-file flag takes
// priority over deploy.yaml compose_file and local auto-detection.
func TestResolveComposeFile_FlagWins(t *testing.T) {
	dir := t.TempDir()
	// Create compose.yaml in local dir so auto-detect would find it.
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating compose.yaml: %v", err)
	}
	file := FileConfig{
		Target: TargetConfig{ComposeFile: "compose.yaml"},
	}
	cfg, err := Resolve(FlagOpts{ComposeFile: "docker-compose.yml"}, file, FileConfig{}, "proj", dir)
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}
	if cfg.ComposeFile != "docker-compose.yml" {
		t.Errorf("ComposeFile = %q, want %q", cfg.ComposeFile, "docker-compose.yml")
	}
}

// TestResolveComposeFile_FileWins verifies that deploy.yaml compose_file wins
// over auto-detection when the flag is empty.
func TestResolveComposeFile_FileWins(t *testing.T) {
	dir := t.TempDir()
	// Create compose.yaml in local dir so auto-detect would find it.
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating compose.yaml: %v", err)
	}
	file := FileConfig{
		Target: TargetConfig{ComposeFile: "mycompose.yaml"},
	}
	cfg, err := Resolve(FlagOpts{}, file, FileConfig{}, "proj", dir)
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}
	if cfg.ComposeFile != "mycompose.yaml" {
		t.Errorf("ComposeFile = %q, want %q", cfg.ComposeFile, "mycompose.yaml")
	}
}

// TestResolveComposeFile_AutoDetectComposeYaml verifies auto-detection finds
// compose.yaml when no flag or deploy.yaml compose_file is set.
func TestResolveComposeFile_AutoDetectComposeYaml(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating compose.yaml: %v", err)
	}
	cfg, err := Resolve(FlagOpts{}, FileConfig{}, FileConfig{}, "proj", dir)
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}
	if cfg.ComposeFile != "compose.yaml" {
		t.Errorf("ComposeFile = %q, want %q", cfg.ComposeFile, "compose.yaml")
	}
}

// TestResolveComposeFile_AutoDetectDockerComposeYml verifies auto-detection
// falls back to docker-compose.yml when compose.yaml is absent.
func TestResolveComposeFile_AutoDetectDockerComposeYml(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating docker-compose.yml: %v", err)
	}
	cfg, err := Resolve(FlagOpts{}, FileConfig{}, FileConfig{}, "proj", dir)
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}
	if cfg.ComposeFile != "docker-compose.yml" {
		t.Errorf("ComposeFile = %q, want %q", cfg.ComposeFile, "docker-compose.yml")
	}
}

// TestResolveComposeFile_NoFileFound verifies that Resolve() returns an error
// containing "no compose file found" when no compose file is available by any
// resolution method.
func TestResolveComposeFile_NoFileFound(t *testing.T) {
	dir := t.TempDir()
	_, err := Resolve(FlagOpts{}, FileConfig{}, FileConfig{}, "proj", dir)
	if err == nil {
		t.Fatal("Resolve() expected error when no compose file found, got nil")
	}
	if !strings.Contains(err.Error(), "no compose file found") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "no compose file found")
	}
}

// TestResolveComposeFile_PreservesExistingFields verifies that adding the
// ComposeFile parameter does not break Host, Path, Excludes, and Force
// resolution.
func TestResolveComposeFile_PreservesExistingFields(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating compose.yaml: %v", err)
	}
	file := FileConfig{
		Target: TargetConfig{
			Host:  "ssh://user@host.example.com:22",
			Path:  "/opt/myapp",
			Force: true,
		},
	}
	cfg, err := Resolve(FlagOpts{Excludes: []string{"*.tmp"}}, file, FileConfig{}, "proj", dir)
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}
	if cfg.Host.Hostname != "host.example.com" {
		t.Errorf("Host.Hostname = %q, want %q", cfg.Host.Hostname, "host.example.com")
	}
	if cfg.Path != "/opt/myapp" {
		t.Errorf("Path = %q, want %q", cfg.Path, "/opt/myapp")
	}
	if cfg.Force != true {
		t.Errorf("Force = %v, want true", cfg.Force)
	}
	found := false
	for _, e := range cfg.Excludes {
		if e == "*.tmp" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Excludes does not contain *.tmp; got %v", cfg.Excludes)
	}
}

// --- TestResolveHealthcheckConfig tests (TDD: 15-01) ---

// TestResolveHealthcheck_AbsentBlockZeroValue verifies that when no healthcheck
// block is present in any source tier, the result is a zero-value HealthcheckConfig.
// Per D-04: no hardcoded defaults; absent block means health polling is skipped.
func TestResolveHealthcheck_AbsentBlockZeroValue(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating compose.yaml: %v", err)
	}

	cfg, err := Resolve(FlagOpts{}, FileConfig{}, FileConfig{}, "proj", dir)
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}
	if cfg.Healthcheck.Interval != 0 {
		t.Errorf("Healthcheck.Interval = %v, want 0 (no hardcoded default)", cfg.Healthcheck.Interval)
	}
	if cfg.Healthcheck.Timeout != 0 {
		t.Errorf("Healthcheck.Timeout = %v, want 0 (no hardcoded default)", cfg.Healthcheck.Timeout)
	}
	if cfg.Healthcheck.Retries != 0 {
		t.Errorf("Healthcheck.Retries = %d, want 0 (no hardcoded default)", cfg.Healthcheck.Retries)
	}
}

// TestResolveHealthcheck_ValidDurationStrings verifies that valid duration strings
// are parsed correctly to time.Duration values.
func TestResolveHealthcheck_ValidDurationStrings(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating compose.yaml: %v", err)
	}

	tests := []struct {
		name         string
		fileInterval string
		fileTimeout  string
		fileRetries  int
		wantInterval time.Duration
		wantTimeout  time.Duration
		wantRetries  int
	}{
		{
			name:         "10s interval and 30s timeout",
			fileInterval: "10s",
			fileTimeout:  "30s",
			fileRetries:  3,
			wantInterval: 10 * time.Second,
			wantTimeout:  30 * time.Second,
			wantRetries:  3,
		},
		{
			name:         "1m30s timeout parsed correctly",
			fileInterval: "10s",
			fileTimeout:  "1m30s",
			fileRetries:  5,
			wantInterval: 10 * time.Second,
			wantTimeout:  90 * time.Second,
			wantRetries:  5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := FileConfig{
				Target: TargetConfig{
					Healthcheck: healthcheckYAML{
						Interval: tt.fileInterval,
						Timeout:  tt.fileTimeout,
						Retries:  tt.fileRetries,
					},
				},
			}
			cfg, err := Resolve(FlagOpts{ComposeFile: "compose.yaml"}, file, FileConfig{}, "proj", dir)
			if err != nil {
				t.Fatalf("Resolve() unexpected error: %v", err)
			}
			if cfg.Healthcheck.Interval != tt.wantInterval {
				t.Errorf("Healthcheck.Interval = %v, want %v", cfg.Healthcheck.Interval, tt.wantInterval)
			}
			if cfg.Healthcheck.Timeout != tt.wantTimeout {
				t.Errorf("Healthcheck.Timeout = %v, want %v", cfg.Healthcheck.Timeout, tt.wantTimeout)
			}
			if cfg.Healthcheck.Retries != tt.wantRetries {
				t.Errorf("Healthcheck.Retries = %d, want %d", cfg.Healthcheck.Retries, tt.wantRetries)
			}
		})
	}
}

// TestResolveHealthcheck_FlagOverridesLocalFile verifies that flag values override
// local deploy.yaml values (first tier wins over second tier).
func TestResolveHealthcheck_FlagOverridesLocalFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating compose.yaml: %v", err)
	}

	file := FileConfig{
		Target: TargetConfig{
			Healthcheck: healthcheckYAML{
				Interval: "30s",
				Timeout:  "60s",
				Retries:  10,
			},
		},
	}
	opts := FlagOpts{
		ComposeFile:         "compose.yaml",
		HealthcheckInterval: "5s",
		HealthcheckTimeout:  "10s",
		HealthcheckRetries:  2,
	}
	cfg, err := Resolve(opts, file, FileConfig{}, "proj", dir)
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}
	if cfg.Healthcheck.Interval != 5*time.Second {
		t.Errorf("Healthcheck.Interval = %v, want 5s (flag wins over local file)", cfg.Healthcheck.Interval)
	}
	if cfg.Healthcheck.Timeout != 10*time.Second {
		t.Errorf("Healthcheck.Timeout = %v, want 10s (flag wins over local file)", cfg.Healthcheck.Timeout)
	}
	if cfg.Healthcheck.Retries != 2 {
		t.Errorf("Healthcheck.Retries = %d, want 2 (flag wins over local file)", cfg.Healthcheck.Retries)
	}
}

// TestResolveHealthcheck_LocalFileOverridesGlobalFile verifies that local deploy.yaml
// values override global config values (second tier wins over third tier).
func TestResolveHealthcheck_LocalFileOverridesGlobalFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating compose.yaml: %v", err)
	}

	file := FileConfig{
		Target: TargetConfig{
			Healthcheck: healthcheckYAML{
				Interval: "30s",
				Timeout:  "60s",
				Retries:  5,
			},
		},
	}
	globalFile := FileConfig{
		Target: TargetConfig{
			Healthcheck: healthcheckYAML{
				Interval: "10s",
				Timeout:  "30s",
				Retries:  3,
			},
		},
	}
	cfg, err := Resolve(FlagOpts{ComposeFile: "compose.yaml"}, file, globalFile, "proj", dir)
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}
	if cfg.Healthcheck.Interval != 30*time.Second {
		t.Errorf("Healthcheck.Interval = %v, want 30s (local file wins over global)", cfg.Healthcheck.Interval)
	}
	if cfg.Healthcheck.Timeout != 60*time.Second {
		t.Errorf("Healthcheck.Timeout = %v, want 60s (local file wins over global)", cfg.Healthcheck.Timeout)
	}
	if cfg.Healthcheck.Retries != 5 {
		t.Errorf("Healthcheck.Retries = %d, want 5 (local file wins over global)", cfg.Healthcheck.Retries)
	}
}

// TestResolveHealthcheck_GlobalFileUsedWhenFlagAndLocalEmpty verifies that global
// config values are used when flag and local deploy.yaml are both absent.
func TestResolveHealthcheck_GlobalFileUsedWhenFlagAndLocalEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating compose.yaml: %v", err)
	}

	globalFile := FileConfig{
		Target: TargetConfig{
			Healthcheck: healthcheckYAML{
				Interval: "10s",
				Timeout:  "30s",
				Retries:  3,
			},
		},
	}
	cfg, err := Resolve(FlagOpts{ComposeFile: "compose.yaml"}, FileConfig{}, globalFile, "proj", dir)
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}
	if cfg.Healthcheck.Interval != 10*time.Second {
		t.Errorf("Healthcheck.Interval = %v, want 10s (from global config)", cfg.Healthcheck.Interval)
	}
	if cfg.Healthcheck.Timeout != 30*time.Second {
		t.Errorf("Healthcheck.Timeout = %v, want 30s (from global config)", cfg.Healthcheck.Timeout)
	}
	if cfg.Healthcheck.Retries != 3 {
		t.Errorf("Healthcheck.Retries = %d, want 3 (from global config)", cfg.Healthcheck.Retries)
	}
}

// TestResolveHealthcheck_InvalidDurationInFlag verifies that an invalid duration
// string in a flag returns an error naming "--healthcheck-interval" and the bad value.
func TestResolveHealthcheck_InvalidDurationInFlag(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating compose.yaml: %v", err)
	}

	_, err := Resolve(FlagOpts{ComposeFile: "compose.yaml", HealthcheckInterval: "notaduration"}, FileConfig{}, FileConfig{}, "proj", dir)
	if err == nil {
		t.Fatal("Resolve() expected error for invalid duration in flag, got nil")
	}
	if !strings.Contains(err.Error(), "--healthcheck-interval") {
		t.Errorf("error = %q, want it to mention --healthcheck-interval", err.Error())
	}
	if !strings.Contains(err.Error(), "notaduration") {
		t.Errorf("error = %q, want it to mention the bad value", err.Error())
	}
}

// TestResolveHealthcheck_InvalidDurationInLocalFile verifies that an invalid duration
// string in local deploy.yaml returns an error naming "deploy.yaml: healthcheck.interval".
func TestResolveHealthcheck_InvalidDurationInLocalFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating compose.yaml: %v", err)
	}

	file := FileConfig{
		Target: TargetConfig{
			Healthcheck: healthcheckYAML{Interval: "badvalue"},
		},
	}
	_, err := Resolve(FlagOpts{ComposeFile: "compose.yaml"}, file, FileConfig{}, "proj", dir)
	if err == nil {
		t.Fatal("Resolve() expected error for invalid duration in local file, got nil")
	}
	if !strings.Contains(err.Error(), "deploy.yaml: healthcheck.interval") {
		t.Errorf("error = %q, want it to mention deploy.yaml: healthcheck.interval", err.Error())
	}
}

// TestResolveHealthcheck_InvalidDurationInGlobalFile verifies that an invalid duration
// string in global config returns an error naming "global config: healthcheck.interval".
func TestResolveHealthcheck_InvalidDurationInGlobalFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating compose.yaml: %v", err)
	}

	globalFile := FileConfig{
		Target: TargetConfig{
			Healthcheck: healthcheckYAML{Interval: "badvalue"},
		},
	}
	_, err := Resolve(FlagOpts{ComposeFile: "compose.yaml"}, FileConfig{}, globalFile, "proj", dir)
	if err == nil {
		t.Fatal("Resolve() expected error for invalid duration in global config, got nil")
	}
	if !strings.Contains(err.Error(), "global config: healthcheck.interval") {
		t.Errorf("error = %q, want it to mention global config: healthcheck.interval", err.Error())
	}
}

// TestResolveHealthcheck_NegativeDurationInFlag verifies that a negative parsed
// duration from a flag (e.g. --healthcheck-interval=-5s) is rejected.
func TestResolveHealthcheck_NegativeDurationInFlag(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating compose.yaml: %v", err)
	}

	_, err := Resolve(FlagOpts{ComposeFile: "compose.yaml", HealthcheckInterval: "-5s"}, FileConfig{}, FileConfig{}, "proj", dir)
	if err == nil {
		t.Fatal("Resolve() expected error for negative duration in flag, got nil")
	}
	if !strings.Contains(err.Error(), "--healthcheck-interval") {
		t.Errorf("error = %q, want it to mention --healthcheck-interval", err.Error())
	}
	if !strings.Contains(err.Error(), "must be >= 0") {
		t.Errorf("error = %q, want it to mention 'must be >= 0'", err.Error())
	}
}

// TestResolveHealthcheck_NegativeDurationInLocalFile verifies that a negative parsed
// duration in local deploy.yaml (e.g. interval: -5s) is rejected.
func TestResolveHealthcheck_NegativeDurationInLocalFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating compose.yaml: %v", err)
	}

	file := FileConfig{
		Target: TargetConfig{
			Healthcheck: healthcheckYAML{Interval: "-5s"},
		},
	}
	_, err := Resolve(FlagOpts{ComposeFile: "compose.yaml"}, file, FileConfig{}, "proj", dir)
	if err == nil {
		t.Fatal("Resolve() expected error for negative duration in local file, got nil")
	}
	if !strings.Contains(err.Error(), "deploy.yaml: healthcheck.interval") {
		t.Errorf("error = %q, want it to mention deploy.yaml: healthcheck.interval", err.Error())
	}
	if !strings.Contains(err.Error(), "must be >= 0") {
		t.Errorf("error = %q, want it to mention 'must be >= 0'", err.Error())
	}
}

// TestResolveHealthcheck_NegativeDurationInGlobalFile verifies that a negative parsed
// duration in global config (e.g. interval: -5s) is rejected.
func TestResolveHealthcheck_NegativeDurationInGlobalFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating compose.yaml: %v", err)
	}

	globalFile := FileConfig{
		Target: TargetConfig{
			Healthcheck: healthcheckYAML{Interval: "-5s"},
		},
	}
	_, err := Resolve(FlagOpts{ComposeFile: "compose.yaml"}, FileConfig{}, globalFile, "proj", dir)
	if err == nil {
		t.Fatal("Resolve() expected error for negative duration in global config, got nil")
	}
	if !strings.Contains(err.Error(), "global config: healthcheck.interval") {
		t.Errorf("error = %q, want it to mention global config: healthcheck.interval", err.Error())
	}
	if !strings.Contains(err.Error(), "must be >= 0") {
		t.Errorf("error = %q, want it to mention 'must be >= 0'", err.Error())
	}
}

// TestResolveHealthcheck_NegativeTimeoutInFlag verifies that a negative parsed
// timeout from a flag (e.g. --healthcheck-timeout=-1m) is rejected.
func TestResolveHealthcheck_NegativeTimeoutInFlag(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating compose.yaml: %v", err)
	}

	_, err := Resolve(FlagOpts{ComposeFile: "compose.yaml", HealthcheckTimeout: "-1m"}, FileConfig{}, FileConfig{}, "proj", dir)
	if err == nil {
		t.Fatal("Resolve() expected error for negative timeout in flag, got nil")
	}
	if !strings.Contains(err.Error(), "--healthcheck-timeout") {
		t.Errorf("error = %q, want it to mention --healthcheck-timeout", err.Error())
	}
	if !strings.Contains(err.Error(), "must be >= 0") {
		t.Errorf("error = %q, want it to mention 'must be >= 0'", err.Error())
	}
}

// TestResolveHealthcheck_NegativeTimeoutInLocalFile verifies that a negative parsed
// timeout in local deploy.yaml (e.g. timeout: -1m) is rejected.
func TestResolveHealthcheck_NegativeTimeoutInLocalFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating compose.yaml: %v", err)
	}

	file := FileConfig{
		Target: TargetConfig{
			Healthcheck: healthcheckYAML{Timeout: "-1m"},
		},
	}
	_, err := Resolve(FlagOpts{ComposeFile: "compose.yaml"}, file, FileConfig{}, "proj", dir)
	if err == nil {
		t.Fatal("Resolve() expected error for negative timeout in local file, got nil")
	}
	if !strings.Contains(err.Error(), "deploy.yaml: healthcheck.timeout") {
		t.Errorf("error = %q, want it to mention deploy.yaml: healthcheck.timeout", err.Error())
	}
	if !strings.Contains(err.Error(), "must be >= 0") {
		t.Errorf("error = %q, want it to mention 'must be >= 0'", err.Error())
	}
}

// TestResolveHealthcheck_NegativeTimeoutInGlobalFile verifies that a negative parsed
// timeout in global config (e.g. timeout: -1m) is rejected.
func TestResolveHealthcheck_NegativeTimeoutInGlobalFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating compose.yaml: %v", err)
	}

	globalFile := FileConfig{
		Target: TargetConfig{
			Healthcheck: healthcheckYAML{Timeout: "-1m"},
		},
	}
	_, err := Resolve(FlagOpts{ComposeFile: "compose.yaml"}, FileConfig{}, globalFile, "proj", dir)
	if err == nil {
		t.Fatal("Resolve() expected error for negative timeout in global config, got nil")
	}
	if !strings.Contains(err.Error(), "global config: healthcheck.timeout") {
		t.Errorf("error = %q, want it to mention global config: healthcheck.timeout", err.Error())
	}
	if !strings.Contains(err.Error(), "must be >= 0") {
		t.Errorf("error = %q, want it to mention 'must be >= 0'", err.Error())
	}
}

// TestResolveHealthcheck_NegativeRetriesInFlag verifies that negative retries from
// a flag returns an error.
func TestResolveHealthcheck_NegativeRetriesInFlag(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating compose.yaml: %v", err)
	}

	_, err := Resolve(FlagOpts{ComposeFile: "compose.yaml", HealthcheckRetries: -1}, FileConfig{}, FileConfig{}, "proj", dir)
	if err == nil {
		t.Fatal("Resolve() expected error for negative retries in flag, got nil")
	}
	if !strings.Contains(err.Error(), "--healthcheck-retries") {
		t.Errorf("error = %q, want it to mention --healthcheck-retries", err.Error())
	}
	if !strings.Contains(err.Error(), ">= 0") {
		t.Errorf("error = %q, want it to mention '>= 0'", err.Error())
	}
}

// TestResolveHealthcheck_NegativeRetriesInLocalFile verifies that negative retries
// in local deploy.yaml returns an error.
func TestResolveHealthcheck_NegativeRetriesInLocalFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating compose.yaml: %v", err)
	}

	file := FileConfig{
		Target: TargetConfig{
			Healthcheck: healthcheckYAML{Retries: -1},
		},
	}
	_, err := Resolve(FlagOpts{ComposeFile: "compose.yaml"}, file, FileConfig{}, "proj", dir)
	if err == nil {
		t.Fatal("Resolve() expected error for negative retries in local file, got nil")
	}
	if !strings.Contains(err.Error(), "deploy.yaml: healthcheck.retries") {
		t.Errorf("error = %q, want it to mention deploy.yaml: healthcheck.retries", err.Error())
	}
	if !strings.Contains(err.Error(), ">= 0") {
		t.Errorf("error = %q, want it to mention '>= 0'", err.Error())
	}
}

// TestResolveHealthcheck_NegativeRetriesInGlobalFile verifies that negative retries
// in global config returns an error.
func TestResolveHealthcheck_NegativeRetriesInGlobalFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating compose.yaml: %v", err)
	}

	globalFile := FileConfig{
		Target: TargetConfig{
			Healthcheck: healthcheckYAML{Retries: -1},
		},
	}
	_, err := Resolve(FlagOpts{ComposeFile: "compose.yaml"}, FileConfig{}, globalFile, "proj", dir)
	if err == nil {
		t.Fatal("Resolve() expected error for negative retries in global config, got nil")
	}
	if !strings.Contains(err.Error(), "global config: healthcheck.retries") {
		t.Errorf("error = %q, want it to mention global config: healthcheck.retries", err.Error())
	}
	if !strings.Contains(err.Error(), ">= 0") {
		t.Errorf("error = %q, want it to mention '>= 0'", err.Error())
	}
}

// --- LoadFile tests ---

func TestLoadFile(t *testing.T) { //nolint:gocognit // comprehensive load-file test covering missing, malformed, and all supported YAML fields
	t.Run("no deploy.yaml returns zero config no error", func(t *testing.T) {
		dir := t.TempDir()
		fc, _, err := LoadFile(dir)
		if err != nil {
			t.Fatalf("LoadFile() unexpected error: %v", err)
		}
		if fc.Version != 0 || fc.Target.Host != "" || fc.Target.Path != "" {
			t.Errorf("expected zero FileConfig, got %+v", fc)
		}
	})

	t.Run("valid deploy.yaml is parsed", func(t *testing.T) {
		dir := t.TempDir()
		content := `version: 1
target:
  host: ssh://user@myhost.com:22
  path: /opt/myapp
`
		if err := os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte(content), 0600); err != nil {
			t.Fatalf("writing deploy.yaml: %v", err)
		}
		fc, _, err := LoadFile(dir)
		if err != nil {
			t.Fatalf("LoadFile() unexpected error: %v", err)
		}
		if fc.Version != 1 {
			t.Errorf("Version = %d, want 1", fc.Version)
		}
		if fc.Target.Host != "ssh://user@myhost.com:22" {
			t.Errorf("Target.Host = %q, want ssh://user@myhost.com:22", fc.Target.Host)
		}
		if fc.Target.Path != "/opt/myapp" {
			t.Errorf("Target.Path = %q, want /opt/myapp", fc.Target.Path)
		}
	})

	t.Run("malformed YAML returns error", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte(":\t:bad yaml:::"), 0600); err != nil {
			t.Fatalf("writing deploy.yaml: %v", err)
		}
		_, _, err := LoadFile(dir)
		if err == nil {
			t.Fatal("LoadFile() with malformed YAML should return error")
		}
	})

	t.Run("deploy.yaml schema uses target subsection not flat keys", func(t *testing.T) {
		dir := t.TempDir()
		// Flat keys (old-style) should NOT populate Target fields
		content := `version: 1
host: ssh://user@myhost.com:22
path: /opt/myapp
`
		if err := os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte(content), 0600); err != nil {
			t.Fatalf("writing deploy.yaml: %v", err)
		}
		fc, _, err := LoadFile(dir)
		if err != nil {
			t.Fatalf("LoadFile() unexpected error: %v", err)
		}
		// Flat keys should be ignored — target subsection is the schema
		if fc.Target.Host != "" {
			t.Errorf("flat 'host' key should not populate Target.Host, got %q", fc.Target.Host)
		}
	})

	t.Run("target.healthcheck block is parsed correctly", func(t *testing.T) {
		dir := t.TempDir()
		content := `version: 1
target:
  host: ssh://user@myhost.com:22
  path: /opt/myapp
  healthcheck:
    interval: 10s
    timeout: 30s
    retries: 5
`
		if err := os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte(content), 0600); err != nil {
			t.Fatalf("writing deploy.yaml: %v", err)
		}
		fc, _, err := LoadFile(dir)
		if err != nil {
			t.Fatalf("LoadFile() unexpected error: %v", err)
		}
		if fc.Target.Healthcheck.Interval != "10s" {
			t.Errorf("Target.Healthcheck.Interval = %q, want %q", fc.Target.Healthcheck.Interval, "10s")
		}
		if fc.Target.Healthcheck.Timeout != "30s" {
			t.Errorf("Target.Healthcheck.Timeout = %q, want %q", fc.Target.Healthcheck.Timeout, "30s")
		}
		if fc.Target.Healthcheck.Retries != 5 {
			t.Errorf("Target.Healthcheck.Retries = %d, want 5", fc.Target.Healthcheck.Retries)
		}
	})
}

// --- NoHostError / LoadFile bool tests (14-02) ---

// TestLoadFile_Absent verifies that LoadFile returns fileExisted=false and no error
// when no deploy.yaml is present in the directory.
func TestLoadFile_Absent(t *testing.T) {
	dir := t.TempDir()
	_, fileExisted, err := LoadFile(dir)
	if err != nil {
		t.Fatalf("LoadFile() unexpected error: %v", err)
	}
	if fileExisted {
		t.Errorf("fileExisted = true, want false for absent deploy.yaml")
	}
}

// TestLoadFile_Present verifies that LoadFile returns fileExisted=true, no error,
// and a populated FileConfig when a valid deploy.yaml is present.
func TestLoadFile_Present(t *testing.T) {
	dir := t.TempDir()
	content := `version: 1
target:
  host: ssh://user@myhost.com:22
  path: /opt/myapp
`
	if err := os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte(content), 0600); err != nil {
		t.Fatalf("writing deploy.yaml: %v", err)
	}
	fc, fileExisted, err := LoadFile(dir)
	if err != nil {
		t.Fatalf("LoadFile() unexpected error: %v", err)
	}
	if !fileExisted {
		t.Errorf("fileExisted = false, want true when deploy.yaml is present")
	}
	if fc.Target.Host != "ssh://user@myhost.com:22" {
		t.Errorf("Target.Host = %q, want %q", fc.Target.Host, "ssh://user@myhost.com:22")
	}
}

// TestLoadFile_Malformed verifies that LoadFile returns fileExisted=true and an error
// when the deploy.yaml file exists but contains invalid YAML.
func TestLoadFile_Malformed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte("not: valid: yaml: ["), 0600); err != nil {
		t.Fatalf("writing deploy.yaml: %v", err)
	}
	_, fileExisted, err := LoadFile(dir)
	if err == nil {
		t.Fatal("LoadFile() expected error for malformed YAML, got nil")
	}
	if !fileExisted {
		t.Errorf("fileExisted = false, want true when deploy.yaml exists but is malformed")
	}
}

// TestNoHostError_FileAbsent verifies that NoHostError with fileExisted=false returns
// the "no deploy.yaml found in <dir> and no --host flag provided" message.
func TestNoHostError_FileAbsent(t *testing.T) {
	err := NoHostError(false, "/some/dir")
	if err == nil {
		t.Fatal("NoHostError() returned nil, want non-nil error")
	}
	want := "no deploy.yaml found in /some/dir and no --host flag provided"
	if err.Error() != want {
		t.Errorf("NoHostError(false, %q) = %q, want %q", "/some/dir", err.Error(), want)
	}
}

// TestNoHostError_FilePresent verifies that NoHostError with fileExisted=true returns
// the "deploy.yaml: target.host is not set" message.
func TestNoHostError_FilePresent(t *testing.T) {
	err := NoHostError(true, "/any")
	if err == nil {
		t.Fatal("NoHostError() returned nil, want non-nil error")
	}
	want := "deploy.yaml: target.host is not set"
	if err.Error() != want {
		t.Errorf("NoHostError(true, %q) = %q, want %q", "/any", err.Error(), want)
	}
}

// --- Alias resolution tests (14-01) ---

// TestResolve_AliasResolved verifies that resolveHostString resolves a bare
// alias (no ssh:// prefix) via LookupHost to the real HostName/User/Port.
func TestResolve_AliasResolved(t *testing.T) {
	cfg := `Host minipc
  HostName 192.168.1.50
  User alice
  Port 2222
`
	dir := t.TempDir()
	tmpCfg := filepath.Join(dir, "ssh_config")
	if err := os.WriteFile(tmpCfg, []byte(cfg), 0600); err != nil {
		t.Fatalf("writing ssh config: %v", err)
	}

	h, err := resolveHostString("minipc", tmpCfg)
	if err != nil {
		t.Fatalf("resolveHostString() unexpected error: %v", err)
	}
	if h.Hostname != "192.168.1.50" {
		t.Errorf("Hostname = %q, want %q", h.Hostname, "192.168.1.50")
	}
	if h.User != "alice" {
		t.Errorf("User = %q, want %q", h.User, "alice")
	}
	if h.Port != 2222 {
		t.Errorf("Port = %d, want 2222", h.Port)
	}
}

// TestResolve_AliasNotFound verifies that resolveHostString returns an error
// containing "alias %q not found in" when the alias has no match.
func TestResolve_AliasNotFound(t *testing.T) {
	dir := t.TempDir()
	tmpCfg := filepath.Join(dir, "ssh_config")
	// Empty config file — no Host blocks.
	if err := os.WriteFile(tmpCfg, []byte(""), 0600); err != nil {
		t.Fatalf("writing ssh config: %v", err)
	}

	_, err := resolveHostString("ghost", tmpCfg)
	if err == nil {
		t.Fatal("resolveHostString() expected error for unknown alias, got nil")
	}
	if !strings.Contains(err.Error(), `alias "ghost" not found`) {
		t.Errorf("error = %q, want it to contain 'alias \"ghost\" not found'", err.Error())
	}
}

// TestResolve_SSHURLUnchanged verifies that an ssh:// URL is passed directly
// to ParseHost without calling LookupHost (existing behaviour unchanged).
func TestResolve_SSHURLUnchanged(t *testing.T) {
	h, err := resolveHostString("ssh://bob@myhost.example.com:22", "/nonexistent/config")
	if err != nil {
		t.Fatalf("resolveHostString() unexpected error: %v", err)
	}
	if h.User != "bob" {
		t.Errorf("User = %q, want %q", h.User, "bob")
	}
	if h.Hostname != "myhost.example.com" {
		t.Errorf("Hostname = %q, want %q", h.Hostname, "myhost.example.com")
	}
	if h.Port != 22 {
		t.Errorf("Port = %d, want 22", h.Port)
	}
}

// TestLoadFile_CwdRelative verifies that LoadFile constructs the config file path
// relative to the provided cwd argument, not from os.Getwd() or a hardcoded path.
// This matters for subcommand callers (e.g. validate) that pass an explicit cwd.
func TestLoadFile_CwdRelative(t *testing.T) {
	t.Run("reads deploy.yaml from the provided directory", func(t *testing.T) {
		dir := t.TempDir()
		content := `version: 1
target:
  host: ssh://user@host.example.com:22
  path: /opt/testapp
`
		if err := os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte(content), 0600); err != nil {
			t.Fatalf("writing deploy.yaml: %v", err)
		}
		fc, _, err := LoadFile(dir)
		if err != nil {
			t.Fatalf("LoadFile(%q) unexpected error: %v", dir, err)
		}
		if fc.Version == 0 && fc.Target.Host == "" {
			t.Errorf("LoadFile(%q) returned zero FileConfig; expected to read deploy.yaml from that directory", dir)
		}
		if fc.Target.Host != "ssh://user@host.example.com:22" {
			t.Errorf("Target.Host = %q, want %q", fc.Target.Host, "ssh://user@host.example.com:22")
		}
	})

	t.Run("returns zero FileConfig and nil error when directory has no deploy.yaml", func(t *testing.T) {
		emptyDir := t.TempDir()
		fc, _, err := LoadFile(emptyDir)
		if err != nil {
			t.Fatalf("LoadFile(%q) unexpected error for empty dir: %v", emptyDir, err)
		}
		if fc.Version != 0 || fc.Target.Host != "" || fc.Target.Path != "" {
			t.Errorf("expected zero FileConfig for empty dir, got %+v", fc)
		}
	})
}

// TestLoadFile_UnknownHealthcheckKey verifies that a typo in a healthcheck YAML key
// (e.g. "retrise" instead of "retries") is rejected with a parse error naming the
// offending key. This prevents silent misconfiguration where the wrong field is silently
// ignored and the user gets retries=0 with no feedback.
func TestLoadFile_UnknownHealthcheckKey(t *testing.T) {
	dir := t.TempDir()
	content := `target:
  host: "ssh://user@host"
  healthcheck:
    retrise: 3
`
	if err := os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte(content), 0600); err != nil {
		t.Fatalf("writing deploy.yaml: %v", err)
	}
	_, _, err := LoadFile(dir)
	if err == nil {
		t.Fatal("LoadFile() expected error for unknown healthcheck key 'retrise', got nil")
	}
	if !strings.Contains(err.Error(), "retrise") {
		t.Fatalf("LoadFile() error %q does not mention 'retrise'", err.Error())
	}
}

// TestLoadFile_UnknownTopLevelKey verifies that an unrecognised top-level key in
// deploy.yaml is rejected with a parse error naming the offending key.
func TestLoadFile_UnknownTopLevelKey(t *testing.T) {
	dir := t.TempDir()
	content := `boguskey: true
target:
  host: "ssh://user@host"
`
	if err := os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte(content), 0600); err != nil {
		t.Fatalf("writing deploy.yaml: %v", err)
	}
	_, _, err := LoadFile(dir)
	if err == nil {
		t.Fatal("LoadFile() expected error for unknown top-level key 'boguskey', got nil")
	}
	if !strings.Contains(err.Error(), "boguskey") {
		t.Fatalf("LoadFile() error %q does not mention 'boguskey'", err.Error())
	}
}

// TestLoadFile_ValidHealthcheckParsed is a regression test verifying that valid
// healthcheck fields are still parsed correctly after switching to strict mode.
func TestLoadFile_ValidHealthcheckParsed(t *testing.T) {
	dir := t.TempDir()
	content := `target:
  host: "ssh://user@host"
  healthcheck:
    interval: "30s"
    timeout: "10s"
    retries: 3
`
	if err := os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte(content), 0600); err != nil {
		t.Fatalf("writing deploy.yaml: %v", err)
	}
	fc, found, err := LoadFile(dir)
	if err != nil {
		t.Fatalf("LoadFile() unexpected error: %v", err)
	}
	if !found {
		t.Fatal("LoadFile() expected found=true for existing deploy.yaml, got false")
	}
	if fc.Target.Healthcheck.Interval != "30s" {
		t.Errorf("Target.Healthcheck.Interval = %q, want %q", fc.Target.Healthcheck.Interval, "30s")
	}
	if fc.Target.Healthcheck.Timeout != "10s" {
		t.Errorf("Target.Healthcheck.Timeout = %q, want %q", fc.Target.Healthcheck.Timeout, "10s")
	}
	if fc.Target.Healthcheck.Retries != 3 {
		t.Errorf("Target.Healthcheck.Retries = %d, want 3", fc.Target.Healthcheck.Retries)
	}
}
