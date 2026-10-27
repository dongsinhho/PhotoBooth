package mf

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/Kirizu-Official/windows-camera-go/utils"
	"github.com/Kirizu-Official/windows-camera-go/windows/consts"
)

type MfApi struct {
	mfPlatDll          *syscall.DLL
	mfCreateAttributes *syscall.Proc
	mFStartup          *syscall.Proc
	mfShutdown         *syscall.Proc
}

func CreateNewMfPlat() (*MfApi, error) {
	dll, err := syscall.LoadDLL("Mfplat.dll")
	if err != nil {
		return nil, err
	}
	newAPI := &MfApi{mfPlatDll: dll}
	find := func(name string) (*syscall.Proc, error) {
		proc, findErr := dll.FindProc(name)
		if findErr != nil {
			return nil, fmt.Errorf("find %s: %w", name, findErr)
		}
		return proc, nil
	}
	if newAPI.mfCreateAttributes, err = find("MFCreateAttributes"); err != nil {
		_ = dll.Release()
		return nil, err
	}
	if newAPI.mFStartup, err = find("MFStartup"); err != nil {
		_ = dll.Release()
		return nil, err
	}
	if newAPI.mfShutdown, err = find("MFShutdown"); err != nil {
		_ = dll.Release()
		return nil, err
	}
	return newAPI, nil
}

func (m *MfApi) Release() error {
	if m == nil || m.mfPlatDll == nil {
		return nil
	}
	err := m.mfPlatDll.Release()
	m.mfPlatDll = nil
	m.mfCreateAttributes = nil
	m.mFStartup = nil
	m.mfShutdown = nil
	return err
}

func (m *MfApi) MFCreateAttributes(ppMFAttributes **IMFAttributes, cInitialSize uint32) error {

	code, _, err := m.mfCreateAttributes.Call(uintptr(unsafe.Pointer(ppMFAttributes)), uintptr(cInitialSize))
	err = utils.CheckError(err)
	if err != nil {
		return err
	}

	if code != consts.S_OK {
		return &consts.HResultError{Code: code}
	}
	return nil
}

func (m *MfApi) MFStartup() error {
	code, _, err := m.mFStartup.Call(uintptr(consts.MF_VERSION), uintptr(consts.MFSTARTUP_NOSOCKET))
	err = utils.CheckError(err)
	if err != nil {
		return err
	}

	if code != consts.S_OK {
		return &consts.HResultError{Code: code}
	}
	return nil
}
func (m *MfApi) MFShutdown() error {
	code, _, err := m.mfShutdown.Call()
	err = utils.CheckError(err)
	if err != nil {
		return err
	}

	if code != consts.S_OK {
		return &consts.HResultError{Code: code}
	}
	return nil
}
