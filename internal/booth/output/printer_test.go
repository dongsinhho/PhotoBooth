package output

import (
	"encoding/binary"
	"image"
	"image/color"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestPreparePrinterPixelsUsesBGRAAndWhiteBackground(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	src.SetNRGBA(0, 0, color.NRGBA{R: 255, G: 0, B: 0, A: 255})
	src.SetNRGBA(1, 0, color.NRGBA{R: 0, G: 255, B: 0, A: 255})
	src.SetNRGBA(0, 1, color.NRGBA{R: 0, G: 0, B: 255, A: 255})
	src.SetNRGBA(1, 1, color.NRGBA{R: 0, G: 0, B: 0, A: 0})

	pixels, width, height, err := preparePrinterPixels(src, false)
	if err != nil {
		t.Fatalf("preparePrinterPixels: %v", err)
	}
	if width != 2 || height != 2 {
		t.Fatalf("dimensions = %dx%d, want 2x2", width, height)
	}
	want := []byte{
		0, 0, 255, 255, 0, 255, 0, 255,
		255, 0, 0, 255, 255, 255, 255, 255,
	}
	if len(pixels) != len(want) {
		t.Fatalf("pixel length = %d, want %d", len(pixels), len(want))
	}
	for i := range want {
		if pixels[i] != want[i] {
			t.Fatalf("pixel byte %d = %d, want %d", i, pixels[i], want[i])
		}
	}
}

func TestPreparePrinterPixelsFlipVertical(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 1, 2))
	src.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	src.SetNRGBA(0, 1, color.NRGBA{B: 255, A: 255})

	pixels, width, height, err := preparePrinterPixels(src, true)
	if err != nil {
		t.Fatalf("preparePrinterPixels: %v", err)
	}
	if width != 1 || height != 2 {
		t.Fatalf("dimensions = %dx%d, want 1x2", width, height)
	}
	// First output row must contain the original bottom blue pixel.
	if got := pixels[:4]; got[0] != 255 || got[1] != 0 || got[2] != 0 || got[3] != 255 {
		t.Fatalf("flipped first row BGRA = %v, want blue", got)
	}
	// Second output row must contain the original top red pixel.
	if got := pixels[4:8]; got[0] != 0 || got[1] != 0 || got[2] != 255 || got[3] != 255 {
		t.Fatalf("flipped second row BGRA = %v, want red", got)
	}
}

func TestNewWindowsPrinterRequiresQueueName(t *testing.T) {
	if _, err := NewWindowsPrinter("  ", WindowsPrinterOptions{}); err == nil {
		t.Fatal("expected an empty queue name to fail")
	}
	target, err := NewWindowsPrinter("  DS-RX1  ", WindowsPrinterOptions{Landscape: true, FlipVertical: true})
	if err != nil {
		t.Fatalf("NewWindowsPrinter: %v", err)
	}
	if target.Name() != TargetPrinter || target.PrinterName != "DS-RX1" {
		t.Fatalf("unexpected printer target: %+v", target)
	}
}

func TestApplyWindowsDevModeOptionsSetsOrientationAndNamedMediaForm(t *testing.T) {
	devMode := make([]byte, 256)
	binary.LittleEndian.PutUint16(devMode[devmodePublicSizeOffset:devmodePublicSizeOffset+2], 220)
	binary.LittleEndian.PutUint32(devMode[devmodeFieldsOffset:devmodeFieldsOffset+4], devmodeFieldOrientation)
	for index := range devMode {
		if index >= 220 {
			devMode[index] = 0xA5 // bytes outside dmSize must not be touched
		}
	}
	if err := applyWindowsDevModeOptions(devMode, true, "DS-RX1 6x4"); err != nil {
		t.Fatalf("applyWindowsDevModeOptions: %v", err)
	}
	if got := binary.LittleEndian.Uint16(devMode[devmodeOrientationOffset : devmodeOrientationOffset+2]); got != 2 {
		t.Fatalf("orientation=%d, want landscape(2)", got)
	}
	fields := binary.LittleEndian.Uint32(devMode[devmodeFieldsOffset : devmodeFieldsOffset+4])
	if fields&devmodeFieldOrientation == 0 || fields&devmodeFieldFormName == 0 {
		t.Fatalf("dmFields=%#x, orientation and form-name flags must be set", fields)
	}
	want := utf16.Encode([]rune("DS-RX1 6x4"))
	for index, value := range want {
		offset := devmodeFormNameOffset + index*2
		if got := binary.LittleEndian.Uint16(devMode[offset : offset+2]); got != value {
			t.Fatalf("form-name unit %d=%#x want %#x", index, got, value)
		}
	}
	if got := binary.LittleEndian.Uint16(devMode[devmodeFormNameOffset+len(want)*2 : devmodeFormNameOffset+(len(want)+1)*2]); got != 0 {
		t.Fatalf("form name is not NUL terminated: %#x", got)
	}
	if devMode[220] != 0xA5 {
		t.Fatal("vendor-private tail outside dmSize was modified")
	}
}

func TestApplyWindowsDevModeOptionsPreservesDefaultMediaWhenUnset(t *testing.T) {
	devMode := make([]byte, 220)
	binary.LittleEndian.PutUint16(devMode[devmodePublicSizeOffset:devmodePublicSizeOffset+2], 220)
	binary.LittleEndian.PutUint32(devMode[devmodeFieldsOffset:devmodeFieldsOffset+4], devmodeFieldOrientation)
	binary.LittleEndian.PutUint16(devMode[devmodeFormNameOffset:devmodeFormNameOffset+2], 'A')
	if err := applyWindowsDevModeOptions(devMode, false, ""); err != nil {
		t.Fatalf("applyWindowsDevModeOptions: %v", err)
	}
	fields := binary.LittleEndian.Uint32(devMode[devmodeFieldsOffset : devmodeFieldsOffset+4])
	if fields&devmodeFieldFormName != 0 {
		t.Fatalf("default media unexpectedly enabled DM_FORMNAME: %#x", fields)
	}
	if got := binary.LittleEndian.Uint16(devMode[devmodeFormNameOffset : devmodeFormNameOffset+2]); got != 'A' {
		t.Fatalf("default form name changed to %#x", got)
	}
}

func TestApplyWindowsDevModeOptionsRejectsUnsupportedFormFieldAndLongNames(t *testing.T) {
	short := make([]byte, 160)
	binary.LittleEndian.PutUint16(short[devmodePublicSizeOffset:devmodePublicSizeOffset+2], uint16(len(short)))
	binary.LittleEndian.PutUint32(short[devmodeFieldsOffset:devmodeFieldsOffset+4], devmodeFieldOrientation)
	if err := applyWindowsDevModeOptions(short, false, "6x4"); err == nil {
		t.Fatal("expected truncated/unsupported form-name field to fail")
	}

	devMode := make([]byte, 220)
	binary.LittleEndian.PutUint16(devMode[devmodePublicSizeOffset:devmodePublicSizeOffset+2], 220)
	binary.LittleEndian.PutUint32(devMode[devmodeFieldsOffset:devmodeFieldsOffset+4], devmodeFieldOrientation)
	if err := applyWindowsDevModeOptions(devMode, false, strings.Repeat("X", devmodeFormNameChars)); err == nil {
		t.Fatal("expected form names that cannot fit with a NUL terminator to fail")
	}
}
