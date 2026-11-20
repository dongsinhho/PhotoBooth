//go:build !windows

package session

import (
	"errors"
	"os"
)

func currentPrintLockLeaseVersion() int { return 0 }

func createPrintLockFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
}

func claimPrintLockFile(string) (*os.File, error) {
	return nil, ErrPrintLockUnsupported
}

func removePrintLockFile(file *os.File, path string) error {
	if file == nil {
		return errors.New("printer lock handle is unavailable")
	}
	closeErr := file.Close()
	removeErr := os.Remove(path)
	if closeErr != nil {
		return errors.Join(closeErr, removeErr)
	}
	return removeErr
}

func openPrintLockForRead(path string) (*os.File, error) {
	return os.Open(path)
}
