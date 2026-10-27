package mf

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/Kirizu-Official/windows-camera-go/windows/consts"
)

type MfReadWrite struct {
	mfreadwriteDll                      *syscall.DLL
	mfCreateSourceReaderFromMediaSource *syscall.Proc
}

func CreateNewMfReadWrite() (*MfReadWrite, error) {
	dll, err := syscall.LoadDLL("Mfreadwrite.dll")
	if err != nil {
		return nil, err
	}
	proc, err := dll.FindProc("MFCreateSourceReaderFromMediaSource")
	if err != nil {
		_ = dll.Release()
		return nil, fmt.Errorf("find MFCreateSourceReaderFromMediaSource: %w", err)
	}
	return &MfReadWrite{
		mfreadwriteDll:                      dll,
		mfCreateSourceReaderFromMediaSource: proc,
	}, nil
}

func (m *MfReadWrite) Release() error {
	if m == nil || m.mfreadwriteDll == nil {
		return nil
	}
	err := m.mfreadwriteDll.Release()
	m.mfreadwriteDll = nil
	m.mfCreateSourceReaderFromMediaSource = nil
	return err
}

func (m *MfReadWrite) MFCreateSourceReaderFromMediaSource(pMediaSource *IMFMediaSource, pAttributes *IMFAttributes, ppSourceReader **IMFSourceReader) error {
	code, _, _ := m.mfCreateSourceReaderFromMediaSource.Call(
		uintptr(unsafe.Pointer(pMediaSource)),
		uintptr(unsafe.Pointer(pAttributes)),
		uintptr(unsafe.Pointer(ppSourceReader)),
	)

	if code != consts.S_OK {
		return &consts.HResultError{Code: code}
	}
	return nil
}
