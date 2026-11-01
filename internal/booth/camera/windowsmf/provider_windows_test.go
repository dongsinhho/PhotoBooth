//go:build windows

package windowsmf

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	mfconsts "github.com/Kirizu-Official/windows-camera-go/windows/consts"
	"github.com/dongsinhho/PhotoBooth/internal/booth/camera"
)

func TestDiagnoseCaptureErrorExplainsBusyOrResourceLimitedCamera(t *testing.T) {
	original := mfconsts.HResultError{Code: 0xC00D3704}
	got := diagnoseCaptureError(original)
	if got == nil {
		t.Fatal("expected a diagnostic error")
	}
	message := got.Error()
	for _, expected := range []string{
		"MF_E_HW_MFT_FAILED_START_STREAMING",
		"0xC00D3704",
		"another application",
		"Close other camera apps",
	} {
		if !strings.Contains(message, expected) {
			t.Fatalf("diagnostic %q does not contain %q", message, expected)
		}
	}
	var preserved mfconsts.HResultError
	if !errors.As(got, &preserved) || preserved.Code != original.Code {
		t.Fatalf("original HRESULT was not preserved: %v", got)
	}
}

func TestDiagnoseCaptureErrorExplainsInvalidatedCamera(t *testing.T) {
	original := mfconsts.HResultError{Code: 0xC00D3EA2}
	got := diagnoseCaptureError(original)
	if got == nil {
		t.Fatal("expected an actionable device-invalidated diagnostic")
	}
	for _, expected := range []string{
		"MF_E_VIDEO_RECORDING_DEVICE_INVALIDATED",
		"0xC00D3EA2",
		"disconnected",
		"reopen the camera provider",
	} {
		if !strings.Contains(got.Error(), expected) {
			t.Fatalf("diagnostic %q does not contain %q", got, expected)
		}
	}
	var preserved mfconsts.HResultError
	if !errors.As(got, &preserved) || uint32(preserved.Code) != 0xC00D3EA2 {
		t.Fatalf("original HRESULT was not preserved: %v", got)
	}
}

func TestDiagnoseCaptureErrorLeavesOtherErrorsUnchanged(t *testing.T) {
	original := errors.New("unrelated camera decode error")
	if got := diagnoseCaptureError(original); got != original {
		t.Fatalf("unrelated error should remain unchanged; got %v", got)
	}
}

func TestWarmupFrameCountIsBoundedAndBasedOnFrameRate(t *testing.T) {
	tests := []struct {
		name     string
		fps      float64
		duration time.Duration
		want     int
	}{
		{name: "30 fps for one second", fps: 30, duration: time.Second, want: 30},
		{name: "15 fps for one second", fps: 15, duration: time.Second, want: 15},
		{name: "fractional second rounds up", fps: 30, duration: 110 * time.Millisecond, want: 4},
		{name: "defaults for unset values", want: 30},
		{name: "high rate is capped", fps: 1000, duration: 3 * time.Second, want: maxWarmupFrames},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := warmupFrameCount(tt.fps, tt.duration); got != tt.want {
				t.Fatalf("warmupFrameCount(%v, %s)=%d, want %d", tt.fps, tt.duration, got, tt.want)
			}
		})
	}
}

func TestNewAppliesBoundedWarmupDuration(t *testing.T) {
	provider := New(Options{})
	backend, ok := provider.backend.(*windowsBackend)
	if !ok {
		t.Fatalf("backend type=%T, want *windowsBackend", provider.backend)
	}
	if backend.options.WarmupDuration != defaultWarmupDuration {
		t.Fatalf("default warm-up duration=%s, want %s", backend.options.WarmupDuration, defaultWarmupDuration)
	}

	provider = New(Options{WarmupDuration: 10 * time.Second})
	backend, ok = provider.backend.(*windowsBackend)
	if !ok {
		t.Fatalf("backend type=%T, want *windowsBackend", provider.backend)
	}
	if backend.options.WarmupDuration != maxWarmupDuration {
		t.Fatalf("warm-up duration=%s, want cap %s", backend.options.WarmupDuration, maxWarmupDuration)
	}
}

