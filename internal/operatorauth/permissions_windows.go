//go:build windows

package operatorauth

import "os"

// Credential storage is scoped to os.UserCacheDir(), normally the current
// user's LocalAppData profile. Windows inherits the profile ACL on new files.
// FileMode on Windows does not expose the effective DACL.
func validateFilePermissions(os.FileInfo) error { return nil }
