//go:build windows

package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var getDriveTypeW = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDriveTypeW")

const driveRemote = 4

// validateLocalStoragePath prevents local-only mode from writing session or
// guest output data to UNC shares or mapped network drives on Windows.
func validateLocalStoragePath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("storage path is required")
	}
	if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//") {
		return fmt.Errorf("network storage path %q is not allowed in local media mode", path)
	}
	volume := filepath.VolumeName(path)
	if volume == "" {
		// Relative paths are resolved against the station config directory
		// before this check is called by Load/Save.
		return nil
	}
	root := volume + string(filepath.Separator)
	rootPtr, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return fmt.Errorf("inspect storage volume %q: %w", root, err)
	}
	result, _, callErr := getDriveTypeW.Call(uintptr(unsafe.Pointer(rootPtr)))
	if result == driveRemote {
		return fmt.Errorf("network storage volume %q is not allowed in local media mode", root)
	}
	// DRIVE_UNKNOWN / DRIVE_NO_ROOT_DIR are rejected rather than treated as
	// local. Fixed, removable and RAM disks remain supported.
	if result == 0 || result == 1 {
		if callErr != syscall.Errno(0) {
			return fmt.Errorf("inspect storage volume %q: %w", root, callErr)
		}
		return fmt.Errorf("storage volume %q is unavailable or has an unknown type", root)
	}
	return nil
}
