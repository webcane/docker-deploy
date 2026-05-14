// Package filetransfer implements file filtering and SFTP upload for docker-deploy.
package filetransfer

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// ShouldExclude reports whether relPath (relative to the walk root, using
// forward slashes) matches any pattern in excludes.
//
// Pattern matching rules:
//   - Directory patterns ending in "/" match: the exact directory name,
//     any path that starts with the pattern, or any path component matching
//     the directory name (to handle deep paths like "a/node_modules/b").
//   - Other patterns use filepath.Match semantics against both the basename
//     and the full path.
func ShouldExclude(relPath string, excludes []string) bool {
	// Normalize to forward slashes for consistent matching.
	relPath = filepath.ToSlash(relPath)

	for _, pattern := range excludes {
		if strings.HasSuffix(pattern, "/") {
			// Directory pattern: match the dir itself, any path under it,
			// or any path component anywhere in the tree.
			dirName := strings.TrimSuffix(pattern, "/")

			// Exact match: relPath == dirName
			if relPath == dirName {
				return true
			}
			// Prefix match: relPath starts with "dirName/"
			if strings.HasPrefix(relPath, pattern) {
				return true
			}
			// Component match: any segment of relPath equals dirName.
			// This handles deep paths like "a/node_modules/b/index.js".
			parts := strings.Split(relPath, "/")
			for _, part := range parts {
				if part == dirName {
					return true
				}
			}
			continue
		}

		// Non-directory pattern: try basename match and full-path match.
		base := filepath.Base(relPath)
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, relPath); matched {
			return true
		}
	}
	return false
}

// WalkFiles walks localDir and returns all relative file paths that are NOT
// excluded by ShouldExclude(path, excludes). Directories are never returned.
// The returned slice is sorted in lexicographic order.
func WalkFiles(localDir string, excludes []string) ([]string, error) {
	var result []string

	err := filepath.WalkDir(localDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip the root directory itself.
		if path == localDir {
			return nil
		}
		// Compute the relative path.
		rel, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}
		// Normalize to forward slashes for consistent exclude matching.
		relFwd := filepath.ToSlash(rel)

		// Skip directories (do not add them to results, but keep walking).
		if d.IsDir() {
			// If the directory itself is excluded, skip the entire subtree.
			if ShouldExclude(relFwd, excludes) {
				return filepath.SkipDir
			}
			return nil
		}

		// File: add if not excluded.
		if !ShouldExclude(relFwd, excludes) {
			result = append(result, relFwd)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(result)
	return result, nil
}
