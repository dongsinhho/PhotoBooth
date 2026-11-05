package recovery

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dongsinhho/PhotoBooth/internal/booth/camera"
)

type stubProvider struct {
	name         string
	openErr      error
	closeErr     error
	captureErr   error
	previewErr   error
	previewFrame []byte
	opened       bool
	openCalls    int
	closeCalls   int
	captureCalls int
	previewCalls int
}

func (s *stubProvider) Info() camera.Info {
	return camera.Info{Name: s.name, Backend: "test"}
}
func (s *stubProvider) Open(ctx context.Context) error {
	s.openCalls++
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.openErr != nil {
		return s.openErr
	}
	if s.opened {
		return errors.New("already open")
	}
	s.opened = true
	return nil
}
func (s *stubProvider) Close() error {
	s.closeCalls++
	if s.closeErr != nil {
		return s.closeErr
	}
	s.opened = false
	return nil
}
func (s *stubProvider) Preview(ctx context.Context) ([]byte, error) {
	s.previewCalls++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !s.opened {
		return nil, errors.New("not open")
	}
	if s.previewErr != nil {
		return nil, s.previewErr
	}
	if len(s.previewFrame) == 0 {
		return []byte("preview-frame"), nil
	}
	return append([]byte(nil), s.previewFrame...), nil
}
func (s *stubProvider) Capture(ctx context.Context, req camera.CaptureRequest) (camera.CaptureResult, error) {
	s.captureCalls++
	if err := ctx.Err(); err != nil {
		return camera.CaptureResult{}, err
	}
	if !s.opened {
		return camera.CaptureResult{}, errors.New("not open")
	}
	if s.captureErr != nil {
		return camera.CaptureResult{}, s.captureErr
	}
	return camera.CaptureResult{Sequence: req.Sequence, Path: req.OutputPath, Width: req.Width, Height: req.Height}, nil
}

func noWait(ctx context.Context, _ time.Duration) error { return ctx.Err() }

type retryableStubProvider struct{ *stubProvider }

func (s *retryableStubProvider) RetryOpenAfterFailure() bool { return true }

type recoveryCloseStubProvider struct {
	*stubProvider
	recoveryCloseCalls int
}

func (s *recoveryCloseStubProvider) RetryOpenAfterFailure() bool { return true }

func (s *recoveryCloseStubProvider) CloseForRecovery() error {
	s.recoveryCloseCalls++
	return s.stubProvider.Close()
}

func (s *recoveryCloseStubProvider) Preview(ctx context.Context) ([]byte, error) {
	s.previewCalls++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !s.opened {
		return nil, errors.New("not open")
	}
	if s.previewCalls == 1 {
		return nil, camera.ErrStreamUnavailable
	}
	return []byte("fresh-preview"), nil
}

type recoveryProbeStubProvider struct {
	*stubProvider
	recoveryCloseCalls int
	probeCalls         int
	closeCallsAtProbe  int
}

func (s *recoveryProbeStubProvider) RetryOpenAfterFailure() bool { return true }

func (s *recoveryProbeStubProvider) Open(ctx context.Context) error {
	s.openCalls++
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.openCalls == 1 {
		s.opened = true
		return nil
	}
	return errors.New("synthetic retry open failure")
}

func (s *recoveryProbeStubProvider) Preview(ctx context.Context) ([]byte, error) {
	s.previewCalls++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.previewCalls == 1 {
		return nil, camera.ErrStreamUnavailable
	}
	return []byte("unexpected preview"), nil
}

func (s *recoveryProbeStubProvider) CloseForRecovery() error {
	s.recoveryCloseCalls++
	return s.stubProvider.Close()
}

func (s *recoveryProbeStubProvider) ProbeAfterRecoveryFailure() (string, error) {
	s.probeCalls++
	s.closeCallsAtProbe = s.closeCalls
	return "status=camera_found devices=1", nil
}

