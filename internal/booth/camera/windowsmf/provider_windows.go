//go:build windows

package windowsmf

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/jpeg"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	mf "github.com/Kirizu-Official/windows-camera-go/camera/v1"
	mfconsts "github.com/Kirizu-Official/windows-camera-go/windows/consts"
	"github.com/dongsinhho/PhotoBooth/internal/booth/camera"
)

const (
	mjpgSubtypeToken = "47504a4d"
	maxJPEGFrameSize = 40 << 20
	closeTimeout     = 5 * time.Second
)

type windowsBackend struct {
	options Options
	opMu    sync.Mutex
	mu      sync.RWMutex
	worker  *captureWorker
	info    camera.Info
}

func newBackend(options Options) backend {
	return &windowsBackend{
		options: options,
		info: camera.Info{
			ID: options.DeviceID, Name: "Windows Media Foundation camera",
			OS: runtime.GOOS, Backend: "media-foundation",
		},
	}
}

func (b *windowsBackend) Info() camera.Info {
	b.mu.RLock()
	defer b.mu.RUnlock()
	info := b.info
	info.Capabilities.Resolution = append([]camera.Resolution(nil), info.Capabilities.Resolution...)
	return info
}

func (b *windowsBackend) Open(ctx context.Context) error {
	b.opMu.Lock()
	defer b.opMu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	if b.worker != nil {
		b.mu.Unlock()
		return errors.New("Windows Media Foundation camera is already open or shutting down")
	}
	worker := newCaptureWorker(b.options)
	b.worker = worker
	b.mu.Unlock()

	select {
	case startup := <-worker.ready:
		if startup.err != nil {
			<-worker.done
			b.clearWorker(worker)
			return startup.err
		}
		b.mu.Lock()
		b.info = startup.info
		b.mu.Unlock()
		return nil
	case <-worker.done:
		b.clearWorker(worker)
		return errors.New("Windows Media Foundation worker stopped during startup")
	case <-ctx.Done():
		// COM initialization/open is synchronous and cannot itself be interrupted.
		// Close the provider as soon as startup returns rather than leaking a device.
		go func() {
			startup := <-worker.ready
			if startup.err == nil {
				_ = worker.close()
			} else {
				<-worker.done
			}
			b.clearWorker(worker)
		}()
		return ctx.Err()
	}
}

func (b *windowsBackend) Close() error {
	b.opMu.Lock()
	defer b.opMu.Unlock()

	b.mu.RLock()
	worker := b.worker
	b.mu.RUnlock()
	if worker == nil {
		return nil
	}
	if err := worker.close(); err != nil {
		// Keep the worker reference so callers cannot unknowingly open a second
		// camera while the previous native thread is still shutting down.
		return err
	}
	b.clearWorker(worker)
	return nil
}

func (b *windowsBackend) Capture(ctx context.Context, req camera.CaptureRequest) (camera.CaptureResult, error) {
	b.opMu.Lock()
	defer b.opMu.Unlock()

	if err := ctx.Err(); err != nil {
		return camera.CaptureResult{}, err
	}
	if req.Width > 0 && req.Width != b.options.Width || req.Height > 0 && req.Height != b.options.Height {
		return camera.CaptureResult{}, fmt.Errorf(
			"camera stream is configured for %dx%d; capture requested %dx%d; reopen with the desired stream settings",
			b.options.Width, b.options.Height, req.Width, req.Height)
	}
	if req.Sequence < 1 {
		return camera.CaptureResult{}, errors.New("capture sequence must be positive")
	}
	if strings.TrimSpace(req.OutputPath) == "" {
		return camera.CaptureResult{}, errors.New("capture output path is required")
	}

	b.mu.RLock()
	worker := b.worker
	b.mu.RUnlock()
	if worker == nil {
		return camera.CaptureResult{}, errors.New("Windows Media Foundation camera is not open")
	}

	data, err := worker.nextFrame(ctx)
	if err != nil {
		return camera.CaptureResult{}, fmt.Errorf("read frame from Windows Media Foundation: %w", diagnoseCaptureError(err))
	}
	data, err = normalizeMJPEG(data)
	if err != nil {
		return camera.CaptureResult{}, fmt.Errorf("normalize camera MJPEG frame: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return camera.CaptureResult{}, err
	}
	if len(data) == 0 || len(data) > maxJPEGFrameSize {
		return camera.CaptureResult{}, fmt.Errorf("JPEG frame size %d is outside allowed limits", len(data))
	}
	header, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return camera.CaptureResult{}, fmt.Errorf("Media Foundation stream did not yield a valid MJPEG frame: %w", err)
	}
	if header.Width != b.options.Width || header.Height != b.options.Height {
		return camera.CaptureResult{}, fmt.Errorf(
			"actual frame is %dx%d; configured stream expects %dx%d",
			header.Width, header.Height, b.options.Width, b.options.Height)
	}
	if err := writeJPEGAtomically(ctx, req.OutputPath, data); err != nil {
		return camera.CaptureResult{}, err
	}
	return camera.CaptureResult{
		Sequence: req.Sequence, Path: req.OutputPath, Width: header.Width, Height: header.Height,
	}, nil
}

