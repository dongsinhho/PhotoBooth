package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/dongsinhho/PhotoBooth/internal/booth/camera"
	"github.com/dongsinhho/PhotoBooth/internal/booth/camera/recovery"
	"github.com/dongsinhho/PhotoBooth/internal/booth/camera/windowsmf"
	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	cameraPreviewEventName       = "camera-preview-frame"
	cameraPreviewStatusEventName = "camera-preview-status"
	cameraPreviewInterval        = 200 * time.Millisecond // maximum 5 published frames per second
	cameraPreviewAckTimeout      = 2 * time.Second
	cameraPreviewStopTimeout     = 7 * time.Second
	maxCameraPreviewBytes        = 8 << 20
)

type CameraPreviewStatus struct {
	State     string `json:"state"`
	Message   string `json:"message"`
	Available bool   `json:"available"`
}

type CameraPreviewFrame struct {
	Sequence uint64 `json:"sequence"`
	DataURL  string `json:"data_url"`
}

const cameraPreviewDiagnosticsEnv = "PHOTOBOOTH_PREVIEW_METRICS_PATH"

var cameraPreviewDiagnosticsMu sync.Mutex
var cameraPreviewDiagnosticsFile *os.File
var cameraPreviewDiagnosticsFilePath string
var cameraPreviewDiagnosticsWriteFailures uint64

type cameraPreviewDiagnosticRecord struct {
	TimestampUTC time.Time `json:"timestamp_utc"`
	Event        string    `json:"event"`
	State        string    `json:"state,omitempty"`
	Sequence     uint64    `json:"sequence,omitempty"`
	LatencyMS    float64   `json:"latency_ms,omitempty"`
	FrameBytes   int       `json:"frame_bytes,omitempty"`
	Message      string    `json:"message,omitempty"`
}

func closeCameraPreviewDiagnosticsLocked() error {
	if cameraPreviewDiagnosticsFile == nil {
		cameraPreviewDiagnosticsFilePath = ""
		return nil
	}
	file := cameraPreviewDiagnosticsFile
	cameraPreviewDiagnosticsFile = nil
	cameraPreviewDiagnosticsFilePath = ""
	if err := file.Sync(); err != nil {
		_ = file.Close()
		cameraPreviewDiagnosticsWriteFailures++
		return err
	}
	if err := file.Close(); err != nil {
		cameraPreviewDiagnosticsWriteFailures++
		return err
	}
	return nil
}

// closeCameraPreviewDiagnostics flushes and releases the opt-in soak-test log.
// It is also used when a test switches/clears PHOTOBOOTH_PREVIEW_METRICS_PATH.
func closeCameraPreviewDiagnostics() error {
	cameraPreviewDiagnosticsMu.Lock()
	defer cameraPreviewDiagnosticsMu.Unlock()
	return closeCameraPreviewDiagnosticsLocked()
}

func previewDiagnosticsWriteFailureCount() uint64 {
	cameraPreviewDiagnosticsMu.Lock()
	defer cameraPreviewDiagnosticsMu.Unlock()
	return cameraPreviewDiagnosticsWriteFailures
}

