//go:build windows

package session

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

const (
	printLockGenericRead           = 0x80000000
	printLockGenericWrite          = 0x40000000
	printLockDeleteAccess          = 0x00010000
	printLockShareRead             = 0x00000001
	printLockShareWrite            = 0x00000002
	printLockShareDelete           = 0x00000004
	printLockCreateNew             = 1
	printLockOpenExisting          = 3
	printLockFileAttributeNormal   = 0x00000080
	printLockOpenReparsePoint      = 0x00200000
	printLockFileDispositionInfo   = 4
	printLockErrorSharingViolation = syscall.Errno(32)
	printLockErrorLockViolation    = syscall.Errno(33)
	printLockErrorFileExists       = syscall.Errno(80)
	printLockErrorAlreadyExists    = syscall.Errno(183)
)

var (
	createFileWProc                = syscall.NewLazyDLL("kernel32.dll").NewProc("CreateFileW")
	setFileInformationByHandleProc = syscall.NewLazyDLL("kernel32.dll").NewProc("SetFileInformationByHandle")
)

func currentPrintLockLeaseVersion() int { return PrintLockLeaseVersion }

func createPrintLockFile(path string) (*os.File, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, _, callErr := createFileWProc.Call(
		uintptr(unsafe.Pointer(name)),
		printLockGenericRead|printLockGenericWrite|printLockDeleteAccess,
		printLockShareRead,
		0,
		printLockCreateNew,
		printLockFileAttributeNormal,
		0,
	)
	if handle == uintptr(syscall.InvalidHandle) {
		if callErr == printLockErrorFileExists || callErr == printLockErrorAlreadyExists {
			return nil, &os.PathError{Op: "create", Path: path, Err: os.ErrExist}
		}
		if callErr != syscall.Errno(0) {
			return nil, callErr
		}
		return nil, errors.New("CreateFileW failed to create printer lock")
	}
	return os.NewFile(handle, path), nil
}

func claimPrintLockFile(path string) (*os.File, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, _, callErr := createFileWProc.Call(
		uintptr(unsafe.Pointer(name)),
		printLockGenericRead|printLockGenericWrite|printLockDeleteAccess,
		0,
		0,
		printLockOpenExisting,
		printLockFileAttributeNormal|printLockOpenReparsePoint,
		0,
	)
	if handle == uintptr(syscall.InvalidHandle) {
		if callErr == printLockErrorSharingViolation || callErr == printLockErrorLockViolation {
			return nil, ErrPrintLockActive
		}
		if callErr != syscall.Errno(0) {
			return nil, callErr
		}
		return nil, errors.New("CreateFileW failed to claim printer lock")
	}
	return os.NewFile(handle, path), nil
}

func removePrintLockFile(file *os.File, _ string) error {
	if file == nil {
		return errors.New("printer lock handle is unavailable")
	}
	deleteFile := byte(1)
	ok, _, callErr := setFileInformationByHandleProc.Call(
		file.Fd(),
		printLockFileDispositionInfo,
		uintptr(unsafe.Pointer(&deleteFile)),
		unsafe.Sizeof(deleteFile),
	)
	if ok == 0 {
		closeErr := file.Close()
		if callErr != syscall.Errno(0) {
			return errors.Join(callErr, closeErr)
		}
		return errors.Join(errors.New("SetFileInformationByHandle(FILE_DISPOSITION_INFO) failed"), closeErr)
	}
	return file.Close()
}

func openPrintLockForRead(path string) (*os.File, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, _, callErr := createFileWProc.Call(
		uintptr(unsafe.Pointer(name)),
		printLockGenericRead,
		printLockShareRead|printLockShareWrite|printLockShareDelete,
		0,
		printLockOpenExisting,
		printLockFileAttributeNormal|printLockOpenReparsePoint,
		0,
	)
	if handle == uintptr(syscall.InvalidHandle) {
		if callErr != syscall.Errno(0) {
			return nil, callErr
		}
		return nil, errors.New("CreateFileW failed to open printer lock for read")
	}
	return os.NewFile(handle, path), nil
}
