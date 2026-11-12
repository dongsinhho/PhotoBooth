//go:build linux && amd64

package v4l2

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image/jpeg"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/dongsinhho/PhotoBooth/internal/booth/camera"
	"github.com/dongsinhho/PhotoBooth/internal/booth/camera/mjpeg"
)

const (
	v4l2IOCType = uint32('V')

	iocWrite = uint32(1)
	iocRead  = uint32(2)

	v4l2BufferTypeVideoCapture = uint32(1)
	v4l2MemoryMmap             = uint32(1)

	v4l2CapabilityDeviceCaps   = uint32(0x80000000)
	v4l2CapabilityVideoCapture = uint32(0x00000001)
	v4l2CapabilityStreaming    = uint32(0x04000000)

	v4l2BufferFlagError = uint32(0x00000040)

	pollIn   = int16(0x0001)
	pollErr  = int16(0x0008)
	pollHup  = int16(0x0010)
	pollNval = int16(0x0020)

	maxJPEGFrameSize        = 40 << 20
	minMMAPBuffers          = 2
	maxMMAPBuffers          = 8
	maxConsecutiveBadFrames = 5
	pollInterval            = 100 * time.Millisecond
)

const (
	v4l2CapabilitySize     = 104
	v4l2FormatSize         = 208
	v4l2RequestBuffersSize = 20
	v4l2BufferSize         = 88
	v4l2StreamParmSize     = 204
)

var (
	vidiocQueryCap  = ioctlRequest(iocRead, 0, v4l2CapabilitySize)
	vidiocGFormat   = ioctlRequest(iocRead|iocWrite, 4, v4l2FormatSize)
	vidiocSFormat   = ioctlRequest(iocRead|iocWrite, 5, v4l2FormatSize)
	vidiocReqBufs   = ioctlRequest(iocRead|iocWrite, 8, v4l2RequestBuffersSize)
	vidiocQueryBuf  = ioctlRequest(iocRead|iocWrite, 9, v4l2BufferSize)
	vidiocQBuf      = ioctlRequest(iocRead|iocWrite, 15, v4l2BufferSize)
	vidiocDQBuf     = ioctlRequest(iocRead|iocWrite, 17, v4l2BufferSize)
	vidiocStreamOn  = ioctlRequest(iocWrite, 18, 4)
	vidiocStreamOff = ioctlRequest(iocWrite, 19, 4)
	vidiocGParm     = ioctlRequest(iocRead|iocWrite, 21, v4l2StreamParmSize)
	vidiocSParm     = ioctlRequest(iocRead|iocWrite, 22, v4l2StreamParmSize)
)

type pollDescriptor struct {
	FD      int32
	Events  int16
	Revents int16
}

type mappedBuffer struct {
	data []byte
}

type linuxBackend struct {
	options Options

	opMu sync.Mutex
	mu   sync.RWMutex
	info camera.Info

	fd               int
	streaming        bool
	buffersAllocated bool
	buffers          []mappedBuffer
	width            int
	height           int
	sizeImage        int
	actualFPS        float64
	name             string
}

var _ backend = (*linuxBackend)(nil)

func newBackend(options Options) backend {
	return &linuxBackend{
		options: options,
		fd:      -1,
		info: camera.Info{
			ID: options.DevicePath, Name: "Linux V4L2 camera",
			OS: "linux", Backend: "v4l2",
		},
	}
}

func (b *linuxBackend) Info() camera.Info {
	b.mu.RLock()
	defer b.mu.RUnlock()
	info := b.info
	info.Capabilities.Resolution = append([]camera.Resolution(nil), info.Capabilities.Resolution...)
	return info
}

