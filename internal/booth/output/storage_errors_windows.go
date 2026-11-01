//go:build windows

package output

import (
	"errors"
	"syscall"
)

func isInsufficientStorageError(err error) bool {
	return errors.Is(err, syscall.Errno(112)) || // ERROR_DISK_FULL
		errors.Is(err, syscall.Errno(39)) // ERROR_HANDLE_DISK_FULL
}