func (b *windowsBackend) clearWorker(worker *captureWorker) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.worker == worker {
		b.worker = nil
	}
}

func writeJPEGAtomically(ctx context.Context, target string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create capture directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".photobooth-capture-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary capture: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write capture: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync capture: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close capture: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, target); err != nil {
		return fmt.Errorf("finalize capture: %w", err)
	}
	return nil
}

type captureWorker struct {
	commands chan workerCommand
	ready    chan startupResult
	done     chan struct{}
}

type workerCommand struct {
	close bool
	reply chan workerResult
}

type workerResult struct {
	data []byte
	err  error
}

type startupResult struct {
	info camera.Info
	err  error
}

type openedCamera struct {
	device  *mf.Device
	capture *mf.CaptureSync
}

func newCaptureWorker(options Options) *captureWorker {
	worker := &captureWorker{
		commands: make(chan workerCommand),
		ready:    make(chan startupResult, 1),
		done:     make(chan struct{}),
	}
	go worker.run(options)
	return worker
}

func (w *captureWorker) run(options Options) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	initialized := false
	var opened *openedCamera
	defer func() {
		if opened != nil && opened.device != nil {
			opened.device.CloseDevice()
		}
		if initialized {
			mf.Shutdown()
		}
		close(w.done)
	}()

	if err := mf.Init(); err != nil {
		w.ready <- startupResult{err: fmt.Errorf("initialize Windows Media Foundation: %w", err)}
		return
	}
	initialized = true
	devices, err := mf.EnumDevice()
	if err != nil {
		w.ready <- startupResult{err: fmt.Errorf("enumerate Windows camera devices: %w", err)}
		return
	}
	if len(devices) == 0 {
		w.ready <- startupResult{err: errors.New("Windows Media Foundation found no camera devices")}
		return
	}

	var selectedDevice *mf.DeviceInfo
	selector := strings.TrimSpace(options.DeviceID)
	for _, device := range devices {
		if selector == "" || strings.EqualFold(selector, device.Name) || selector == device.SymbolLink {
			selectedDevice = device
			break
		}
	}
	if selectedDevice == nil {
		w.ready <- startupResult{err: fmt.Errorf("camera device %q was not found", selector)}
		return
	}

	device, err := mf.OpenDevice(selectedDevice.SymbolLink)
	if err != nil {
		w.ready <- startupResult{err: fmt.Errorf("open camera %q: %w", selectedDevice.Name, err)}
		return
	}
	opened = &openedCamera{device: device}
	formats, err := device.EnumerateCaptureFormats()
	if err != nil {
		w.ready <- startupResult{err: fmt.Errorf("enumerate formats for %q: %w", selectedDevice.Name, err)}
		return
	}
	format, resolutions, err := selectMJPEGFormat(formats, options.Width, options.Height, options.FPS)
	if err != nil {
		w.ready <- startupResult{err: fmt.Errorf("select camera stream for %q: %w", selectedDevice.Name, err)}
		return
	}
	capture, err := device.StartCapture(format)
	if err != nil {
		w.ready <- startupResult{err: fmt.Errorf("start %dx%d MJPEG stream: %w", format.Width, format.Height, err)}
		return
	}
	opened.capture = capture
	info := camera.Info{
		ID: selectedDevice.SymbolLink, Name: selectedDevice.Name, OS: runtime.GOOS, Backend: "media-foundation",
		Capabilities: camera.Capabilities{
			FrameCaptureFromStream: true,
			Resolution:             resolutions,
		},
	}
	w.ready <- startupResult{info: info}

	for {
		command := <-w.commands
		if command.close {
			command.reply <- workerResult{}
			return
		}
		data, err := readMJPEGFrame(opened)
		command.reply <- workerResult{data: data, err: err}
	}
}

func (w *captureWorker) nextFrame(ctx context.Context) ([]byte, error) {
	command := workerCommand{reply: make(chan workerResult, 1)}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-w.done:
		return nil, errors.New("camera worker is closed")
	case w.commands <- command:
	}
	select {
	case result := <-command.reply:
		return result.data, result.err
	case <-ctx.Done():
		// A synchronous Media Foundation read cannot be interrupted directly by
		// context. The buffered reply lets the worker finish safely if it returns.
		return nil, ctx.Err()
	case <-w.done:
		select {
		case result := <-command.reply:
			return result.data, result.err
		default:
			return nil, errors.New("camera worker stopped during capture")
		}
	}
}