func TestPreviewRecoveryProbeRunsBeforeRetryableProviderCleanup(t *testing.T) {
	t.Setenv("PHOTOBOOTH_MF_POST_RECOVERY_PROBE", "1")
	provider := &recoveryProbeStubProvider{stubProvider: &stubProvider{name: "probe camera"}}
	factoryCalls := 0
	var events []Event
	p := New(func() camera.Provider {
		factoryCalls++
		return provider
	}, Options{
		OpenAttempts: 1, RecoveryAttempts: 2, Sleep: noWait,
		OnEvent: func(event Event) { events = append(events, event) },
	})

	if err := p.Open(context.Background()); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := p.Preview(context.Background()); err == nil {
		t.Fatal("expected preview recovery to fail after synthetic Open failures")
	}
	if factoryCalls != 1 || provider.openCalls != 3 {
		t.Fatalf("factory=%d opens=%d, want one provider and initial + two recovery opens", factoryCalls, provider.openCalls)
	}
	if provider.recoveryCloseCalls != 1 || provider.probeCalls != 1 {
		t.Fatalf("recovery close calls=%d probe calls=%d, want one each", provider.recoveryCloseCalls, provider.probeCalls)
	}
	if provider.closeCallsAtProbe != 1 || provider.closeCalls != 2 {
		t.Fatalf("close calls at probe=%d final close calls=%d, want probe before exhaustion cleanup", provider.closeCallsAtProbe, provider.closeCalls)
	}
	foundProbeEvent := false
	for _, event := range events {
		if event.Type == "recovery_post_failure_probe" {
			foundProbeEvent = true
			if event.Message != "status=camera_found devices=1" {
				t.Fatalf("probe event message=%q", event.Message)
			}
		}
	}
	if !foundProbeEvent {
		t.Fatalf("recovery events did not contain post-failure probe: %+v", events)
	}
}