func TestNewCanSkipWarmupOnlyWhenExplicitlyRequestedForDiagnostics(t *testing.T) {
	provider := New(Options{SkipWarmupForDiagnostics: true})
	backend, ok := provider.backend.(*windowsBackend)
	if !ok {
		t.Fatalf("backend type=%T, want *windowsBackend", provider.backend)
	}
	if !backend.options.SkipWarmupForDiagnostics {
		t.Fatal("diagnostic skip option was not retained")
	}
	if backend.options.WarmupDuration != 0 {
		t.Fatalf("diagnostic warm-up duration=%s, want 0", backend.options.WarmupDuration)
	}

	normal := New(Options{})
	normalBackend := normal.backend.(*windowsBackend)
	if normalBackend.options.SkipWarmupForDiagnostics {
		t.Fatal("normal provider unexpectedly disabled warm-up")
	}
	if normalBackend.options.WarmupDuration != defaultWarmupDuration {
		t.Fatalf("normal warm-up duration=%s, want %s", normalBackend.options.WarmupDuration, defaultWarmupDuration)
	}
}

func TestDiscardWarmupFramesReadsExactlyRequestedFrames(t *testing.T) {
	calls := 0
	err := discardWarmupFrames(5, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("discardWarmupFrames: %v", err)
	}
	if calls != 5 {
		t.Fatalf("read callback calls=%d, want 5", calls)
	}
}