func (w *captureWorker) close() error {
	command := workerCommand{close: true, reply: make(chan workerResult, 1)}
	ctx, cancel := context.WithTimeout(context.Background(), closeTimeout)
	defer cancel()

	select {
	case <-w.done:
		return nil
	case w.commands <- command:
	case <-ctx.Done():
		return fmt.Errorf("timed out waiting to shut down Media Foundation camera after %s", closeTimeout)
	}
	select {
	case result := <-command.reply:
		if result.err != nil {
			return result.err
		}
		select {
		case <-w.done:
			return nil
		case <-ctx.Done():
			return fmt.Errorf("timed out releasing Media Foundation resources after %s", closeTimeout)
		}
	case <-w.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("timed out closing Media Foundation camera after %s", closeTimeout)
	}
}

// diagnoseCaptureError turns a common hardware-streaming HRESULT into an
// actionable message without discarding the original error for diagnostics.
// MF_E_HW_MFT_FAILED_START_STREAMING may occur when another application owns
// the camera or when the device/driver cannot allocate streaming resources.
func diagnoseCaptureError(err error) error {
	var hresult mfconsts.HResultError
	if errors.As(err, &hresult) && uint32(hresult.Code) == 0xC00D3704 {
		return fmt.Errorf(
			"Media Foundation could not start the camera stream (MF_E_HW_MFT_FAILED_START_STREAMING, HRESULT 0x%08X); the camera may be in use by another application or the driver may lack streaming resources. Close other camera apps, wait for the device to be released, then start a new session: %w",
			uint32(hresult.Code), err,
		)
	}
	return err
}

func readMJPEGFrame(opened *openedCamera) ([]byte, error) {
	frame, err := opened.capture.GetFrame()
	if err != nil {
		return nil, err
	}
	if frame == nil || frame.PpSample == nil {
		if frame != nil {
			_ = frame.Release()
		}
		return nil, errors.New("Media Foundation returned an empty frame sample")
	}
	buffer, err := opened.device.ParseSampleToBuffer(frame.PpSample)
	if err != nil {
		_ = frame.Release()
		return nil, fmt.Errorf("read camera sample buffer: %w", err)
	}
	length := int(buffer.Length)
	if length < 1 || length > len(buffer.Buffer) {
		buffer.Release()
		_ = frame.Release()
		return nil, fmt.Errorf("invalid sample length %d (buffer has %d bytes)", length, len(buffer.Buffer))
	}
	data := append([]byte(nil), buffer.Buffer[:length]...)
	buffer.Release()
	if err := frame.Release(); err != nil {
		return nil, fmt.Errorf("release camera sample: %w", err)
	}
	return data, nil
}

func selectMJPEGFormat(formats []*mf.CaptureFormats, width, height int, fps float64) (*mf.CaptureFormats, []camera.Resolution, error) {
	resolutionSet := make(map[camera.Resolution]struct{})
	var exact []*mf.CaptureFormats
	for _, format := range formats {
		if format == nil || format.Width == 0 || format.Height == 0 || !isMJPEG(format) {
			continue
		}
		resolution := camera.Resolution{Width: int(format.Width), Height: int(format.Height)}
		resolutionSet[resolution] = struct{}{}
		if int(format.Width) == width && int(format.Height) == height {
			exact = append(exact, format)
		}
	}
	resolutions := make([]camera.Resolution, 0, len(resolutionSet))
	for resolution := range resolutionSet {
		resolutions = append(resolutions, resolution)
	}
	sort.Slice(resolutions, func(i, j int) bool {
		if resolutions[i].Width == resolutions[j].Width {
			return resolutions[i].Height < resolutions[j].Height
		}
		return resolutions[i].Width < resolutions[j].Width
	})
	if len(exact) == 0 {
		return nil, resolutions, fmt.Errorf("device has no MJPEG format at %dx%d", width, height)
	}
	var below, above *mf.CaptureFormats
	for _, format := range exact {
		if format.Fps <= fps && (below == nil || format.Fps > below.Fps) {
			below = format
		}
		if format.Fps > fps && (above == nil || format.Fps < above.Fps) {
			above = format
		}
	}
	selected := below
	if selected == nil {
		selected = above
	}
	if selected == nil {
		return nil, resolutions, fmt.Errorf("device has no usable frame rate at %dx%d", width, height)
	}
	return selected, resolutions, nil
}

func isMJPEG(format *mf.CaptureFormats) bool {
	return format != nil && format.SubType != nil &&
		strings.Contains(strings.ToLower(fmt.Sprint(format.SubType)), mjpgSubtypeToken)
}
