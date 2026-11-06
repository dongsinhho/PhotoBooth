package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// CanonicalLocalStoragePath resolves existing path indirections while
// preserving not-yet-created child components. It does not create directories.
func CanonicalLocalStoragePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("storage path is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve storage path: %w", err)
	}
	canonical, err := canonicalLocalStoragePath(filepath.Clean(absolute))
	if err != nil {
		return "", err
	}
	return filepath.Clean(canonical), nil
}

// ValidateDistinctStorageRoots prevents session workspace and finalized output
// from sharing or nesting within the same physical directory tree.
func ValidateDistinctStorageRoots(sessionsDir, outputDir string) error {
	sessions, err := CanonicalLocalStoragePath(sessionsDir)
	if err != nil {
		return fmt.Errorf("resolve sessions root: %w", err)
	}
	output, err := CanonicalLocalStoragePath(outputDir)
	if err != nil {
		return fmt.Errorf("resolve output root: %w", err)
	}
	normalize := func(path string) string {
		path = filepath.Clean(path)
		if runtime.GOOS == "windows" {
			path = strings.ToLower(path)
		}
		return path
	}
	sessions, output = normalize(sessions), normalize(output)
	if storagePathContains(sessions, output) || storagePathContains(output, sessions) {
		return fmt.Errorf("storage.sessions_dir and storage.output_dir must be separate, non-overlapping roots (resolved sessions=%q, output=%q)", sessions, output)
	}
	return nil
}

func storagePathContains(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

// ValidateStorageChild ensures child resolves to a direct child with the same
// name beneath root. It rejects junctions/symlinks that redirect one session
// ID into another session's data tree or outside its owning root.
func ValidateStorageChild(root, child string) error {
	canonicalRoot, err := CanonicalLocalStoragePath(root)
	if err != nil {
		return fmt.Errorf("resolve storage root: %w", err)
	}
	canonicalChild, err := CanonicalLocalStoragePath(child)
	if err != nil {
		return fmt.Errorf("resolve storage child: %w", err)
	}
	requestedName := filepath.Base(filepath.Clean(child))
	resolvedName := filepath.Base(canonicalChild)
	resolvedParent := filepath.Dir(canonicalChild)
	samePath := func(left, right string) bool {
		left, right = filepath.Clean(left), filepath.Clean(right)
		if runtime.GOOS == "windows" {
			return strings.EqualFold(left, right)
		}
		return left == right
	}
	if !samePath(resolvedParent, canonicalRoot) || !samePath(resolvedName, requestedName) {
		return fmt.Errorf("storage child %q resolves outside its assigned root/identity (root=%q resolved=%q)", child, canonicalRoot, canonicalChild)
	}
	return nil
}