func (b *linuxBackend) Open(ctx context.Context) error {
	b.opMu.Lock()
	defer b.opMu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}
	if b.fd >= 0 {
		return errors.New("Linux V4L2 camera is already open")
	}
	if err := b.validateOptions(); err != nil {
		return err
	}

	fileInfo, err := os.Stat(b.options.DevicePath)
	if err != nil {
		return fmt.Errorf("stat V4L2 device %q: %w", b.options.DevicePath, err)
	}
	if fileInfo.Mode()&os.ModeDevice == 0 || fileInfo.Mode()&os.ModeCharDevice == 0 {
		return fmt.Errorf("V4L2 device %q is not a character device", b.options.DevicePath)
	}

	fd, err := syscall.Open(b.options.DevicePath, syscall.O_RDWR|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open V4L2 device %q: %w", b.options.DevicePath, err)
	}
	b.fd = fd

	fail := func(cause error) error {
		closeErr := b.closeLocked()
		if closeErr != nil {
			return errors.Join(cause, fmt.Errorf("clean up failed V4L2 open: %w", closeErr))
		}
		return cause
	}

	capability := make([]byte, v4l2CapabilitySize)
	if err := ioctl(fd, vidiocQueryCap, capability); err != nil {
		return fail(fmt.Errorf("query V4L2 capabilities: %w", err))
	}
	capabilities := binary.LittleEndian.Uint32(capability[84:88])
	deviceCapabilities := binary.LittleEndian.Uint32(capability[88:92])
	if capabilities&v4l2CapabilityDeviceCaps != 0 {
		capabilities = deviceCapabilities
	}
	if capabilities&v4l2CapabilityVideoCapture == 0 {
		return fail(fmt.Errorf("V4L2 node %q does not support single-planar video capture (it may be a metadata-only node)", b.options.DevicePath))
	}
	if capabilities&v4l2CapabilityStreaming == 0 {
		return fail(fmt.Errorf("V4L2 node %q does not support streaming I/O", b.options.DevicePath))
	}
	b.name = strings.TrimRight(string(capability[16:48]), "\x00")

	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	format := make([]byte, v4l2FormatSize)
	binary.LittleEndian.PutUint32(format[0:4], v4l2BufferTypeVideoCapture)
	// The union begins at byte 8 on 64-bit Linux because it contains pointer-
	// aligned members such as v4l2_window. The four bytes at 4:8 are padding.
	binary.LittleEndian.PutUint32(format[8:12], uint32(b.options.Width))
	binary.LittleEndian.PutUint32(format[12:16], uint32(b.options.Height))
	binary.LittleEndian.PutUint32(format[16:20], fourCCMJPEG)
	// V4L2_FIELD_ANY lets the driver select its normal progressive/field mode.
	binary.LittleEndian.PutUint32(format[20:24], 0)
	if err := ioctl(fd, vidiocSFormat, format); err != nil {
		return fail(fmt.Errorf("negotiate %dx%d MJPEG format: %w", b.options.Width, b.options.Height, err))
	}
	b.width = int(binary.LittleEndian.Uint32(format[8:12]))
	b.height = int(binary.LittleEndian.Uint32(format[12:16]))
	pixelFormat := binary.LittleEndian.Uint32(format[16:20])
	b.sizeImage = int(binary.LittleEndian.Uint32(format[28:32]))
	if b.width != b.options.Width || b.height != b.options.Height || pixelFormat != fourCCMJPEG {
		return fail(fmt.Errorf(
			"V4L2 negotiated %dx%d pixel format %q; exact %dx%d MJPEG was requested",
			b.width, b.height, fourCCString(pixelFormat), b.options.Width, b.options.Height,
		))
	}
	if b.sizeImage <= 0 || b.sizeImage > maxJPEGFrameSize {
		return fail(fmt.Errorf("V4L2 returned unsupported sizeimage %d", b.sizeImage))
	}

	if err := b.setFrameRate(); err != nil {
		return fail(err)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if err := b.allocateMMAPBuffers(); err != nil {
		return fail(err)
	}
	streamType := v4l2BufferTypeVideoCapture
	if err := ioctlValue(fd, vidiocStreamOn, &streamType); err != nil {
		return fail(fmt.Errorf("start V4L2 stream: %w", err))
	}
	b.streaming = true

	info := camera.Info{
		ID:      b.options.DevicePath,
		Name:    b.name,
		OS:      "linux",
		Backend: "v4l2",
		Capabilities: camera.Capabilities{
			Preview:                true,
			FrameCaptureFromStream: true,
			Resolution:             []camera.Resolution{{Width: b.width, Height: b.height}},
		},
	}
	if info.Name == "" {
		info.Name = "V4L2 camera"
	}
	b.mu.Lock()
	b.info = info
	b.mu.Unlock()

	if !b.options.SkipWarmup {
		frames := warmupFrameCount(b.options.FPS, b.options.WarmupDuration)
		for index := 0; index < frames; index++ {
			if _, _, _, err := b.readValidatedFrame(ctx); err != nil {
				return fail(fmt.Errorf("warm up V4L2 stream frame %d/%d: %w", index+1, frames, err))
			}
		}
	}
	return nil
}

