//go:build windows

package session

import (
	"fmt"
	"syscall"
	"unsafe"
)

func publishFrameSnapshotNoReplace(source, destination string) error {
	sourceUTF16, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	destinationUTF16, err := syscall.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	// Without MOVEFILE_REPLACE_EXISTING the target must be absent; WRITE_THROUGH
	// makes the completed move durable before the initial manifest is published.
	result, _, callErr := moveFileExW.Call(
		uintptr(unsafe.Pointer(sourceUTF16)),
		uintptr(unsafe.Pointer(destinationUTF16)),
		moveFileWriteThrough,
	)
	if result == 0 {
		if callErr != syscall.Errno(0) {
			return callErr
		}
		return fmt.Errorf("MoveFileExW failed")
	}
	return nil
}
