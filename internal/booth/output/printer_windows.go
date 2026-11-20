//go:build windows

package output

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	dmOutBuffer      = 0x00000002
	dmInBuffer       = 0x00000008
	dmOrientation    = 0x00000001
	dmPortrait       = 1
	dmLandscape      = 2
	dibRGBColors     = 0
	biRGB            = 0
	srccopy          = 0x00CC0020
	halftone         = 4
	capHorizontalRes = 8
	capVerticalRes   = 10
)

var (
	openPrinterWProc       = syscall.NewLazyDLL("winspool.drv").NewProc("OpenPrinterW")
	closePrinterProc       = syscall.NewLazyDLL("winspool.drv").NewProc("ClosePrinter")
	documentPropertiesProc = syscall.NewLazyDLL("winspool.drv").NewProc("DocumentPropertiesW")

	createDCWProc         = syscall.NewLazyDLL("gdi32.dll").NewProc("CreateDCW")
	deleteDCProc          = syscall.NewLazyDLL("gdi32.dll").NewProc("DeleteDC")
	getDeviceCapsProc     = syscall.NewLazyDLL("gdi32.dll").NewProc("GetDeviceCaps")
	startDocWProc         = syscall.NewLazyDLL("gdi32.dll").NewProc("StartDocW")
	startPageProc         = syscall.NewLazyDLL("gdi32.dll").NewProc("StartPage")
	endPageProc           = syscall.NewLazyDLL("gdi32.dll").NewProc("EndPage")
	endDocProc            = syscall.NewLazyDLL("gdi32.dll").NewProc("EndDoc")
	abortDocProc          = syscall.NewLazyDLL("gdi32.dll").NewProc("AbortDoc")
	stretchDIBitsProc     = syscall.NewLazyDLL("gdi32.dll").NewProc("StretchDIBits")
	setStretchBltModeProc = syscall.NewLazyDLL("gdi32.dll").NewProc("SetStretchBltMode")
	setBrushOrgExProc     = syscall.NewLazyDLL("gdi32.dll").NewProc("SetBrushOrgEx")
)

type docInfoW struct {
	cbSize   int32
	docName  *uint16
	output   *uint16
	dataType *uint16
	docType  uint32
}

type bitmapInfoHeader struct {
	size          uint32
	width         int32
	height        int32
	planes        uint16
	bitCount      uint16
	compression   uint32
	sizeImage     uint32
	xPelsPerMeter int32
	yPelsPerMeter int32
	clrUsed       uint32
	clrImportant  uint32
}

type rgbQuad struct {
	blue     byte
	green    byte
	red      byte
	reserved byte
}

type bitmapInfo struct {
	header bitmapInfoHeader
	colors [1]rgbQuad
}

type winPoint struct {
	x int32
	y int32
}