// recordCameraPreviewDiagnostic is opt-in and writes append-only JSONL for
// host-side soak tests. Normal guest sessions do not create diagnostic files.
// One append handle is reused for the test run rather than opening and closing
// it for every 5 FPS frame. Path changes close the previous handle, and failed
// writes drop the handle so a later event can retry opening it. Every write is
// checked and serialized with preview status/frame events.
func recordCameraPreviewDiagnostic(record cameraPreviewDiagnosticRecord) {
	path := os.Getenv(cameraPreviewDiagnosticsEnv)
	encoded, err := json.Marshal(record)
	if err != nil {
		cameraPreviewDiagnosticsMu.Lock()
		cameraPreviewDiagnosticsWriteFailures++
		cameraPreviewDiagnosticsMu.Unlock()
		fmt.Fprintf(os.Stderr, "photobooth preview diagnostics marshal failed: %v\n", err)
		return
	}
	line := append(encoded, byte('\n'))

	cameraPreviewDiagnosticsMu.Lock()
	defer cameraPreviewDiagnosticsMu.Unlock()

	if path == "" {
		if err := closeCameraPreviewDiagnosticsLocked(); err != nil {
			fmt.Fprintf(os.Stderr, "photobooth preview diagnostics close failed: %v\n", err)
		}
		return
	}

	if cameraPreviewDiagnosticsFile != nil && cameraPreviewDiagnosticsFilePath != path {
		if err := closeCameraPreviewDiagnosticsLocked(); err != nil {
			fmt.Fprintf(os.Stderr, "photobooth preview diagnostics path-change close failed: %v\n", err)
		}
	}
	if cameraPreviewDiagnosticsFile == nil {
		file, openErr := openPreviewDiagnostics(path)
		if openErr != nil {
			cameraPreviewDiagnosticsWriteFailures++
			fmt.Fprintf(os.Stderr, "photobooth preview diagnostics open failed: %v\n", openErr)
			return
		}
		cameraPreviewDiagnosticsFile = file
		cameraPreviewDiagnosticsFilePath = path
	}

	written, writeErr := cameraPreviewDiagnosticsFile.Write(line)
	if writeErr == nil && written != len(line) {
		writeErr = fmt.Errorf("short write: %d of %d bytes", written, len(line))
	}
	if writeErr != nil {
		cameraPreviewDiagnosticsWriteFailures++
		file := cameraPreviewDiagnosticsFile
		cameraPreviewDiagnosticsFile = nil
		cameraPreviewDiagnosticsFilePath = ""
		closeErr := file.Close()
		fmt.Fprintf(os.Stderr, "photobooth preview diagnostics write failed: %v\n", writeErr)
		if closeErr != nil {
			fmt.Fprintf(os.Stderr, "photobooth preview diagnostics close after write failure failed: %v\n", closeErr)
		}
	}
}

// StartCameraPreview starts one idle-only preview pump. It opens the same
// provider/worker implementation used by capture, but StartPhotoSession always
// cancels and closes it before the workflow is allowed to open the camera.
func (a *App) StartCameraPreview() (CameraPreviewStatus, error) {
	a.cameraLifecycleMu.Lock()
	defer a.cameraLifecycleMu.Unlock()

	if runtime.GOOS != "windows" {
		status := CameraPreviewStatus{State: "unavailable", Message: "Live camera preview is currently supported on Windows only."}
		a.setCameraPreviewStatus(status)
		return status, nil
	}
	if err := a.preparePlayerPaths(); err != nil {
		return CameraPreviewStatus{}, err
	}
	a.mu.Lock()
	if a.playerRunning || a.outputRetrying || a.settingsSaving || a.player.State != "idle" {
		state := a.player.State
		a.mu.Unlock()
		return CameraPreviewStatus{}, fmt.Errorf("camera preview is only available while the player is idle (current state: %s)", state)
	}
	config := a.stationConfig
	a.mu.Unlock()

	a.previewMu.Lock()
	previousDone := a.previewDone
	if previousDone != nil {
		select {
		case <-previousDone:
			previousCloseErr := a.previewCloseErr
			if a.previewCancel != nil {
				a.previewCancel()
			}
			a.previewCancel = nil
			a.previewDone = nil
			a.previewAck = nil
			a.previewAckPending = false
			a.previewInFlight = 0
			if previousCloseErr != nil {
				status := CameraPreviewStatus{State: "unavailable", Message: "The previous preview did not release the webcam cleanly: " + previousCloseErr.Error()}
				a.previewStatus = status
				a.previewMu.Unlock()
				a.emitCameraPreviewStatus(status)
				return status, previousCloseErr
			}
		default:
			status := a.previewStatus
			a.previewMu.Unlock()
			return status, nil
		}
	} else if a.previewCloseErr != nil {
		closeErr := a.previewCloseErr
		a.previewMu.Unlock()
		return CameraPreviewStatus{State: "unavailable", Message: closeErr.Error()}, closeErr
	}
	ctx, cancel := context.WithCancel(a.playerBaseContext())
	done := make(chan struct{})
	ack := make(chan uint64, 1)
	a.previewCancel = cancel
	a.previewDone = done
	a.previewAck = ack
	a.previewAckPending = false
	a.previewInFlight = 0
	a.previewCloseErr = nil
	a.previewStatus = CameraPreviewStatus{State: "starting", Message: "Connecting to the webcam…"}
	status := a.previewStatus
	a.previewMu.Unlock()

	a.emitCameraPreviewStatus(status)
	go a.runCameraPreview(ctx, done, ack, config)
	return status, nil
}