func (b *linuxBackend) validateOptions() error {
	if strings.TrimSpace(b.options.DevicePath) == "" {
		return errors.New("V4L2 device path is required")
	}
	if b.options.Width < 1 || b.options.Width > 4096 || b.options.Height < 1 || b.options.Height > 4096 {
		return errors.New("V4L2 width and height must be between 1 and 4096")
	}
	if math.IsNaN(b.options.FPS) || math.IsInf(b.options.FPS, 0) || b.options.FPS <= 0 || b.options.FPS > 120 {
		return errors.New("V4L2 FPS must be greater than 0 and at most 120")
	}
	if math.Abs(b.options.FPS-math.Round(b.options.FPS)) > 0.001 {
		return errors.New("experimental V4L2 provider currently requires an integer FPS")
	}
	if b.options.BufferCount < minMMAPBuffers || b.options.BufferCount > maxMMAPBuffers {
		return fmt.Errorf("V4L2 MMAP buffer count must be between %d and %d", minMMAPBuffers, maxMMAPBuffers)
	}
	if b.options.FrameTimeout <= 0 {
		return errors.New("V4L2 frame timeout must be positive")
	}
	if b.options.WarmupDuration < 0 || b.options.WarmupDuration > 3*time.Second {
		return errors.New("V4L2 warm-up duration must be between 0 and 3 seconds")
	}
	return nil
}

func (b *linuxBackend) setFrameRate() error {
	parameters := make([]byte, v4l2StreamParmSize)
	binary.LittleEndian.PutUint32(parameters[0:4], v4l2BufferTypeVideoCapture)
	binary.LittleEndian.PutUint32(parameters[12:16], 1)
	binary.LittleEndian.PutUint32(parameters[16:20], uint32(math.Round(b.options.FPS)))
	if err := ioctl(b.fd, vidiocSParm, parameters); err != nil {
		return fmt.Errorf("set V4L2 frame rate to %.3g FPS: %w", b.options.FPS, err)
	}

	parameters = make([]byte, v4l2StreamParmSize)
	binary.LittleEndian.PutUint32(parameters[0:4], v4l2BufferTypeVideoCapture)
	if err := ioctl(b.fd, vidiocGParm, parameters); err != nil {
		return fmt.Errorf("read negotiated V4L2 frame rate: %w", err)
	}
	numerator := binary.LittleEndian.Uint32(parameters[12:16])
	denominator := binary.LittleEndian.Uint32(parameters[16:20])
	if numerator == 0 || denominator == 0 {
		return errors.New("V4L2 returned an invalid frame interval")
	}
	b.actualFPS = float64(denominator) / float64(numerator)
	if math.Abs(b.actualFPS-b.options.FPS) > 0.5 {
		return fmt.Errorf("V4L2 negotiated %.3g FPS; requested %.3g FPS", b.actualFPS, b.options.FPS)
	}
	return nil
}

