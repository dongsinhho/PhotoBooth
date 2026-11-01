//go:build !windows

package output

import (
	"errors"
	"syscall"
)

func isInsufficientStorageError(err error) bool {
	return errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT)
}