// StopCameraPreview is a synchronization barrier: it returns only after the
// preview goroutine has closed its provider. If native cleanup times out,
// callers must not open another camera provider.
func (a *App) StopCameraPreview() error {
	a.cameraLifecycleMu.Lock()
	defer a.cameraLifecycleMu.Unlock()
	return a.stopCameraPreviewLocked()
}

func (a *App) stopCameraPreviewLocked() error {
	a.previewMu.Lock()
	cancel, done := a.previewCancel, a.previewDone
	closeErr := a.previewCloseErr
	var stoppingStatus CameraPreviewStatus
	if done != nil {
		stoppingStatus = CameraPreviewStatus{State: "stopping", Message: "Releasing the webcam before continuing…"}
		a.previewStatus = stoppingStatus
	}
	a.previewMu.Unlock()
	if done == nil {
		return closeErr
	}
	a.emitCameraPreviewStatus(stoppingStatus)
	if cancel != nil {
		cancel()
	}
	timer := time.NewTimer(cameraPreviewStopTimeout)
	defer timer.Stop()
	select {
	case <-done:
		return a.clearCameraPreviewRun(done)
	case <-timer.C:
		err := errors.New("camera preview did not stop within the shutdown budget; refusing to start capture while the preview worker may still own the camera")
		a.setCameraPreviewStatus(CameraPreviewStatus{State: "unavailable", Message: err.Error()})
		return err
	}
}

func (a *App) clearCameraPreviewRun(done chan struct{}) error {
	a.previewMu.Lock()
	defer a.previewMu.Unlock()
	if a.previewDone != done {
		return nil
	}
	closeErr := a.previewCloseErr
	a.previewCancel = nil
	a.previewDone = nil
	a.previewAck = nil
	a.previewAckPending = false
	a.previewInFlight = 0
	if closeErr == nil {
		a.previewCloseErr = nil
	}
	return closeErr
}

func (a *App) ReportCameraPreviewDecodeFailure(sequence uint64) error {
	if sequence == 0 {
		return errors.New("camera preview decode failure requires a positive sequence")
	}
	a.previewMu.Lock()
	current := a.previewDone != nil && sequence == a.previewInFlight
	a.previewMu.Unlock()
	if !current {
		return errors.New("camera preview decode failure refers to a stale frame")
	}
	recordCameraPreviewDiagnostic(cameraPreviewDiagnosticRecord{
		TimestampUTC: time.Now().UTC(), Event: "frame_decode_failed", Sequence: sequence,
	})
	return nil
}

func (a *App) AcknowledgeCameraPreview(sequence uint64) error {
	if sequence == 0 {
		return errors.New("camera preview sequence must be positive")
	}
	a.previewMu.Lock()
	defer a.previewMu.Unlock()
	if a.previewDone == nil || a.previewAck == nil || sequence != a.previewInFlight {
		return errors.New("camera preview frame is stale or no longer in flight")
	}
	if a.previewAckPending {
		return errors.New("camera preview acknowledgement is already pending")
	}
	a.previewAckPending = true
	select {
	case a.previewAck <- sequence:
		return nil
	default:
		a.previewAckPending = false
		return errors.New("camera preview acknowledgement queue is unexpectedly full")
	}
}