func (b *linuxBackend) allocateMMAPBuffers() error {
	request := make([]byte, v4l2RequestBuffersSize)
	binary.LittleEndian.PutUint32(request[0:4], uint32(b.options.BufferCount))
	binary.LittleEndian.PutUint32(request[4:8], v4l2BufferTypeVideoCapture)
	binary.LittleEndian.PutUint32(request[8:12], v4l2MemoryMmap)
	if err := ioctl(b.fd, vidiocReqBufs, request); err != nil {
		return fmt.Errorf("request V4L2 MMAP buffers: %w", err)
	}
	b.buffersAllocated = true
	count := int(binary.LittleEndian.Uint32(request[0:4]))
	if count < minMMAPBuffers || count > maxMMAPBuffers {
		return fmt.Errorf("V4L2 allocated %d MMAP buffers; expected between %d and %d", count, minMMAPBuffers, maxMMAPBuffers)
	}
	b.buffers = make([]mappedBuffer, 0, count)

	for index := 0; index < count; index++ {
		buffer := newV4L2Buffer(uint32(index))
		if err := ioctl(b.fd, vidiocQueryBuf, buffer); err != nil {
			return fmt.Errorf("query V4L2 MMAP buffer %d: %w", index, err)
		}
		length := int(binary.LittleEndian.Uint32(buffer[72:76]))
		offset := int64(binary.LittleEndian.Uint32(buffer[64:68]))
		if length < b.sizeImage || length <= 0 || length > maxJPEGFrameSize {
			return fmt.Errorf("V4L2 MMAP buffer %d has invalid length %d (sizeimage %d)", index, length, b.sizeImage)
		}
		mapped, err := syscall.Mmap(b.fd, offset, length, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
		if err != nil {
			return fmt.Errorf("map V4L2 buffer %d: %w", index, err)
		}
		b.buffers = append(b.buffers, mappedBuffer{data: mapped})
		if err := ioctl(b.fd, vidiocQBuf, newV4L2Buffer(uint32(index))); err != nil {
			return fmt.Errorf("queue V4L2 buffer %d: %w", index, err)
		}
	}
	return nil
}

func (b *linuxBackend) Capture(ctx context.Context, req camera.CaptureRequest) (camera.CaptureResult, error) {
	b.opMu.Lock()
	defer b.opMu.Unlock()

	if err := ctx.Err(); err != nil {
		return camera.CaptureResult{}, err
	}
	if b.fd < 0 || !b.streaming {
		return camera.CaptureResult{}, errors.New("Linux V4L2 camera is not open")
	}
	if req.Sequence < 1 {
		return camera.CaptureResult{}, errors.New("capture sequence must be positive")
	}
	if strings.TrimSpace(req.OutputPath) == "" {
		return camera.CaptureResult{}, errors.New("capture output path is required")
	}
	if req.Width > 0 && req.Width != b.width || req.Height > 0 && req.Height != b.height {
		return camera.CaptureResult{}, fmt.Errorf(
			"V4L2 stream is configured for %dx%d; capture requested %dx%d; reopen with the desired stream settings",
			b.width, b.height, req.Width, req.Height,
		)
	}
	data, width, height, err := b.readValidatedFrame(ctx)
	if err != nil {
		return camera.CaptureResult{}, err
	}
	if err := writeJPEGExclusive(ctx, req.OutputPath, data); err != nil {
		return camera.CaptureResult{}, err
	}
	return camera.CaptureResult{Sequence: req.Sequence, Path: req.OutputPath, Width: width, Height: height}, nil
}

func (b *linuxBackend) Preview(ctx context.Context) ([]byte, error) {
	b.opMu.Lock()
	defer b.opMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if b.fd < 0 || !b.streaming {
		return nil, errors.New("Linux V4L2 camera is not open")
	}
	data, _, _, err := b.readValidatedFrame(ctx)
	return data, err
}

func (b *linuxBackend) readValidatedFrame(ctx context.Context) ([]byte, int, int, error) {
	frameCtx, cancel := context.WithTimeout(ctx, b.options.FrameTimeout)
	defer cancel()

	var lastFrameError error
	for attempt := 1; attempt <= maxConsecutiveBadFrames; attempt++ {
		raw, err := b.dequeueFrame(frameCtx)
		if err != nil {
			return nil, 0, 0, err
		}
		data, width, height, err := validateMJPEGFrame(raw, b.width, b.height)
		if err == nil {
			return data, width, height, nil
		}
		lastFrameError = err
		if frameCtx.Err() != nil {
			return nil, 0, 0, frameCtx.Err()
		}
	}
	return nil, 0, 0, fmt.Errorf(
		"%w: discarded %d consecutive invalid MJPEG buffers from %s; last frame error: %v",
		camera.ErrStreamUnavailable, maxConsecutiveBadFrames, b.options.DevicePath, lastFrameError,
	)
}

func validateMJPEGFrame(raw []byte, width, height int) ([]byte, int, int, error) {
	if len(raw) < 4 || len(raw) > maxJPEGFrameSize {
		return nil, 0, 0, fmt.Errorf("V4L2 JPEG frame size %d is outside allowed limits", len(raw))
	}
	data, err := mjpeg.Normalize(raw)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("normalize V4L2 MJPEG frame: %w", err)
	}
	header, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("V4L2 stream did not yield a valid JPEG frame: %w", err)
	}
	if header.Width != width || header.Height != height {
		return nil, 0, 0, fmt.Errorf("V4L2 frame is %dx%d; negotiated format is %dx%d", header.Width, header.Height, width, height)
	}
	return data, header.Width, header.Height, nil
}