func TestCaptureReopensAndRetriesOnStreamUnavailable(t *testing.T) {
	oldProvider := &stubProvider{name: "old camera", captureErr: camera.ErrStreamUnavailable}
	newProvider := &stubProvider{name: "new camera"}
	factoryCalls := 0
	var events []Event
	p := New(func() camera.Provider {
		factoryCalls++
		if factoryCalls == 1 {
			return oldProvider
		}
		return newProvider
	}, Options{OpenAttempts: 2, Sleep: noWait, OnEvent: func(e Event) { events = append(events, e) }})

	if err := p.Open(context.Background()); err != nil {
		t.Fatalf("Open: %v", err)
	}
	path := filepath.Join(t.TempDir(), "shot.jpg")
	if err := osWriteFile(path, []byte("stale partial artifact")); err != nil {
		t.Fatal(err)
	}
	got, err := p.Capture(context.Background(), camera.CaptureRequest{
		Sequence: 7, Width: 1280, Height: 720, OutputPath: path,
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if got.Sequence != 7 || got.Path != path {
		t.Fatalf("result=%+v", got)
	}
	if factoryCalls != 2 {
		t.Fatalf("factory calls=%d, want 2", factoryCalls)
	}
	if oldProvider.closeCalls != 1 || oldProvider.opened {
		t.Fatalf("old provider close/open state: closeCalls=%d opened=%v", oldProvider.closeCalls, oldProvider.opened)
	}
	if newProvider.captureCalls != 1 || !newProvider.opened {
		t.Fatalf("new provider capture/open state: captureCalls=%d opened=%v", newProvider.captureCalls, newProvider.opened)
	}
	if len(events) < 3 || events[0].Type != "recovery_started" || events[len(events)-1].Type != "recovery_succeeded" {
		t.Fatalf("recovery events=%+v", events)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("test stub should not create an artifact; stat error=%v", err)
	}
}

func TestPreviewReopensAndRetriesOnStreamUnavailable(t *testing.T) {
	oldProvider := &recoveryCloseStubProvider{stubProvider: &stubProvider{name: "old camera"}}
	factoryCalls := 0
	var events []Event
	p := New(func() camera.Provider {
		factoryCalls++
		return oldProvider
	}, Options{
		OpenAttempts: 2, RecoveryAttempts: 3, Sleep: noWait,
		OnEvent: func(event Event) { events = append(events, event) },
	})
	if err := p.Open(context.Background()); err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, err := p.Preview(context.Background())
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if string(got) != "fresh-preview" {
		t.Fatalf("preview=%q, want fresh frame", got)
	}
	if factoryCalls != 1 || oldProvider.openCalls != 2 || oldProvider.closeCalls != 1 || oldProvider.recoveryCloseCalls != 1 || oldProvider.previewCalls != 2 {
		t.Fatalf("factory=%d old opens=%d close=%d recovery close=%d preview=%d", factoryCalls, oldProvider.openCalls, oldProvider.closeCalls, oldProvider.recoveryCloseCalls, oldProvider.previewCalls)
	}
	if len(events) < 3 || events[0].Type != "recovery_started" || events[len(events)-1].Type != "recovery_succeeded" {
		t.Fatalf("preview recovery events=%+v", events)
	}
}

func TestPreviewDoesNotRecoverContextOrDecodeErrors(t *testing.T) {
	decodeErr := errors.New("bad preview JPEG")
	provider := &stubProvider{name: "camera", previewErr: decodeErr}
	factoryCalls := 0
	p := New(func() camera.Provider {
		factoryCalls++
		return provider
	}, Options{Sleep: noWait})
	if err := p.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Preview(context.Background()); !errors.Is(err, decodeErr) {
		t.Fatalf("Preview error=%v, want original error", err)
	}
	if factoryCalls != 1 || provider.closeCalls != 0 {
		t.Fatalf("non-stream preview error should not recover: factory=%d close=%d", factoryCalls, provider.closeCalls)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Preview(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled preview error=%v, want context cancellation", err)
	}
	if factoryCalls != 1 {
		t.Fatalf("context cancellation triggered a reopen: factories=%d", factoryCalls)
	}
}

func TestCaptureDoesNotReopenForNonStreamError(t *testing.T) {
	invalidRequest := errors.New("invalid width")
	provider := &stubProvider{name: "camera", captureErr: invalidRequest}
	factoryCalls := 0
	p := New(func() camera.Provider {
		factoryCalls++
		return provider
	}, Options{Sleep: noWait})
	if err := p.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err := p.Capture(context.Background(), camera.CaptureRequest{Sequence: 1, OutputPath: "ignored"})
	if !errors.Is(err, invalidRequest) {
		t.Fatalf("Capture error=%v, want original validation error", err)
	}
	if factoryCalls != 1 || provider.closeCalls != 0 {
		t.Fatalf("unnecessary recovery: factory calls=%d close calls=%d", factoryCalls, provider.closeCalls)
	}
}

func TestCaptureDoesNotRecoverContextErrors(t *testing.T) {
	provider := &stubProvider{name: "camera", captureErr: context.Canceled}
	factoryCalls := 0
	p := New(func() camera.Provider {
		factoryCalls++
		return provider
	}, Options{Sleep: noWait})
	if err := p.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err := p.Capture(context.Background(), camera.CaptureRequest{Sequence: 1, OutputPath: "ignored"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Capture error=%v, want context cancellation", err)
	}
	if factoryCalls != 1 || provider.closeCalls != 0 {
		t.Fatalf("context error should not recover: factory calls=%d close calls=%d", factoryCalls, provider.closeCalls)
	}
}

func TestFailedRecoveryCanOpenOnNextSession(t *testing.T) {
	disconnected := &stubProvider{name: "old camera", captureErr: camera.ErrStreamUnavailable}
	failedOpen := &stubProvider{name: "disconnected camera", openErr: errors.New("device not found")}
	healthy := &stubProvider{name: "healthy camera"}
	factoryCalls := 0
	p := New(func() camera.Provider {
		factoryCalls++
		switch factoryCalls {
		case 1:
			return disconnected
		case 2, 3:
			return failedOpen
		default:
			return healthy
		}
	}, Options{OpenAttempts: 2, Sleep: noWait})
	if err := p.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err := p.Capture(context.Background(), camera.CaptureRequest{Sequence: 1, OutputPath: filepath.Join(t.TempDir(), "shot.jpg")})
	if err == nil || !strings.Contains(err.Error(), "reopen camera") {
		t.Fatalf("Capture error=%v, expected reconnect failure", err)
	}
	if err := p.Open(context.Background()); err != nil {
		t.Fatalf("Open on next session: %v", err)
	}
	got, err := p.Capture(context.Background(), camera.CaptureRequest{Sequence: 2, OutputPath: filepath.Join(t.TempDir(), "shot2.jpg")})
	if err != nil {
		t.Fatalf("Capture on next session: %v", err)
	}
	if got.Sequence != 2 || healthy.captureCalls != 1 {
		t.Fatalf("result=%+v healthy captures=%d", got, healthy.captureCalls)
	}
}

func TestCloseFailurePreservesProviderForRetry(t *testing.T) {
	closeFailure := errors.New("native worker still shutting down")
	provider := &stubProvider{name: "camera", captureErr: camera.ErrStreamUnavailable, closeErr: closeFailure}
	p := New(func() camera.Provider { return provider }, Options{Sleep: noWait})
	if err := p.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err := p.Capture(context.Background(), camera.CaptureRequest{Sequence: 1, OutputPath: filepath.Join(t.TempDir(), "shot.jpg")})
	if !errors.Is(err, closeFailure) {
		t.Fatalf("Capture error=%v, want close failure preserved", err)
	}
	// The provider must not be silently discarded when cleanup is incomplete.
	if got := p.Info().Name; got != "camera" {
		t.Fatalf("Info name=%q, want provider to remain referenced", got)
	}
	provider.closeErr = nil
	if err := p.Close(); err != nil {
		t.Fatalf("Close retry: %v", err)
	}
}

func osWriteFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}

func TestRecoveryUsesLargerReconnectBudgetThanInitialOpen(t *testing.T) {
	streamLost := &stubProvider{name: "original camera", captureErr: camera.ErrStreamUnavailable}
	healthy := &stubProvider{name: "reconnected camera"}
	factoryCalls := 0
	var failedOpenEvents []Event
	p := New(func() camera.Provider {
		factoryCalls++
		switch factoryCalls {
		case 1:
			return streamLost
		case 2, 3, 4:
			return &stubProvider{name: "camera absent", openErr: errors.New("device not found")}
		default:
			return healthy
		}
	}, Options{
		OpenAttempts:     1,
		RecoveryAttempts: 4,
		Sleep:            noWait,
		OnEvent: func(event Event) {
			if event.Type == "recovery_open_attempt_failed" {
				failedOpenEvents = append(failedOpenEvents, event)
			}
		},
	})
	if err := p.Open(context.Background()); err != nil {
		t.Fatalf("Open initial provider: %v", err)
	}
	got, err := p.Capture(context.Background(), camera.CaptureRequest{
		Sequence: 3, Width: 1280, Height: 720, OutputPath: filepath.Join(t.TempDir(), "capture.jpg"),
	})
	if err != nil {
		t.Fatalf("Capture should recover within the configured recovery budget: %v", err)
	}
	if got.Sequence != 3 || healthy.captureCalls != 1 {
		t.Fatalf("result=%+v healthy capture calls=%d", got, healthy.captureCalls)
	}
	if factoryCalls != 5 {
		t.Fatalf("factory calls=%d, want initial + 4 reconnect attempts", factoryCalls)
	}
	if len(failedOpenEvents) != 3 {
		t.Fatalf("failed-open events=%d, want 3 before the fourth reconnect succeeds", len(failedOpenEvents))
	}
	for i, event := range failedOpenEvents {
		if event.Attempt != i+1 || event.Total != 4 {
			t.Fatalf("failed-open event %d=%+v, want attempt %d of 4", i, event, i+1)
		}
	}
}

func TestOpenRetriesReuseRetryableProviderAndReleaseItAfterExhaustion(t *testing.T) {
	provider := &retryableStubProvider{stubProvider: &stubProvider{
		name:    "thread-affine camera",
		openErr: errors.New("device sources unavailable"),
	}}
	factoryCalls := 0
	p := New(func() camera.Provider {
		factoryCalls++
		return provider
	}, Options{OpenAttempts: 3, Sleep: noWait})

	err := p.Open(context.Background())
	if err == nil || !strings.Contains(err.Error(), "camera open failed after 3 attempt(s)") {
		t.Fatalf("Open error=%v, want bounded retry failure", err)
	}
	if factoryCalls != 1 {
		t.Fatalf("factory calls=%d, want one provider reused across retry attempts", factoryCalls)
	}
	if provider.openCalls != 3 {
		t.Fatalf("open calls=%d, want three Open calls on the same provider", provider.openCalls)
	}
	if provider.closeCalls != 1 {
		t.Fatalf("close calls=%d, want one final cleanup after retries are exhausted", provider.closeCalls)
	}
}

func TestRetryableOpenCleanupFailurePreservesProviderForCloseRetry(t *testing.T) {
	openErr := errors.New("device sources unavailable")
	closeErr := errors.New("synthetic retained-thread release failure")
	provider := &retryableStubProvider{stubProvider: &stubProvider{
		name:     "thread-affine camera",
		openErr:  openErr,
		closeErr: closeErr,
	}}
	p := New(func() camera.Provider { return provider }, Options{
		OpenAttempts: 1,
		Sleep:        noWait,
	})

	err := p.Open(context.Background())
	if !errors.Is(err, openErr) || !errors.Is(err, closeErr) {
		t.Fatalf("Open error=%v, want both startup and cleanup errors", err)
	}
	if p.active != provider {
		t.Fatal("provider reference was lost after retryable cleanup failed")
	}

	provider.closeErr = nil
	if err := p.Close(); err != nil {
		t.Fatalf("Close retry after transient cleanup failure: %v", err)
	}
	if p.active != nil || provider.closeCalls != 2 {
		t.Fatalf("active=%T closeCalls=%d, want nil and two Close calls", p.active, provider.closeCalls)
	}
}
