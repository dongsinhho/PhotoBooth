//go:build !windows && !linux

package config

import "path/filepath"

// macOS and other platforms still need mount/filesystem classification before
// local mode can claim to reject network-backed storage.
func canonicalLocalStoragePath(path string) (string, error) {
	return filepath.Abs(filepath.Clean(path))
}

func validateLocalStoragePath(string) error { return nil }