func (b *linuxBackend) dequeueFrame(ctx context.Context) ([]byte, error) {
	deadline := time.Now().Add(b.options.FrameTimeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	consecutiveErrors := 0

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, fmt.Errorf("%w: timed out waiting for a V4L2 frame from %s", camera.ErrStreamUnavailable, b.options.DevicePath)
		}
		wait := pollInterval
		if remaining < wait {
			wait = remaining
		}
		if err := b.waitReadable(wait); err != nil {
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			return nil, fmt.Errorf("%w: poll V4L2 device %s: %w", camera.ErrStreamUnavailable, b.options.DevicePath, err)
		}

		buffer := newV4L2Buffer(0)
		if err := ioctl(b.fd, vidiocDQBuf, buffer); err != nil {
			if errors.Is(err, syscall.EAGAIN) {
				continue
			}
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			return nil, fmt.Errorf("%w: dequeue V4L2 buffer from %s: %w", camera.ErrStreamUnavailable, b.options.DevicePath, err)
		}
		index := int(binary.LittleEndian.Uint32(buffer[0:4]))
		if index < 0 || index >= len(b.buffers) {
			return nil, fmt.Errorf("%w: driver returned invalid V4L2 buffer index %d", camera.ErrStreamUnavailable, index)
		}
		flags := binary.LittleEndian.Uint32(buffer[12:16])
		bytesUsed := int(binary.LittleEndian.Uint32(buffer[8:12]))
		if flags&v4l2BufferFlagError != 0 {
			if err := b.queueBuffer(uint32(index)); err != nil {
				return nil, fmt.Errorf("%w: requeue V4L2 error buffer: %w", camera.ErrStreamUnavailable, err)
			}
			consecutiveErrors++
			if consecutiveErrors >= maxConsecutiveBadFrames {
				return nil, fmt.Errorf("%w: %d consecutive V4L2 buffers were marked V4L2_BUF_FLAG_ERROR", camera.ErrStreamUnavailable, consecutiveErrors)
			}
			continue
		}
		if bytesUsed <= 0 || bytesUsed > len(b.buffers[index].data) {
			_ = b.queueBuffer(uint32(index))
			return nil, fmt.Errorf("%w: V4L2 buffer %d reported invalid bytesused %d", camera.ErrStreamUnavailable, index, bytesUsed)
		}
		// Copy before QBUF: after requeue the driver owns this MMAP memory again.
		frame := append([]byte(nil), b.buffers[index].data[:bytesUsed]...)
		if err := b.queueBuffer(uint32(index)); err != nil {
			return nil, fmt.Errorf("%w: requeue V4L2 buffer %d: %w", camera.ErrStreamUnavailable, index, err)
		}
		consecutiveErrors = 0
		return frame, nil
	}
}

func (b *linuxBackend) waitReadable(timeout time.Duration) error {
	pfd := pollDescriptor{FD: int32(b.fd), Events: pollIn}
	milliseconds := int(timeout / time.Millisecond)
	if milliseconds < 1 {
		milliseconds = 1
	}
	result, _, errno := syscall.Syscall(
		syscall.SYS_POLL,
		uintptr(unsafe.Pointer(&pfd)),
		1,
		uintptr(milliseconds),
	)
	runtime.KeepAlive(&pfd)
	if errno != 0 {
		return errno
	}
	if result == 0 {
		return nil
	}
	if pfd.Revents&(pollErr|pollHup|pollNval) != 0 {
		return fmt.Errorf("poll reported device revents %#x", uint16(pfd.Revents))
	}
	return nil
}

func (b *linuxBackend) queueBuffer(index uint32) error {
	return ioctl(b.fd, vidiocQBuf, newV4L2Buffer(index))
}