func (a *App) runCameraPreview(ctx context.Context, done chan struct{}, ack chan uint64, config stationconfig.StoreConfig) {
	provider := newConfiguredCameraProvider(config, func(event recovery.Event) { a.handlePreviewRecoveryEvent(event) })
	defer func() {
		closeErr := provider.Close()
		a.previewMu.Lock()
		if a.previewDone == done {
			a.previewCloseErr = closeErr
		}
		a.previewMu.Unlock()
		if closeErr != nil {
			a.setCameraPreviewStatus(CameraPreviewStatus{State: "unavailable", Message: "The webcam preview stopped, but the camera driver did not release cleanly: " + closeErr.Error()})
		}
		close(done)
	}()

	if err := provider.Open(ctx); err != nil {
		if ctx.Err() == nil {
			a.setCameraPreviewStatus(CameraPreviewStatus{State: "unavailable", Message: friendlyPlayerError(err, false)})
		}
		return
	}
	a.setCameraPreviewStatus(CameraPreviewStatus{State: "ready", Message: "Webcam ready. Live preview is active.", Available: true})

	for {
		if err := ctx.Err(); err != nil {
			return
		}
		frame, err := provider.Preview(ctx)
		if err != nil {
			if ctx.Err() == nil {
				a.setCameraPreviewStatus(CameraPreviewStatus{State: "unavailable", Message: "Webcam preview stopped: " + err.Error()})
			}
			return
		}
		if len(frame) == 0 || len(frame) > maxCameraPreviewBytes {
			a.setCameraPreviewStatus(CameraPreviewStatus{State: "unavailable", Message: fmt.Sprintf("The webcam returned a preview frame outside the %d MiB size limit.", maxCameraPreviewBytes>>20)})
			return
		}
		sequence := a.previewSequence.Add(1)
		dataURL := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(frame)
		a.previewMu.Lock()
		if a.previewDone != done || a.previewAck != ack {
			a.previewMu.Unlock()
			return
		}
		a.previewInFlight = sequence
		a.previewAckPending = false
		a.previewMu.Unlock()

		publishedAt := time.Now()
		recordCameraPreviewDiagnostic(cameraPreviewDiagnosticRecord{
			TimestampUTC: publishedAt.UTC(), Event: "frame_published",
			Sequence: sequence, FrameBytes: len(frame),
		})
		if a.ctx != nil {
			wailsruntime.EventsEmit(a.ctx, cameraPreviewEventName, CameraPreviewFrame{Sequence: sequence, DataURL: dataURL})
		}
		timer := time.NewTimer(cameraPreviewAckTimeout)
		acknowledged := false
		select {
		case got := <-ack:
			a.previewMu.Lock()
			if got == sequence && a.previewInFlight == sequence && a.previewAckPending {
				acknowledged = true
				a.previewAckPending = false
				a.previewInFlight = 0
			}
			a.previewMu.Unlock()
			if acknowledged {
				recordCameraPreviewDiagnostic(cameraPreviewDiagnosticRecord{
					TimestampUTC: time.Now().UTC(), Event: "frame_acknowledged",
					Sequence: sequence, LatencyMS: float64(time.Since(publishedAt)) / float64(time.Millisecond),
				})
			}
		case <-ctx.Done():
			// Prefer an ACK already queued at the same moment as cancellation.
			// Otherwise record the in-flight frame as intentionally cancelled.
			select {
			case got := <-ack:
				a.previewMu.Lock()
				if got == sequence && a.previewInFlight == sequence && a.previewAckPending {
					acknowledged = true
					a.previewAckPending = false
					a.previewInFlight = 0
				}
				a.previewMu.Unlock()
			default:
			}
			if acknowledged {
				recordCameraPreviewDiagnostic(cameraPreviewDiagnosticRecord{
					TimestampUTC: time.Now().UTC(), Event: "frame_acknowledged",
					Sequence: sequence, LatencyMS: float64(time.Since(publishedAt)) / float64(time.Millisecond),
				})
			} else {
				recordCameraPreviewDiagnostic(cameraPreviewDiagnosticRecord{
					TimestampUTC: time.Now().UTC(), Event: "frame_cancelled",
					Sequence: sequence, Message: "preview context cancelled before frame ACK",
				})
			}
		case <-timer.C:
			recordCameraPreviewDiagnostic(cameraPreviewDiagnosticRecord{
				TimestampUTC: time.Now().UTC(), Event: "frame_ack_timeout",
				Sequence: sequence, LatencyMS: float64(cameraPreviewAckTimeout) / float64(time.Millisecond),
			})
			a.setCameraPreviewStatus(CameraPreviewStatus{State: "stalled", Message: "The preview display stopped responding. Retry the camera preview when the UI is ready."})
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		a.previewMu.Lock()
		if a.previewInFlight == sequence {
			a.previewInFlight = 0
		}
		if !acknowledged {
			a.previewAckPending = false
		}
		a.previewMu.Unlock()
		if ctx.Err() != nil || !acknowledged {
			return
		}

		ticker := time.NewTimer(cameraPreviewInterval)
		select {
		case <-ticker.C:
		case <-ctx.Done():
			if !ticker.Stop() {
				select {
				case <-ticker.C:
				default:
				}
			}
			return
		}
	}
}

func (a *App) setCameraPreviewStatus(status CameraPreviewStatus) {
	a.previewMu.Lock()
	a.previewStatus = status
	a.previewMu.Unlock()
	a.emitCameraPreviewStatus(status)
}

func (a *App) emitCameraPreviewStatus(status CameraPreviewStatus) {
	recordCameraPreviewDiagnostic(cameraPreviewDiagnosticRecord{
		TimestampUTC: time.Now().UTC(), Event: "preview_status",
		State: status.State, Message: status.Message,
	})
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, cameraPreviewStatusEventName, status)
	}
}

