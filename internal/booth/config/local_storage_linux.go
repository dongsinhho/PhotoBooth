//go:build linux

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func canonicalLocalStoragePath(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("storage path %q is not absolute", path)
	}
	return resolveLinuxStoragePath(path)
}

func validateLocalStoragePath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("storage path is required")
	}
	if !filepath.IsAbs(path) {
		// Load and Save resolve station-relative paths before this check.
		return nil
	}

	// A configured storage root must be a directory when it already exists.
	// CanonicalLocalStoragePath is also used by ValidateStorageChild for file
	// artifacts such as frame.json, so the lower-level resolver permits a
	// regular file only as its final component.
	if info, statErr := os.Stat(path); statErr == nil {
		if !info.IsDir() {
			return fmt.Errorf("storage root %q is not a directory", path)
		}
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("inspect storage root %q: %w", path, statErr)
	}

	resolved, err := resolveLinuxStoragePath(path)
	if err != nil {
		return fmt.Errorf("resolve storage path %q: %w", path, err)
	}
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return fmt.Errorf("read mount table for local-storage validation: %w", err)
	}
	entries, err := parseMountInfo(strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("parse mount table for local-storage validation: %w", err)
	}
	mount, err := mountForPath(resolved, entries)
	if err != nil {
		return err
	}
	if isNetworkFilesystem(mount.FSType, mount.Source) {
		return fmt.Errorf("network filesystem %q (source %q) mounted at %q is not allowed in local media mode",
			mount.FSType, mount.Source, mount.MountPoint)
	}
	return nil
}

// resolveLinuxStoragePath follows symlinks in the nearest existing ancestor,
// preserving any not-yet-created child components. It also canonicalizes an
// existing regular file when it is the final component (for identity checks
// on immutable session artifacts). A file cannot be an ancestor of a storage
// path. Broken or inaccessible components fail closed.
func resolveLinuxStoragePath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	candidate := filepath.Clean(absolute)
	var missing []string
	for {
		info, statErr := os.Lstat(candidate)
		if statErr == nil {
			resolved, evalErr := filepath.EvalSymlinks(candidate)
			if evalErr != nil {
				return "", fmt.Errorf("resolve symlinked storage path component %q: %w", candidate, evalErr)
			}
			resolvedInfo, statErr := os.Stat(resolved)
			if statErr != nil {
				return "", fmt.Errorf("stat resolved storage path %q: %w", resolved, statErr)
			}
			if !resolvedInfo.IsDir() {
				if len(missing) != 0 || !resolvedInfo.Mode().IsRegular() {
					return "", fmt.Errorf("existing storage path component %q is not a directory", candidate)
				}
				return filepath.Clean(resolved), nil
			}
			// A non-symlink regular file is valid only as the final component.
			if info.Mode()&os.ModeSymlink == 0 && !info.IsDir() {
				return "", fmt.Errorf("existing storage path component %q is not a directory", candidate)
			}
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(statErr) {
			return "", fmt.Errorf("inspect storage path component %q: %w", candidate, statErr)
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return "", fmt.Errorf("no existing ancestor for storage path %q", path)
		}
		missing = append(missing, filepath.Base(candidate))
		candidate = parent
	}
}
