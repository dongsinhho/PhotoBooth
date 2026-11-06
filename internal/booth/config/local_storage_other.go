//go:build !windows && !linux

package config

// macOS and other platforms still need mount/filesystem classification before
// local mode can claim to reject network-backed storage.
func validateLocalStoragePath(string) error { return nil }