func printJPEGToWindowsQueue(ctx context.Context, printerName, sessionID, documentName string, pixels []byte, width, height int, landscape bool, mediaForm string) (jobID string, err error) {
	accepted := false
	acceptedJobID := ""
	defer func() {
		if err != nil {
			err = &PrinterSubmissionError{Uncertain: accepted, JobID: acceptedJobID, Err: err}
		}
	}()
	if ctx == nil {
		return "", errors.New("printer context is required")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if width <= 0 || height <= 0 || len(pixels) != width*height*4 {
		return "", errors.New("printer DIB dimensions or pixel buffer are invalid")
	}

	// GDI device contexts are used and released on one OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	printerNameUTF16, err := syscall.UTF16PtrFromString(printerName)
	if err != nil {
		return "", fmt.Errorf("encode Windows printer queue name: %w", err)
	}
	var printerHandle syscall.Handle
	result, _, callErr := openPrinterWProc.Call(
		uintptr(unsafe.Pointer(printerNameUTF16)),
		uintptr(unsafe.Pointer(&printerHandle)),
		0,
	)
	if result == 0 {
		return "", win32CallError("OpenPrinterW", callErr)
	}
	defer closePrinterProc.Call(uintptr(printerHandle))

	devMode, err := loadPrinterDevMode(printerHandle, printerNameUTF16, landscape, mediaForm)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	driverName, err := syscall.UTF16PtrFromString("WINSPOOL")
	if err != nil {
		return "", err
	}
	hdcValue, _, callErr := createDCWProc.Call(
		uintptr(unsafe.Pointer(driverName)),
		uintptr(unsafe.Pointer(printerNameUTF16)),
		0,
		uintptr(unsafe.Pointer(&devMode[0])),
	)
	if hdcValue == 0 {
		return "", win32CallError("CreateDCW", callErr)
	}
	hdc := hdcValue
	defer deleteDCProc.Call(hdc)

	pageWidth := int32(getDeviceCaps(hdc, capHorizontalRes))
	pageHeight := int32(getDeviceCaps(hdc, capVerticalRes))
	if pageWidth <= 0 || pageHeight <= 0 {
		return "", fmt.Errorf("printer %q returned an invalid printable area (%d x %d)", printerName, pageWidth, pageHeight)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	title, err := syscall.UTF16PtrFromString(documentName)
	if err != nil {
		return "", err
	}
	docInfo := docInfoW{cbSize: int32(unsafe.Sizeof(docInfoW{})), docName: title}
	spoolJobID, _, callErr := startDocWProc.Call(hdc, uintptr(unsafe.Pointer(&docInfo)))
	if int32(spoolJobID) <= 0 {
		return "", win32CallError("StartDocW", callErr)
	}
	accepted = true
	acceptedJobID = fmt.Sprintf("%d", uint32(spoolJobID))
	docStarted := true
	docFinished := false
	defer func() {
		if docStarted && !docFinished {
			abortDocProc.Call(hdc)
		}
	}()

	if err := ctx.Err(); err != nil {
		return "", err
	}
	pageResult, _, callErr := startPageProc.Call(hdc)
	if int32(pageResult) <= 0 {
		return "", win32CallError("StartPage", callErr)
	}

	mode, _, callErr := setStretchBltModeProc.Call(hdc, halftone)
	if mode == 0 {
		return "", win32CallError("SetStretchBltMode", callErr)
	}
	var brushOrigin winPoint
	brushResult, _, callErr := setBrushOrgExProc.Call(
		hdc, 0, 0, uintptr(unsafe.Pointer(&brushOrigin)),
	)
	if brushResult == 0 {
		return "", win32CallError("SetBrushOrgEx", callErr)
	}

	bmi := bitmapInfo{
		header: bitmapInfoHeader{
			size:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
			width:       int32(width),
			height:      -int32(height), // top-down DIB
			planes:      1,
			bitCount:    32,
			compression: biRGB,
			sizeImage:   uint32(len(pixels)),
		},
	}
	copied, _, callErr := stretchDIBitsProc.Call(
		hdc,
		0, 0, uintptr(pageWidth), uintptr(pageHeight),
		0, 0, uintptr(width), uintptr(height),
		uintptr(unsafe.Pointer(&pixels[0])),
		uintptr(unsafe.Pointer(&bmi)),
		dibRGBColors,
		srccopy,
	)
	runtime.KeepAlive(pixels)
	runtime.KeepAlive(bmi)
	if int32(copied) == -1 || copied == 0 {
		return "", win32CallError("StretchDIBits", callErr)
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}
	pageEnd, _, callErr := endPageProc.Call(hdc)
	if int32(pageEnd) <= 0 {
		return "", win32CallError("EndPage", callErr)
	}
	docEnd, _, callErr := endDocProc.Call(hdc)
	if int32(docEnd) <= 0 {
		return "", win32CallError("EndDoc", callErr)
	}
	docFinished = true
	return acceptedJobID, nil
}

func loadPrinterDevMode(printerHandle syscall.Handle, printerName *uint16, landscape bool, mediaForm string) ([]byte, error) {
	required, _, callErr := documentPropertiesProc.Call(
		0, uintptr(printerHandle), uintptr(unsafe.Pointer(printerName)), 0, 0, 0,
	)
	if int32(required) <= 0 || required > 1<<20 {
		return nil, win32CallError("DocumentPropertiesW(size)", callErr)
	}
	devMode := make([]byte, int(required))
	result, _, callErr := documentPropertiesProc.Call(
		0,
		uintptr(printerHandle),
		uintptr(unsafe.Pointer(printerName)),
		uintptr(unsafe.Pointer(&devMode[0])),
		0,
		dmOutBuffer,
	)
	if int32(result) != 1 {
		return nil, win32CallError("DocumentPropertiesW(default)", callErr)
	}
	// Update only documented DEVMODEW public fields. The helper preserves
	// the vendor-private tail returned by DocumentPropertiesW.
	if err := applyWindowsDevModeOptions(devMode, landscape, mediaForm); err != nil {
		return nil, err
	}

	result, _, callErr = documentPropertiesProc.Call(
		0,
		uintptr(printerHandle),
		uintptr(unsafe.Pointer(printerName)),
		uintptr(unsafe.Pointer(&devMode[0])),
		uintptr(unsafe.Pointer(&devMode[0])),
		dmInBuffer|dmOutBuffer,
	)
	if int32(result) != 1 {
		return nil, win32CallError("DocumentPropertiesW(merge)", callErr)
	}
	return devMode, nil
}

func getDeviceCaps(hdc uintptr, index uintptr) int {
	value, _, _ := getDeviceCapsProc.Call(hdc, index)
	return int(int32(value))
}

func win32CallError(operation string, callErr error) error {
	if callErr != nil && callErr != syscall.Errno(0) {
		return fmt.Errorf("%s: %w", operation, callErr)
	}
	return fmt.Errorf("%s failed", operation)
}
