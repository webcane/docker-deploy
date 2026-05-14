package filetransfer

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestShouldExclude(t *testing.T) {
	tests := []struct {
		name     string
		relPath  string
		excludes []string
		want     bool
	}{
		{
			name:     "no excludes returns false",
			relPath:  "README.md",
			excludes: []string{},
			want:     false,
		},
		{
			name:     "directory prefix match - .git/config",
			relPath:  ".git/config",
			excludes: []string{".git/"},
			want:     true,
		},
		{
			name:     "deep path - node_modules anywhere",
			relPath:  "deep/node_modules/pkg/index.js",
			excludes: []string{"node_modules/"},
			want:     true,
		},
		{
			name:     "glob match - *.log basename",
			relPath:  "app.log",
			excludes: []string{"*.log"},
			want:     true,
		},
		{
			name:     "glob match - *.log in subdir",
			relPath:  "logs/app.log",
			excludes: []string{"*.log"},
			want:     true,
		},
		{
			name:     ".env not in built-in excludes",
			relPath:  ".env",
			excludes: []string{".git/", "node_modules/", "vendor/", "*.log", ".DS_Store", "__pycache__/"},
			want:     false,
		},
		{
			name:     "vendor/ directory prefix",
			relPath:  "vendor/dep/lib.go",
			excludes: []string{"vendor/"},
			want:     true,
		},
		{
			name:     "exact match - .DS_Store",
			relPath:  ".DS_Store",
			excludes: []string{".DS_Store"},
			want:     true,
		},
		{
			name:     "__pycache__/ directory prefix",
			relPath:  "__pycache__/cache.pyc",
			excludes: []string{"__pycache__/"},
			want:     true,
		},
		{
			name:     "deploy.yaml not matched by unrelated patterns",
			relPath:  "deploy.yaml",
			excludes: []string{".git/", "*.log"},
			want:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ShouldExclude(tc.relPath, tc.excludes)
			if got != tc.want {
				t.Errorf("ShouldExclude(%q, %v) = %v; want %v", tc.relPath, tc.excludes, got, tc.want)
			}
		})
	}
}

func TestWalkFiles(t *testing.T) {
	// Create a temp directory with a specific structure.
	dir := t.TempDir()

	// Create files: .git/config, README.md, compose.yaml, app.log
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		".git/config":  "git config content",
		"README.md":    "# readme",
		"compose.yaml": "version: '3'",
		"app.log":      "log content",
	}
	for rel, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	excludes := []string{".git/", "*.log"}
	got, err := WalkFiles(dir, excludes)
	if err != nil {
		t.Fatalf("WalkFiles returned error: %v", err)
	}

	want := []string{"README.md", "compose.yaml"}
	sort.Strings(got)
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("WalkFiles returned %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("WalkFiles[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}

func TestWalkFilesSkipsDirs(t *testing.T) {
	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "subdir", "file.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := WalkFiles(dir, nil)
	if err != nil {
		t.Fatalf("WalkFiles returned error: %v", err)
	}

	for _, p := range got {
		info, err := os.Stat(filepath.Join(dir, p))
		if err != nil {
			t.Fatalf("stat %q: %v", p, err)
		}
		if info.IsDir() {
			t.Errorf("WalkFiles returned directory %q; expected only files", p)
		}
	}
}
