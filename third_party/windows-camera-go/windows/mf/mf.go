package mf

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/Kirizu-Official/windows-camera-go/utils"
	"github.com/Kirizu-Official/windows-camera-go/windows/consts"
)

// MFEnumDeviceSources returns its pointer array allocated with CoTaskMemAlloc.
// The array must be freed by the caller after copying its IMFActivate pointers.
var coTaskMemFreeProc = syscall.NewLazyDLL("Ole32.dll").NewProc("CoTaskMemFree")

type MF struct {
	mf *syscall.DLL

	mfEnumDeviceSources *syscall.Proc
}

func CreateNewMF() (*MF, error) {
	dll, err := syscall.LoadDLL("Mf.dll")
	if err != nil {
		return nil, err
	}
	newMF := &MF{mf: dll}
	proc, err := dll.FindProc("MFEnumDeviceSources")
	if err != nil {
		_ = dll.Release()
		return nil, fmt.Errorf("find MFEnumDeviceSources: %w", err)
	}
	newMF.mfEnumDeviceSources = proc
	return newMF, nil
}
func (mf *MF) Release() error {
	if mf == nil || mf.mf == nil {
		return nil
	}
	err := mf.mf.Release()
	mf.mf = nil
	mf.mfEnumDeviceSources = nil
	return err
}

func (mf *MF) MFEnumDeviceSources(pAttributes *IMFAttributes, pcSourceActivate *uint32) ([]*IMFActivate, error) {

	var listPtr uintptr
	call, _, err := mf.mfEnumDeviceSources.Call(uintptr(unsafe.Pointer(pAttributes)), uintptr(unsafe.Pointer(&listPtr)), uintptr(unsafe.Pointer(pcSourceActivate)))
	// CoTaskMemFree must run on every return path when the API returned an array.
	// The pointed-to IMFActivate interfaces are separately released by the caller.
	if listPtr != 0 {
		defer coTaskMemFreeProc.Call(listPtr)
	}
	err = utils.CheckError(err)
	if err != nil {
		return nil, err
	}
	if call != consts.S_OK {
		return nil, consts.HResultError{Code: call}
	}

	var pppSourceActivate []*IMFActivate
	for i := uint32(0); i < *pcSourceActivate; i++ {
		arrayIthElementOffset := uintptr(i) * unsafe.Sizeof(uintptr(0))
		arrayIthElementAddress := listPtr + arrayIthElementOffset
		arrayIthElementPointer := (*uintptr)(unsafe.Pointer(arrayIthElementAddress))
		pppSourceActivate = append(pppSourceActivate, (*IMFActivate)(unsafe.Pointer(*arrayIthElementPointer)))
	}
	return pppSourceActivate, nil

}
