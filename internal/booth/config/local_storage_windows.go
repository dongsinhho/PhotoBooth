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

const (
	driveRemote             = 4
	fileReadAttributes      = 0x80
	fileShareAll            = 0x7
	openExisting            = 3
	fileFlagBackupSemantics = 0x02000000
)

var (
	getDriveTypeW            = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDriveTypeW")
	createFileW              = syscall.NewLazyDLL("kernel32.dll").NewProc("CreateFileW")
	getFinalPathNameByHandle = syscall.NewLazyDLL("kernel32.dll").NewProc("GetFinalPathNameByHandleW")
)

// validateLocalStoragePath prevents local-only mode from writing session or
// guest output data to UNC shares, mapped network drives, or directory
// reparse points whose resolved target is network-backed.
func validateLocalStoragePath(path string) error {
	path = normalizeWindowsFinalPath(strings.TrimSpace(path))
	if path == "" {
		return errors.New("storage path is required")
	}
	if isWindowsUNCPath(path) {
		return fmt.Errorf("network storage path %q is not allowed in local media mode", path)
	}
	if !filepath.IsAbs(path) {
		// Relative paths are resolved against the station config directory
		// before this check is called by Load/Save.
		return nil
	}

	resolved, err := resolveWindowsStorageTarget(path)
	if err != nil {
		return fmt.Errorf("resolve storage path %q: %w", path, err)
	}
	if isWindowsUNCPath(resolved) {
		return fmt.Errorf("network storage target %q is not allowed in local media mode", resolved)
	}

	volume := filepath.VolumeName(resolved)
	if volume == "" {
		return fmt.Errorf("cannot determine storage volume for resolved path %q", resolved)
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

// resolveWindowsStorageTarget follows existing junctions/symlinks by opening
// the nearest existing ancestor without FILE_FLAG_OPEN_REPARSE_POINT, then
// asks Windows for the final target path. Missing child components are
// appended after the resolved ancestor so paths need not exist at setup time.
func resolveWindowsStorageTarget(path string) (string, error) {
	candidate := filepath.Clean(path)
	var missing []string
	for {
		name, err := syscall.UTF16PtrFromString(candidate)
		if err != nil {
			return "", err
		}
		handle, _, callErr := createFileW.Call(
			uintptr(unsafe.Pointer(name)),
			fileReadAttributes,
			fileShareAll,
			0,
			openExisting,
			fileFlagBackupSemantics,
			0,
		)
		if syscall.Handle(handle) != syscall.InvalidHandle {
			finalPath, finalErr := windowsFinalPath(syscall.Handle(handle))
			_ = syscall.CloseHandle(syscall.Handle(handle))
			if finalErr != nil {
				return "", finalErr
			}
			for i := len(missing) - 1; i >= 0; i-- {
				finalPath = filepath.Join(finalPath, missing[i])
			}
			return finalPath, nil
		}
		if !errors.Is(callErr, syscall.ERROR_FILE_NOT_FOUND) && !errors.Is(callErr, syscall.ERROR_PATH_NOT_FOUND) {
			return "", fmt.Errorf("open existing path component %q: %w", candidate, callErr)
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return "", fmt.Errorf("no existing ancestor for %q", path)
		}
		missing = append(missing, filepath.Base(candidate))
		candidate = parent
	}
}

func windowsFinalPath(handle syscall.Handle) (string, error) {
	buffer := make([]uint16, 32768)
	length, _, callErr := getFinalPathNameByHandle.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
		0, // FILE_NAME_NORMALIZED | VOLUME_NAME_DOS
	)
	if length == 0 {
		return "", fmt.Errorf("GetFinalPathNameByHandleW: %w", callErr)
	}
	if length >= uintptr(len(buffer)) {
		return "", errors.New("resolved Windows storage path exceeds the supported path buffer")
	}
	path := syscall.UTF16ToString(buffer[:length])
	return normalizeWindowsFinalPath(path), nil
}

func isWindowsUNCPath(path string) bool {
	upper := strings.ToUpper(path)
	if strings.HasPrefix(upper, "\\\\?\\UNC\\") || strings.HasPrefix(path, "//") {
		return true
	}
	if !strings.HasPrefix(path, "\\\\") {
		return false
	}
	// Volume-GUID device paths are local volume references, not UNC shares.
	if strings.HasPrefix(upper, "\\\\?\\VOLUME{") || strings.HasPrefix(upper, "\\\\.\\") {
		return false
	}
	return true
}

func normalizeWindowsFinalPath(path string) string {
	upper := strings.ToUpper(path)
	switch {
	case strings.HasPrefix(upper, "\\\\?\\UNC\\"):
		path = "\\\\" + path[len("\\\\?\\UNC\\"):]
	case isExtendedDrivePath(path):
		path = strings.TrimPrefix(path, "\\\\?\\")
	}
	return filepath.Clean(path)
}

func isExtendedDrivePath(path string) bool {
	upper := strings.ToUpper(path)
	return len(upper) >= 7 &&
		strings.HasPrefix(upper, "\\\\?\\") &&
		upper[4] >= 'A' && upper[4] <= 'Z' &&
		upper[5] == ':' &&
		(upper[6] == '\\' || upper[6] == '/')
}