func (a *App) handlePreviewRecoveryEvent(event recovery.Event) {
	switch event.Type {
	case "recovery_started", "recovery_open_attempt_failed":
		message := "Webcam connection changed. Reconnecting preview…"
		if event.Type == "recovery_open_attempt_failed" {
			message = fmt.Sprintf("Waiting for the webcam to return (attempt %d of %d)…", event.Attempt, event.Total)
		}
		a.setCameraPreviewStatus(CameraPreviewStatus{State: "reconnecting", Message: message})
	case "recovery_provider_opened", "recovery_succeeded":
		a.setCameraPreviewStatus(CameraPreviewStatus{State: "ready", Message: "Webcam reconnected. Live preview resumed.", Available: true})
	case "recovery_failed":
		message := "Webcam preview could not recover. Check the USB connection and retry preview."
		if event.Err != nil {
			message += " " + event.Err.Error()
		}
		a.setCameraPreviewStatus(CameraPreviewStatus{State: "unavailable", Message: message})
	}
}

func (a *App) shutdown(context.Context) {
	_ = a.StopCameraPreview()
	if err := closeCameraPreviewDiagnostics(); err != nil {
		fmt.Fprintf(os.Stderr, "photobooth preview diagnostics shutdown close failed: %v\n", err)
	}
}

func newConfiguredCameraProvider(config stationconfig.StoreConfig, onEvent func(recovery.Event)) *recovery.Provider {
	options := windowsmf.Options{
		DeviceID: config.Camera.Device,
		Width:    config.Camera.Width,
		Height:   config.Camera.Height,
		FPS:      config.Camera.FPS,
	}
	return recovery.New(func() camera.Provider { return windowsmf.New(options) }, recovery.Options{
		OpenAttempts:     3,
		RecoveryAttempts: 30,
		RetryDelay:       250 * time.Millisecond,
		OnEvent:          onEvent,
	})
}

// CurrentCameraPreviewStatus returns the latest preview lifecycle state so the
// UI can reconcile the initial StartCameraPreview response with asynchronous
// startup/recovery events without briefly overwriting a newer status.
func (a *App) CurrentCameraPreviewStatus() CameraPreviewStatus {
	a.previewMu.Lock()
	defer a.previewMu.Unlock()
	return a.previewStatus
}
