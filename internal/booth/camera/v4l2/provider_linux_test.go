//go:build linux && amd64

package v4l2

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIOCTLRequestNumbersMatchLinuxV4L2ABI(t *testing.T) {
	cases := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{name: "query capability", got: vidiocQueryCap, want: 0x80685600},
		{name: "get format", got: vidiocGFormat, want: 0xc0d05604},
		{name: "set format", got: vidiocSFormat, want: 0xc0d05605},
		{name: "request buffers", got: vidiocReqBufs, want: 0xc0145608},
		{name: "query buffer", got: vidiocQueryBuf, want: 0xc0585609},
		{name: "dequeue buffer", got: vidiocDQBuf, want: 0xc0585611},
		{name: "queue buffer", got: vidiocQBuf, want: 0xc058560f},
		{name: "stream on", got: vidiocStreamOn, want: 0x40045612},
		{name: "stream off", got: vidiocStreamOff, want: 0x40045613},
		{name: "set stream parameters", got: vidiocSParm, want: 0xc0cc5616},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("ioctl request = %#x, want %#x", tc.got, tc.want)
			}
		})
	}
}

func TestNewV4L2BufferInitializesRequiredFields(t *testing.T) {
	buffer := newV4L2Buffer(3)
	if len(buffer) != v4l2BufferSize {
		t.Fatalf("buffer size = %d, want %d", len(buffer), v4l2BufferSize)
	}
	if got := binary.LittleEndian.Uint32(buffer[0:4]); got != 3 {
		t.Fatalf("index = %d, want 3", got)
	}
	if got := binary.LittleEndian.Uint32(buffer[4:8]); got != v4l2BufferTypeVideoCapture {
		t.Fatalf("type = %d, want video capture", got)
	}
	if got := binary.LittleEndian.Uint32(buffer[60:64]); got != v4l2MemoryMmap {
		t.Fatalf("memory = %d, want MMAP", got)
	}
}

func TestFourCCMJPEGEncoding(t *testing.T) {
	if got, want := fourCCString(fourCCMJPEG), "MJPG"; got != want {
		t.Fatalf("fourCCString(MJPG) = %q, want %q", got, want)
	}
}

func TestWarmupFrameCountIsBounded(t *testing.T) {
	cases := []struct {
		fps      float64
		duration time.Duration
		want     int
	}{
		{fps: 15, duration: 0, want: 0},
		{fps: 15, duration: time.Second, want: 15},
		{fps: 12.4, duration: time.Second, want: 13},
		{fps: 30, duration: 10 * time.Second, want: 120},
	}
	for _, tc := range cases {
		if got := warmupFrameCount(tc.fps, tc.duration); got != tc.want {
			t.Errorf("warmupFrameCount(%v, %s) = %d, want %d", tc.fps, tc.duration, got, tc.want)
		}
	}
}

func TestValidateOptionsRejectsInvalidSettings(t *testing.T) {
	cases := []struct {
		name    string
		options Options
	}{
		{name: "non-integer FPS", options: Options{DevicePath: "/dev/video0", Width: 640, Height: 480, FPS: 14.5, BufferCount: 4, FrameTimeout: time.Second}},
		{name: "too few buffers", options: Options{DevicePath: "/dev/video0", Width: 640, Height: 480, FPS: 15, BufferCount: 1, FrameTimeout: time.Second}},
		{name: "too many buffers", options: Options{DevicePath: "/dev/video0", Width: 640, Height: 480, FPS: 15, BufferCount: 9, FrameTimeout: time.Second}},
		{name: "zero timeout", options: Options{DevicePath: "/dev/video0", Width: 640, Height: 480, FPS: 15, BufferCount: 4}},
		{name: "negative warmup", options: Options{DevicePath: "/dev/video0", Width: 640, Height: 480, FPS: 15, BufferCount: 4, FrameTimeout: time.Second, WarmupDuration: -time.Second}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			backend := newBackend(tc.options).(*linuxBackend)
			if err := backend.validateOptions(); err == nil {
				t.Fatal("expected invalid options to be rejected")
			}
		})
	}
}

func TestWriteJPEGExclusiveDoesNotReplaceExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "capture.jpg")
	original := []byte("keep-existing-capture")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	err := writeJPEGExclusive(context.Background(), path, validJPEG(t))
	if err == nil {
		t.Fatal("expected existing capture path to be rejected")
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("existing file changed: %q", got)
	}
}

func TestWriteJPEGExclusiveWritesDecodableJPEG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.jpg")
	if err := writeJPEGExclusive(context.Background(), path, validJPEG(t)); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decoded, _, err := image.Decode(file)
	if err != nil {
		t.Fatalf("decode captured JPEG: %v", err)
	}
	if decoded.Bounds().Dx() != 32 || decoded.Bounds().Dy() != 24 {
		t.Fatalf("decoded dimensions = %v, want 32x24", decoded.Bounds())
	}
}

func TestWriteJPEGExclusiveHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	path := filepath.Join(t.TempDir(), "capture.jpg")
	if err := writeJPEGExclusive(ctx, path, validJPEG(t)); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled capture left output behind: %v", err)
	}
}

func validJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 32, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 7), G: uint8(y * 9), B: 90, A: 255})
		}
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestValidateMJPEGFrameAcceptsExpectedJPEG(t *testing.T) {
	data, width, height, err := validateMJPEGFrame(validJPEG(t), 32, 24)
	if err != nil {
		t.Fatalf("validateMJPEGFrame() error = %v", err)
	}
	if width != 32 || height != 24 {
		t.Fatalf("dimensions = %dx%d, want 32x24", width, height)
	}
	if len(data) == 0 {
		t.Fatal("validated JPEG is empty")
	}
}

func TestValidateMJPEGFrameRejectsWrongDimensions(t *testing.T) {
	if _, _, _, err := validateMJPEGFrame(validJPEG(t), 640, 480); err == nil {
		t.Fatal("expected wrong frame dimensions to be rejected")
	}
}

func TestValidateMJPEGFrameRejectsMalformedJPEG(t *testing.T) {
	if _, _, _, err := validateMJPEGFrame([]byte{0xff, 0xd8, 0x00, 0x01}, 32, 24); err == nil {
		t.Fatal("expected malformed JPEG to be rejected")
	}
}
