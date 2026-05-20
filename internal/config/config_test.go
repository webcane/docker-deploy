package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
			cfg, err := Resolve(FlagOpts{Host: tt.flagHost, ComposeFile: "compose.yaml"}, file, "myproject", "")
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
			cfg, err := Resolve(FlagOpts{Path: tt.flagPath, ComposeFile: "compose.yaml"}, file, tt.projectName, "")
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
	_, err := Resolve(FlagOpts{Host: "http://not-ssh.example.com", ComposeFile: "compose.yaml"}, FileConfig{}, "proj", "")
	if err == nil {
		t.Fatal("Resolve() with non-ssh scheme should return error")
	}
}

// TestResolveExcludes verifies that Config.Excludes follows the
// defaultExcludes + file.Target.Exclude + flagExcludes merge model with
// deduplication (insertion-order preserved, later duplicates dropped).
func TestResolveExcludes(t *testing.T) {
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
			cfg, err := Resolve(FlagOpts{Excludes: tt.flagExcludes, ComposeFile: "compose.yaml"}, file, "proj", "")
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
func TestResolveSkipEnv(t *testing.T) {
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
		cfg, err := Resolve(FlagOpts{SkipEnv: true, ComposeFile: "compose.yaml"}, FileConfig{}, "proj", "")
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
		cfg, err := Resolve(FlagOpts{ComposeFile: "compose.yaml"}, file, "proj", "")
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
		cfg, err := Resolve(FlagOpts{SkipEnv: true, ComposeFile: "compose.yaml"}, file, "proj", "")
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
		cfg, err := Resolve(FlagOpts{SkipEnv: true, Excludes: []string{".env"}, ComposeFile: "compose.yaml"}, FileConfig{}, "proj", "")
		if err != nil {
			t.Fatalf("Resolve() unexpected error: %v", err)
		}
		if c := countOf(cfg.Excludes, ".env"); c != 1 {
			t.Errorf(".env appears %d times in Excludes, want exactly 1; got %v", c, cfg.Excludes)
		}
	})

	t.Run("no_skip_env_dot_env_absent", func(t *testing.T) {
		// Neither flag nor file enables SkipEnv — .env must not appear.
		cfg, err := Resolve(FlagOpts{ComposeFile: "compose.yaml"}, FileConfig{}, "proj", "")
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
		cfg, err := Resolve(FlagOpts{Verbose: true, ComposeFile: "compose.yaml"}, FileConfig{}, "proj", "")
		if err != nil {
			t.Fatalf("Resolve() unexpected error: %v", err)
		}
		if !cfg.Verbose {
			t.Errorf("cfg.Verbose = false, want true")
		}
	})

	t.Run("verbose_false_default", func(t *testing.T) {
		cfg, err := Resolve(FlagOpts{ComposeFile: "compose.yaml"}, FileConfig{}, "proj", "")
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

	cfg, err := Resolve(FlagOpts{}, FileConfig{}, "proj", dir)
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
			cfg, err := Resolve(FlagOpts{Force: tt.flagForce, ComposeFile: "compose.yaml"}, file, "proj", "")
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
	cfg, err := Resolve(FlagOpts{ComposeFile: "docker-compose.yml"}, file, "proj", dir)
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
	cfg, err := Resolve(FlagOpts{}, file, "proj", dir)
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
	cfg, err := Resolve(FlagOpts{}, FileConfig{}, "proj", dir)
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
	cfg, err := Resolve(FlagOpts{}, FileConfig{}, "proj", dir)
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
	_, err := Resolve(FlagOpts{}, FileConfig{}, "proj", dir)
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
	cfg, err := Resolve(FlagOpts{Excludes: []string{"*.tmp"}}, file, "proj", dir)
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

// --- TestResolveHealthConfig tests (TDD: 05-01) ---

// TestResolveHealthConfig verifies that Config.HealthTimeout and
// Config.HealthInterval follow flag > file > default precedence.
// Defaults: HealthTimeout=60, HealthInterval=5.
// Zero-value is treated as "not set" (same as empty-string pattern for other fields).
func TestResolveHealthConfig(t *testing.T) {
	dir := t.TempDir()
	// Create a compose.yaml so Resolve() doesn't fail on missing compose file.
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("creating compose.yaml: %v", err)
	}

	tests := []struct {
		name               string
		flagHealthTimeout  int
		flagHealthInterval int
		fileHealthTimeout  int
		fileHealthInterval int
		wantHealthTimeout  int
		wantHealthInterval int
	}{
		{
			name:               "defaults_when_no_overrides",
			flagHealthTimeout:  0,
			flagHealthInterval: 0,
			fileHealthTimeout:  0,
			fileHealthInterval: 0,
			wantHealthTimeout:  60,
			wantHealthInterval: 5,
		},
		{
			name:               "file_overrides_default",
			flagHealthTimeout:  0,
			flagHealthInterval: 0,
			fileHealthTimeout:  30,
			fileHealthInterval: 10,
			wantHealthTimeout:  30,
			wantHealthInterval: 10,
		},
		{
			name:               "flag_overrides_file",
			flagHealthTimeout:  45,
			flagHealthInterval: 15,
			fileHealthTimeout:  30,
			fileHealthInterval: 10,
			wantHealthTimeout:  45,
			wantHealthInterval: 15,
		},
		{
			name:               "zero_file_falls_back_to_default",
			flagHealthTimeout:  0,
			flagHealthInterval: 0,
			fileHealthTimeout:  0,
			fileHealthInterval: 0,
			wantHealthTimeout:  60,
			wantHealthInterval: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := FileConfig{
				Target: TargetConfig{
					HealthTimeout:  tt.fileHealthTimeout,
					HealthInterval: tt.fileHealthInterval,
				},
			}
			cfg, err := Resolve(FlagOpts{HealthTimeout: tt.flagHealthTimeout, HealthInterval: tt.flagHealthInterval}, file, "proj", dir)
			if err != nil {
				t.Fatalf("Resolve() unexpected error: %v", err)
			}
			if cfg.HealthTimeout != tt.wantHealthTimeout {
				t.Errorf("HealthTimeout = %d, want %d", cfg.HealthTimeout, tt.wantHealthTimeout)
			}
			if cfg.HealthInterval != tt.wantHealthInterval {
				t.Errorf("HealthInterval = %d, want %d", cfg.HealthInterval, tt.wantHealthInterval)
			}
		})
	}
}

// --- LoadFile tests ---

func TestLoadFile(t *testing.T) {
	t.Run("no deploy.yaml returns zero config no error", func(t *testing.T) {
		dir := t.TempDir()
		fc, err := LoadFile(dir)
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
		fc, err := LoadFile(dir)
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
		_, err := LoadFile(dir)
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
		fc, err := LoadFile(dir)
		if err != nil {
			t.Fatalf("LoadFile() unexpected error: %v", err)
		}
		// Flat keys should be ignored — target subsection is the schema
		if fc.Target.Host != "" {
			t.Errorf("flat 'host' key should not populate Target.Host, got %q", fc.Target.Host)
		}
	})
}