func TestDiscardWarmupFramesStopsAndPreservesReadError(t *testing.T) {
	sentinel := errors.New("synthetic warm-up read failure")
	calls := 0
	err := discardWarmupFrames(5, func() error {
		calls++
		if calls == 3 {
			return sentinel
		}
		return nil
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("error=%v, want original read error", err)
	}
	if !strings.Contains(err.Error(), "frame 3/5") {
		t.Fatalf("error=%q, want failing frame index", err)
	}
	if calls != 3 {
		t.Fatalf("read callback calls=%d, want stop on call 3", calls)
	}
}

func TestDiscardWarmupFramesRejectsInvalidInput(t *testing.T) {
	if err := discardWarmupFrames(0, func() error { return nil }); err == nil {
		t.Fatal("zero frame count should fail")
	}
	if err := discardWarmupFrames(1, nil); err == nil {
		t.Fatal("nil frame reader should fail")
	}
}

func TestCancelledCaptureDoesNotLeakWorkerAndCloseCanRetry(t *testing.T) {
	readStarted := make(chan struct{})
	releaseRead := make(chan struct{})
	worker := &captureWorker{
		commands:        make(chan workerCommand),
		ready:           make(chan startupResult, 1),
		done:            make(chan struct{}),
		shutdownTimeout: 25 * time.Millisecond,
	}
	go func() {
		defer close(worker.done)
		serveCaptureCommands(worker.commands, func() ([]byte, error) {
			close(readStarted)
			<-releaseRead // Simulates a native Media Foundation read that cannot be interrupted.
			return []byte("synthetic frame"), nil
		})
	}()

	ctx, cancel := context.WithCancel(context.Background())
	captureDone := make(chan error, 1)
	go func() {
		_, err := worker.nextFrame(ctx)
		captureDone <- err
	}()

	select {
	case <-readStarted:
	case <-time.After(time.Second):
		cancel()
		close(releaseRead)
		t.Fatal("capture worker did not begin the simulated native read")
	}
	cancel()

	select {
	case err := <-captureDone:
		if !errors.Is(err, context.Canceled) {
			close(releaseRead)
			t.Fatalf("nextFrame error=%v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		close(releaseRead)
		t.Fatal("cancelled caller remained blocked on the native read")
	}

	started := time.Now()
	closeErr := worker.close()
	elapsed := time.Since(started)
	if closeErr == nil {
		close(releaseRead)
		t.Fatal("Close should report a bounded timeout while the native read is blocked")
	}
	if !strings.Contains(closeErr.Error(), "timed out waiting to shut down Media Foundation camera") {
		close(releaseRead)
		t.Fatalf("Close error=%v, want shutdown timeout", closeErr)
	}
	if elapsed > time.Second {
		close(releaseRead)
		t.Fatalf("Close took %s despite a %s test timeout", elapsed, worker.shutdownTimeout)
	}

	// Once the simulated native call returns, the buffered capture reply must
	// not deadlock the worker. A later Close can then send its command and wait
	// for native resources to be released.
	close(releaseRead)
	select {
	case <-worker.done:
		t.Fatal("worker should remain available for a second Close command after the read returns")
	case <-time.After(10 * time.Millisecond):
	}

	if err := worker.close(); err != nil {
		t.Fatalf("second Close after the read returned: %v", err)
	}
	select {
	case <-worker.done:
	case <-time.After(time.Second):
		t.Fatal("worker did not shut down after the successful second Close")
	}
}
func TestDeadlineCaptureDoesNotLeakWorkerAndCloseCanRetry(t *testing.T) {
	readStarted := make(chan struct{})
	releaseRead := make(chan struct{})
	worker := &captureWorker{
		commands:        make(chan workerCommand),
		ready:           make(chan startupResult, 1),
		done:            make(chan struct{}),
		shutdownTimeout: 25 * time.Millisecond,
	}
	go func() {
		defer close(worker.done)
		serveCaptureCommands(worker.commands, func() ([]byte, error) {
			close(readStarted)
			<-releaseRead
			return []byte("synthetic frame"), nil
		})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	captureDone := make(chan error, 1)
	go func() {
		_, err := worker.nextFrame(ctx)
		captureDone <- err
	}()

	select {
	case <-readStarted:
	case <-time.After(time.Second):
		close(releaseRead)
		t.Fatal("capture worker did not begin the simulated native read")
	}
	select {
	case err := <-captureDone:
		if !errors.Is(err, context.DeadlineExceeded) {
			close(releaseRead)
			t.Fatalf("nextFrame error=%v, want context.DeadlineExceeded", err)
		}
	case <-time.After(time.Second):
		close(releaseRead)
		t.Fatal("deadline caller remained blocked on the native read")
	}

	closeErr := worker.close()
	if closeErr == nil || !strings.Contains(closeErr.Error(), "timed out waiting to shut down Media Foundation camera") {
		close(releaseRead)
		t.Fatalf("Close error=%v, want bounded shutdown timeout", closeErr)
	}

	close(releaseRead)
	if err := worker.close(); err != nil {
		t.Fatalf("second Close after timed-out read returned: %v", err)
	}
	select {
	case <-worker.done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after second Close")
	}
}

func TestCancelledStartupWaitsForNativeWorkerCleanup(t *testing.T) {
	releaseStartup := make(chan struct{})
	worker := &captureWorker{
		commands:        make(chan workerCommand),
		ready:           make(chan startupResult, 1),
		done:            make(chan struct{}),
		shutdownTimeout: time.Second,
	}
	go func() {
		defer close(worker.done)
		<-releaseStartup // Simulate synchronous COM/device startup.
		worker.ready <- startupResult{info: camera.Info{Name: "test camera"}}
		serveCaptureCommands(worker.commands, func() ([]byte, error) { return []byte("frame"), nil })
	}()

	cleanupDone := make(chan error, 1)
	go func() { cleanupDone <- closeWorkerAfterCancelledStartup(worker) }()
	select {
	case err := <-cleanupDone:
		t.Fatalf("cancelled startup cleanup returned before native startup finished: %v", err)
	case <-time.After(30 * time.Millisecond):
	}

	close(releaseStartup)
	select {
	case err := <-cleanupDone:
		if err != nil {
			t.Fatalf("cleanup after startup returned: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled startup did not close the worker after startup returned")
	}
	select {
	case <-worker.done:
	default:
		t.Fatal("cancelled startup cleanup returned before worker resources were released")
	}
}

func TestCancelledStartupWithOpenFailureWaitsForWorkerExit(t *testing.T) {
	releaseExit := make(chan struct{})
	worker := &captureWorker{
		commands:        make(chan workerCommand),
		ready:           make(chan startupResult, 1),
		done:            make(chan struct{}),
		shutdownTimeout: time.Second,
	}
	go func() {
		<-releaseExit
		worker.ready <- startupResult{err: errors.New("synthetic camera startup failure")}
		close(worker.done)
	}()
	cleanupDone := make(chan error, 1)
	go func() { cleanupDone <- closeWorkerAfterCancelledStartup(worker) }()
	select {
	case err := <-cleanupDone:
		t.Fatalf("cleanup returned before failed worker exited: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(releaseExit)
	select {
	case err := <-cleanupDone:
		if err != nil {
			t.Fatalf("cleanup error=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cleanup did not wait for failed worker exit")
	}
}
