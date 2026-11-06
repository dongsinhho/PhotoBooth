//go:build !windows

package config

// Non-Windows storage mount classification is intentionally not inferred from
// path syntax. Linux/macOS mount-type enforcement remains a separate platform
// gate; this helper currently preserves existing path behavior there.
func validateLocalStoragePath(string) error { return nil }