func (b *linuxBackend) Close() error {
	b.opMu.Lock()
	defer b.opMu.Unlock()
	return b.closeLocked()
}

func (b *linuxBackend) closeLocked() error {
	if b.fd < 0 {
		return nil
	}
	var cleanupErrors []error
	if b.streaming {
		streamType := v4l2BufferTypeVideoCapture
		if err := ioctlValue(b.fd, vidiocStreamOff, &streamType); err != nil && !errors.Is(err, syscall.ENODEV) && !errors.Is(err, syscall.EINVAL) {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("stop V4L2 stream: %w", err))
		}
		b.streaming = false
	}
	for index := range b.buffers {
		if len(b.buffers[index].data) == 0 {
			continue
		}
		if err := syscall.Munmap(b.buffers[index].data); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("unmap V4L2 buffer %d: %w", index, err))
		}
		b.buffers[index].data = nil
	}
	b.buffers = nil
	if b.buffersAllocated {
		request := make([]byte, v4l2RequestBuffersSize)
		binary.LittleEndian.PutUint32(request[4:8], v4l2BufferTypeVideoCapture)
		binary.LittleEndian.PutUint32(request[8:12], v4l2MemoryMmap)
		if err := ioctl(b.fd, vidiocReqBufs, request); err != nil && !errors.Is(err, syscall.ENODEV) && !errors.Is(err, syscall.EINVAL) {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("release V4L2 MMAP buffers: %w", err))
		}
		b.buffersAllocated = false
	}
	if err := syscall.Close(b.fd); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("close V4L2 device: %w", err))
	}
	b.fd = -1
	b.width, b.height, b.sizeImage, b.actualFPS = 0, 0, 0, 0
	return errors.Join(cleanupErrors...)
}

func writeJPEGExclusive(ctx context.Context, outputPath string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(outputPath) == "" {
		return errors.New("capture output path is required")
	}
	file, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create capture staging file without replacement: %w", err)
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.Remove(outputPath)
		}
	}()
	for written := 0; written < len(data); {
		if err := ctx.Err(); err != nil {
			_ = file.Close()
			return err
		}
		n, writeErr := file.Write(data[written:])
		written += n
		if writeErr != nil {
			_ = file.Close()
			return fmt.Errorf("write capture staging file: %w", writeErr)
		}
		if n == 0 {
			_ = file.Close()
			return errors.New("write capture staging file: zero-byte write")
		}
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync capture staging file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close capture staging file: %w", err)
	}
	keep = true
	return nil
}

func ioctl(fd int, request uintptr, data []byte) error {
	if len(data) == 0 {
		return errors.New("ioctl data buffer is empty")
	}
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		request,
		uintptr(unsafe.Pointer(&data[0])),
	)
	runtime.KeepAlive(data)
	if errno != 0 {
		return errno
	}
	return nil
}

func ioctlValue(fd int, request uintptr, value *uint32) error {
	return ioctl(fd, request, unsafe.Slice((*byte)(unsafe.Pointer(value)), 4))
}

func newV4L2Buffer(index uint32) []byte {
	buffer := make([]byte, v4l2BufferSize)
	binary.LittleEndian.PutUint32(buffer[0:4], index)
	binary.LittleEndian.PutUint32(buffer[4:8], v4l2BufferTypeVideoCapture)
	binary.LittleEndian.PutUint32(buffer[60:64], v4l2MemoryMmap)
	return buffer
}

func ioctlRequest(direction, number, size uint32) uintptr {
	return uintptr((direction << 30) | (size << 16) | (v4l2IOCType << 8) | number)
}

func fourCCString(value uint32) string {
	return string([]byte{
		byte(value), byte(value >> 8), byte(value >> 16), byte(value >> 24),
	})
}

const fourCCMJPEG = uint32('M') | uint32('J')<<8 | uint32('P')<<16 | uint32('G')<<24

func warmupFrameCount(fps float64, duration time.Duration) int {
	if duration <= 0 {
		return 0
	}
	frames := int(math.Ceil(fps * duration.Seconds()))
	if frames < 1 {
		return 1
	}
	if frames > 120 {
		return 120
	}
	return frames
}
