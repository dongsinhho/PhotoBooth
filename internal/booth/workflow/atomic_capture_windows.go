//go:build windows

package workflow

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var moveCaptureFileW = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")

const moveFileWriteThroughCapture = 0x8

func publishCaptureNoReplace(stagedPath, finalPath string) error {
	info, err := os.Lstat(stagedPath)
	if err != nil {
		return fmt.Errorf("inspect staged capture: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 {
		return fmt.Errorf("staged capture must be a non-empty regular file")
	}
	source, err := syscall.UTF16PtrFromString(stagedPath)
	if err != nil {
		return err
	}
	destination, err := syscall.UTF16PtrFromString(finalPath)
	if err != nil {
		return err
	}
	// Omitting MOVEFILE_REPLACE_EXISTING makes a collision fail closed.
	result, _, callErr := moveCaptureFileW.Call(
		uintptr(unsafe.Pointer(source)),
		uintptr(unsafe.Pointer(destination)),
		moveFileWriteThroughCapture,
	)
	if result == 0 {
		if callErr != syscall.Errno(0) {
			return fmt.Errorf("publish capture without replacement: %w", callErr)
		}
		return fmt.Errorf("MoveFileExW failed while publishing capture")
	}
	return nil
}

func syncCaptureDirectory(string) error {
	// MoveFileExW with MOVEFILE_WRITE_THROUGH flushes the move operation.
	return nil
}
