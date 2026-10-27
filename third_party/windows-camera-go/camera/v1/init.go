package camera

import (
	"fmt"
	"syscall"

	"github.com/Kirizu-Official/windows-camera-go/windows/consts"
	"github.com/Kirizu-Official/windows-camera-go/windows/mf"
)

var (
	mfApi          *mf.MF
	mfPlat         *mf.MfApi
	mfReadWrite    *mf.MfReadWrite
	ole32dll       *syscall.DLL
	comInitialized bool
	mfStarted      bool
)

// Init initializes the Windows COM and Media Foundation runtime on the calling
// OS thread. The caller must balance a successful Init with Shutdown on that
// same thread.
func Init() error {
	if mfApi != nil || mfPlat != nil || mfReadWrite != nil || ole32dll != nil {
		return fmt.Errorf("Windows Media Foundation runtime is already initialized")
	}

	var err error
	mfApi, err = mf.CreateNewMF()
	if err != nil {
		shutdownResources()
		return err
	}
	mfPlat, err = mf.CreateNewMfPlat()
	if err != nil {
		shutdownResources()
		return err
	}
	mfReadWrite, err = mf.CreateNewMfReadWrite()
	if err != nil {
		shutdownResources()
		return err
	}
	if err := initWindowsCOM(); err != nil {
		shutdownResources()
		return err
	}
	return nil
}

func initWindowsCOM() error {
	var err error
	ole32dll, err = syscall.LoadDLL("Ole32.dll")
	if err != nil {
		return fmt.Errorf("failed to load Ole32.dll: %w", err)
	}
	proc, err := ole32dll.FindProc("CoInitializeEx")
	if err != nil {
		return fmt.Errorf("find CoInitializeEx: %w", err)
	}
	r, _, _ := proc.Call(0, uintptr(consts.COINIT_MULTITHREADED))
	// S_FALSE (1) also means COM was initialized and must be balanced.
	if r != consts.S_OK && r != 1 {
		return fmt.Errorf("CoInitializeEx failed: HRESULT 0x%08X", uint32(r))
	}
	comInitialized = true

	if err := mfPlat.MFStartup(); err != nil {
		return fmt.Errorf("Media Foundation startup: %w", err)
	}
	mfStarted = true
	return nil
}

// Shutdown releases the camera runtime. It must run on the same locked OS
// thread as Init. COM and Media Foundation DLL references are released after
// native resources have shut down.
func Shutdown() {
	shutdownResources()
}

func shutdownResources() {
	if mfStarted && mfPlat != nil {
		if err := mfPlat.MFShutdown(); err != nil {
			fmt.Printf("[WARN] GoCamera: MFShutdown failed: %v\n", err)
		}
		mfStarted = false
	}

	if comInitialized && ole32dll != nil {
		proc, err := ole32dll.FindProc("CoUninitialize")
		if err == nil && proc != nil {
			// CoUninitialize has no meaningful return value.
			_, _, _ = proc.Call(0)
		} else {
			fmt.Printf("[WARN] GoCamera: find CoUninitialize failed: %v\n", err)
		}
		comInitialized = false
	}

	if mfReadWrite != nil {
		if err := mfReadWrite.Release(); err != nil {
			fmt.Printf("[WARN] GoCamera: release Mfreadwrite.dll failed: %v\n", err)
		}
		mfReadWrite = nil
	}
	if mfPlat != nil {
		if err := mfPlat.Release(); err != nil {
			fmt.Printf("[WARN] GoCamera: release Mfplat.dll failed: %v\n", err)
		}
		mfPlat = nil
	}
	if mfApi != nil {
		if err := mfApi.Release(); err != nil {
			fmt.Printf("[WARN] GoCamera: release Mf.dll failed: %v\n", err)
		}
		mfApi = nil
	}
	if ole32dll != nil {
		if err := ole32dll.Release(); err != nil {
			fmt.Printf("[WARN] GoCamera: release Ole32.dll failed: %v\n", err)
		}
		ole32dll = nil
	}
}
