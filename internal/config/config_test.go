package config

import (
	"os"
	"path/filepath"
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
			cfg, err := Resolve(tt.flagHost, "", file, "myproject")
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
			cfg, err := Resolve("", tt.flagPath, file, tt.projectName)
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
	_, err := Resolve("http://not-ssh.example.com", "", FileConfig{}, "proj")
	if err == nil {
		t.Fatal("Resolve() with non-ssh scheme should return error")
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
