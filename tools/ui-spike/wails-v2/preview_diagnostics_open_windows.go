//go:build windows

package main

import (
	"os"
	"syscall"
)

// openPreviewDiagnostics opens an append-only diagnostic file while allowing
// the host-side soak harness to read the JSONL concurrently.
func openPreviewDiagnostics(path string) (*os.File, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := syscall.CreateFile(
		name,
		syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.OPEN_ALWAYS,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, err
	}
	if _, err := syscall.Seek(handle, 0, 2); err != nil {
		_ = syscall.CloseHandle(handle)
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}
