package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dongsinhho/PhotoBooth/internal/booth/camera"
	"github.com/dongsinhho/PhotoBooth/internal/booth/camera/fake"
	"github.com/dongsinhho/PhotoBooth/internal/booth/camera/recovery"
	"github.com/dongsinhho/PhotoBooth/internal/booth/output"
	"github.com/dongsinhho/PhotoBooth/internal/booth/render"
	"github.com/dongsinhho/PhotoBooth/internal/booth/session"
)

func testFrameConfig(t *testing.T) string {
	t.Helper()
	cfg := render.Config{
		SchemaVersion: 1,
		Canvas:        render.CanvasConfig{Width: 320, Height: 480, Background: "#FFFFFF"},
		Slots: []render.SlotConfig{
			{ID: "one", X: 10, Y: 10, Width: 300, Height: 140, Fit: "cover"},
			{ID: "two", X: 10, Y: 170, Width: 300, Height: 140, Fit: "cover"},
			{ID: "three", X: 10, Y: 330, Width: 300, Height: 140, Fit: "cover"},
		},
	}
	path := filepath.Join(t.TempDir(), "frame.json")
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func newTestRunner(t *testing.T, cam *fake.Provider, cfgPath string, onEvent func(Event)) (*Runner, Config, string, string) {
	t.Helper()
	root := t.TempDir()
	workDir := filepath.Join(root, "sessions")
	outputDir := filepath.Join(root, "output")
	runner := &Runner{Camera: cam, Renderer: render.New(), Output: output.NewLocal(outputDir), OnEvent: onEvent,
		Sleep: func(ctx context.Context, _ time.Duration) error { return ctx.Err() }}
	cfg := Config{FrameConfigPath: cfgPath, WorkDir: workDir, CaptureCount: 3, Countdown: 0, Interval: 0, CaptureWidth: 640, CaptureHeight: 480,
		SessionID: "0123456789abcdef0123456789abcdef"}
	return runner, cfg, workDir, outputDir
}

func TestRunCompletesOfflineCaptureRenderSaveFlow(t *testing.T) {
	cfgPath := testFrameConfig(t)
	cam := fake.New()
	var states []session.State
	countdownEvents := 0
	runner, cfg, workDir, outputDir := newTestRunner(t, cam, cfgPath, func(e Event) {
		if e.Type == "state" {
			states = append(states, e.State)
		}
		if e.Type == "countdown" {
			countdownEvents++
		}
	})
	result, err := runner.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.State != session.Completed {
		t.Fatalf("state=%s, want completed", result.State)
	}
	if len(result.CaptureDurations) != 3 {
		t.Fatalf("capture duration count=%d, want 3", len(result.CaptureDurations))
	}
	for i, duration := range result.CaptureDurations {
		if duration < 0 {
			t.Fatalf("capture duration %d is negative: %s", i+1, duration)
		}
	}
	if result.RenderDuration < 0 || result.OutputDuration < 0 || result.TotalDuration < 0 {
		t.Fatalf("phase durations must be non-negative: render=%s output=%s total=%s", result.RenderDuration, result.OutputDuration, result.TotalDuration)
	}
	if len(result.CapturePaths) != 3 {
		t.Fatalf("captures=%d, want 3", len(result.CapturePaths))
	}
	if countdownEvents != 3 {
		t.Fatalf("countdown events=%d, want 3", countdownEvents)
	}
	want := []session.State{session.Preflight, session.Capturing, session.Rendering, session.Output, session.Completed}
	if len(states) != len(want) {
		t.Fatalf("states=%v, want %v", states, want)
	}
	for i := range want {
		if states[i] != want[i] {
			t.Fatalf("states=%v, want %v", states, want)
		}
	}
	for _, p := range result.CapturePaths {
		info, err := os.Stat(p)
		if err != nil || info.Size() == 0 {
			t.Fatalf("capture missing/empty: path=%s err=%v", p, err)
		}
	}
	if result.RenderedPath == "" {
		t.Fatal("missing rendered path")
	}
	out, err := os.Stat(result.OutputPath)
	if err != nil || out.Size() == 0 {
		t.Fatalf("output missing/empty: path=%s err=%v", result.OutputPath, err)
	}
	if filepath.Dir(result.OutputPath) != filepath.Join(outputDir, result.SessionID) {
		t.Fatalf("output is not session-scoped: %s", result.OutputPath)
	}
	if _, err := os.Stat(filepath.Join(workDir, result.SessionID, "captures", "capture-03.jpg")); err != nil {
		t.Fatalf("third capture missing: %v", err)
	}
}

func TestCaptureFailureStopsSessionAndClosesCamera(t *testing.T) {
	cam := fake.New()
	cam.FailAt = 2
	runner, cfg, _, _ := newTestRunner(t, cam, testFrameConfig(t), nil)
	result, err := runner.Run(context.Background(), cfg)
	if err == nil || result.State != session.Failed {
		t.Fatalf("result=%+v err=%v, want failed session", result, err)
	}
	if len(result.CapturePaths) != 1 {
		t.Fatalf("captures before failure=%d, want 1", len(result.CapturePaths))
	}
	if _, err := cam.Capture(context.Background(), camera.CaptureRequest{Sequence: 1, Width: 640, Height: 480, OutputPath: filepath.Join(t.TempDir(), "capture.jpg")}); err == nil {
		t.Fatal("expected fake camera to be closed after failed session")
	}
}

type failingOutput struct {
	targetName string
}

func (f failingOutput) Name() string {
	return f.targetName
}

func (f failingOutput) Execute(context.Context, output.Job) (output.TargetOutput, error) {
	return output.TargetOutput{}, errors.New("simulated output failure")
}

type successfulPrinter struct{}

func (successfulPrinter) Name() string { return output.TargetPrinter }
func (successfulPrinter) Execute(context.Context, output.Job) (output.TargetOutput, error) {
	return output.TargetOutput{Status: output.TargetQueued, JobID: "print-job-test-123"}, nil
}

func TestOutputFailureMarksSessionFailed(t *testing.T) {
	cam := fake.New()
	runner, cfg, _, _ := newTestRunner(t, cam, testFrameConfig(t), nil)
	runner.Output = output.NewCoordinator(failingOutput{targetName: output.TargetLocalStorage})
	result, err := runner.Run(context.Background(), cfg)
	if err == nil || result.State != session.Failed {
		t.Fatalf("result=%+v err=%v, want failed session", result, err)
	}
	if len(result.OutputResults) != 1 || result.OutputResults[0].Status != output.TargetFailed {
		t.Fatalf("unexpected target results: %+v", result.OutputResults)
	}
}

func TestLocalOutputSucceedsWhenPrinterFails(t *testing.T) {
	cam := fake.New()
	runner, cfg, _, outputDir := newTestRunner(t, cam, testFrameConfig(t), nil)
	runner.Output = output.NewCoordinator(
		output.NewLocal(outputDir),
		failingOutput{targetName: output.TargetPrinter},
	)
	result, err := runner.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.State != session.PartialSuccess {
		t.Fatalf("state=%s, want partial_success", result.State)
	}
	if result.OutputPath == "" {
		t.Fatal("local output should succeed independently of printer failure")
	}
	if _, err := os.Stat(result.OutputPath); err != nil {
		t.Fatalf("local output missing: %v", err)
	}
	if len(result.OutputResults) != 2 ||
		result.OutputResults[0].Status != output.TargetSucceeded ||
		result.OutputResults[1].Status != output.TargetFailed {
		t.Fatalf("unexpected per-target outcomes: %+v", result.OutputResults)
	}
}

func TestPrinterCanSucceedWhenLocalOutputFails(t *testing.T) {
	cam := fake.New()
	runner, cfg, _, _ := newTestRunner(t, cam, testFrameConfig(t), nil)
	runner.Output = output.NewCoordinator(
		failingOutput{targetName: output.TargetLocalStorage},
		successfulPrinter{},
	)
	result, err := runner.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.State != session.PartialSuccess {
		t.Fatalf("state=%s, want partial_success", result.State)
	}
	if result.OutputPath != "" {
		t.Fatalf("local output path=%q, expected no local file", result.OutputPath)
	}
	if len(result.OutputResults) != 2 ||
		result.OutputResults[0].Status != output.TargetFailed ||
		result.OutputResults[1].Status != output.TargetQueued ||
		result.OutputResults[1].JobID != "print-job-test-123" {
		t.Fatalf("unexpected per-target outcomes: %+v", result.OutputResults)
	}
}

func TestInvalidFrameFailsBeforeCameraOpens(t *testing.T) {
	cam := fake.New()
	runner, cfg, _, _ := newTestRunner(t, cam, filepath.Join(t.TempDir(), "missing.json"), nil)
	result, err := runner.Run(context.Background(), cfg)
	if err == nil || result.State != session.Failed {
		t.Fatalf("result=%+v err=%v, want failed preflight", result, err)
	}
	if _, err := cam.Capture(context.Background(), camera.CaptureRequest{Sequence: 1, Width: 640, Height: 480, OutputPath: filepath.Join(t.TempDir(), "capture.jpg")}); err == nil {
		t.Fatal("camera should remain closed if preflight fails")
	}
}

func TestFrameSlotCountMismatchFailsBeforeCameraOpens(t *testing.T) {
	cam := fake.New()
	runner, cfg, _, _ := newTestRunner(t, cam, testFrameConfig(t), nil)
	cfg.CaptureCount = 2
	result, err := runner.Run(context.Background(), cfg)
	if err == nil || result.State != session.Failed {
		t.Fatalf("result=%+v err=%v, want preflight failure", result, err)
	}
	if len(result.CapturePaths) != 0 {
		t.Fatalf("captured %d images before detecting configuration mismatch", len(result.CapturePaths))
	}
	if _, err := cam.Capture(context.Background(), camera.CaptureRequest{Sequence: 1, Width: 640, Height: 480, OutputPath: filepath.Join(t.TempDir(), "capture.jpg")}); err == nil {
		t.Fatal("camera should remain closed after preflight rejects slot mismatch")
	}
}

func TestCancelledContextMarksSessionCancelled(t *testing.T) {
	cam := fake.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner, cfg, _, _ := newTestRunner(t, cam, testFrameConfig(t), func(e Event) {
		if e.Type == "state" && e.State == session.Capturing {
			cancel()
		}
	})
	result, err := runner.Run(ctx, cfg)
	if !errors.Is(err, context.Canceled) || result.State != session.Cancelled {
		t.Fatalf("result=%+v err=%v, want cancelled session", result, err)
	}
	if len(result.CapturePaths) != 0 {
		t.Fatalf("captured %d shots after cancellation", len(result.CapturePaths))
	}
	if _, err := cam.Capture(context.Background(), camera.CaptureRequest{Sequence: 1, Width: 640, Height: 480, OutputPath: filepath.Join(t.TempDir(), "capture.jpg")}); err == nil {
		t.Fatal("camera should be closed after cancellation")
	}
}

func TestExpiredContextMarksSessionTimedOut(t *testing.T) {
	cam := fake.New()
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	runner, cfg, _, _ := newTestRunner(t, cam, testFrameConfig(t), nil)
	result, err := runner.Run(ctx, cfg)
	if !errors.Is(err, context.DeadlineExceeded) || result.State != session.TimedOut {
		t.Fatalf("result=%+v err=%v, want timed out session", result, err)
	}
}

func TestRepeatedSessionsCloseCameraAndKeepOutputsIsolated(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "sessions")
	outputDir := filepath.Join(root, "output")
	framePath := testFrameConfig(t)

	const sessionCount = 25
	for i := 0; i < sessionCount; i++ {
		cam := fake.New()
		runner := &Runner{
			Camera: cam, Renderer: render.New(), Output: output.NewLocal(outputDir),
			Sleep: func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
		}
		cfg := Config{
			FrameConfigPath: framePath,
			WorkDir:         workDir,
			CaptureCount:    3,
			Countdown:       0,
			Interval:        0,
			CaptureWidth:    640,
			CaptureHeight:   480,
			SessionID:       fmt.Sprintf("%032x", i+1),
		}
		result, err := runner.Run(context.Background(), cfg)
		if err != nil {
			t.Fatalf("session %d: Run: %v", i+1, err)
		}
		if result.State != session.Completed {
			t.Fatalf("session %d state=%s, want completed", i+1, result.State)
		}
		if len(result.CapturePaths) != 3 || len(result.OutputResults) != 1 {
			t.Fatalf("session %d has incomplete artifacts: captures=%d targets=%+v", i+1, len(result.CapturePaths), result.OutputResults)
		}
		if _, err := os.Stat(result.OutputPath); err != nil {
			t.Fatalf("session %d output is missing: %v", i+1, err)
		}
		if _, err := cam.Capture(context.Background(), camera.CaptureRequest{
			Sequence: 1, Width: 640, Height: 480,
			OutputPath: filepath.Join(root, "must-not-exist.jpg"),
		}); err == nil {
			t.Fatalf("session %d did not release the camera after completion", i+1)
		}
	}
	sessions, err := os.ReadDir(workDir)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != sessionCount || len(outputs) != sessionCount {
		t.Fatalf("session directories=%d output directories=%d; want %d each", len(sessions), len(outputs), sessionCount)
	}
}

type blockingCamera struct {
	mu     sync.Mutex
	opened bool
	closed bool
}

func (c *blockingCamera) Info() camera.Info {
	return camera.Info{Name: "Blocking test camera", Backend: "test"}
}
func (c *blockingCamera) Open(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.opened = true
	return nil
}
func (c *blockingCamera) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.opened = false
	c.closed = true
	return nil
}
func (c *blockingCamera) Capture(ctx context.Context, _ camera.CaptureRequest) (camera.CaptureResult, error) {
	c.mu.Lock()
	opened := c.opened
	c.mu.Unlock()
	if !opened {
		return camera.CaptureResult{}, errors.New("blocking camera is not open")
	}
	<-ctx.Done()
	return camera.CaptureResult{}, ctx.Err()
}
func (c *blockingCamera) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed && !c.opened
}

func TestDeadlineDuringCaptureClosesCameraAndTimesOutSession(t *testing.T) {
	cam := &blockingCamera{}
	runner := &Runner{
		Camera: cam, Renderer: render.New(), Output: output.NewLocal(t.TempDir()),
		Sleep: func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
	}
	cfg := Config{
		FrameConfigPath: testFrameConfig(t),
		WorkDir:         filepath.Join(t.TempDir(), "sessions"),
		CaptureCount:    3,
		CaptureWidth:    640,
		CaptureHeight:   480,
		SessionID:       "abcdef0123456789abcdef0123456789",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	result, err := runner.Run(ctx, cfg)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run err=%v, want context deadline exceeded", err)
	}
	if result.State != session.TimedOut {
		t.Fatalf("state=%s, want timed_out", result.State)
	}
	if !cam.isClosed() {
		t.Fatal("camera must be closed when cancellation interrupts capture")
	}
	if len(result.CapturePaths) != 0 {
		t.Fatalf("captured %d frames even though capture was cancelled", len(result.CapturePaths))
	}
}

type streamUnavailableProvider struct {
	camera.Provider
}

func (p *streamUnavailableProvider) Capture(ctx context.Context, req camera.CaptureRequest) (camera.CaptureResult, error) {
	if err := ctx.Err(); err != nil {
		return camera.CaptureResult{}, err
	}
	return camera.CaptureResult{}, camera.ErrStreamUnavailable
}

func TestRunCompletesCaptureRenderSaveAfterCameraRecovery(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "sessions")
	outputDir := filepath.Join(root, "output")
	factoryCalls := 0
	var recoveryEvents []recovery.Event
	recoveredCamera := recovery.New(func() camera.Provider {
		factoryCalls++
		if factoryCalls == 1 {
			return &streamUnavailableProvider{Provider: fake.New()}
		}
		return fake.New()
	}, recovery.Options{
		OpenAttempts:     1,
		RecoveryAttempts: 1,
		Sleep:            func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
		OnEvent:          func(event recovery.Event) { recoveryEvents = append(recoveryEvents, event) },
	})
	runner := &Runner{
		Camera: recoveredCamera, Renderer: render.New(), Output: output.NewLocal(outputDir),
		Sleep: func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
	}
	cfg := Config{
		FrameConfigPath: testFrameConfig(t),
		WorkDir:         workDir,
		CaptureCount:    3,
		CaptureWidth:    640,
		CaptureHeight:   480,
		SessionID:       "fedcba9876543210fedcba9876543210",
	}

	result, err := runner.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run after camera recovery: %v", err)
	}
	if result.State != session.Completed {
		t.Fatalf("session state=%s, want completed", result.State)
	}
	if len(result.CapturePaths) != 3 {
		t.Fatalf("captures=%d, want 3", len(result.CapturePaths))
	}
	if _, err := os.Stat(result.OutputPath); err != nil {
		t.Fatalf("final output missing after camera recovery: %v", err)
	}
	if factoryCalls != 2 {
		t.Fatalf("camera factory calls=%d, want original and replacement providers", factoryCalls)
	}
	recovered := false
	for _, event := range recoveryEvents {
		if event.Type == "recovery_succeeded" {
			recovered = true
			break
		}
	}
	if !recovered {
		t.Fatalf("recovery success event not observed: %+v", recoveryEvents)
	}
}

func TestRunResumesExistingCapturesWithoutOverwritingThem(t *testing.T) {
	cfgPath := testFrameConfig(t)
	tmp := t.TempDir()
	workDir := filepath.Join(tmp, "sessions")
	outputDir := filepath.Join(tmp, "output")
	sessionID := "abcdef0123456789abcdef0123456789"
	captureDir := filepath.Join(workDir, sessionID, "captures")
	if err := os.MkdirAll(captureDir, 0o700); err != nil {
		t.Fatal(err)
	}
	firstPath := filepath.Join(captureDir, "capture-01.jpg")
	firstCamera := fake.New()
	if err := firstCamera.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := firstCamera.Capture(context.Background(), camera.CaptureRequest{
		Sequence: 1, Width: 640, Height: 480, OutputPath: firstPath,
	}); err != nil {
		t.Fatalf("prepare prior capture: %v", err)
	}
	if err := firstCamera.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatal(err)
	}

	cam := fake.New()
	runner := &Runner{
		Camera: cam, Renderer: render.New(), Output: output.NewLocal(outputDir),
		Sleep: func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
	}
	result, err := runner.Run(context.Background(), Config{
		FrameConfigPath: cfgPath, WorkDir: workDir, CaptureCount: 3,
		Countdown: 0, Interval: 0, CaptureWidth: 640, CaptureHeight: 480,
		SessionID: sessionID, ResumeCaptureCount: 1,
	})
	if err != nil {
		t.Fatalf("Run resumed session: %v", err)
	}
	if result.State != session.Completed {
		t.Fatalf("state=%s, want completed", result.State)
	}
	if len(result.CapturePaths) != 3 || len(result.CaptureDurations) != 2 {
		t.Fatalf("captures=%d durations=%d; want 3 paths with only 2 new capture durations", len(result.CapturePaths), len(result.CaptureDurations))
	}
	if result.CapturePaths[0] != firstPath {
		t.Fatalf("first capture path=%q, want preserved path %q", result.CapturePaths[0], firstPath)
	}
	after, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("resume overwrote the already completed first capture")
	}
	for _, name := range []string{"capture-02.jpg", "capture-03.jpg"} {
		path := filepath.Join(captureDir, name)
		info, err := os.Stat(path)
		if err != nil || info.Size() == 0 {
			t.Fatalf("missing resumed capture %s: info=%v err=%v", name, info, err)
		}
	}
	if _, err := os.Stat(result.OutputPath); err != nil {
		t.Fatalf("final output missing after resume: %v", err)
	}
}

type insufficientStorageOutput struct{}

func (insufficientStorageOutput) Name() string { return output.TargetLocalStorage }
func (insufficientStorageOutput) Execute(context.Context, output.Job) (output.TargetOutput, error) {
	return output.TargetOutput{}, fmt.Errorf("%w: simulated disk full; free disk space and retry. Captures and the rendered session file are preserved.", output.ErrInsufficientStorage)
}

func TestDiskFullOutputFailurePreservesCapturesAndRenderedSession(t *testing.T) {
	cam := fake.New()
	var failedEvent Event
	runner, cfg, workDir, _ := newTestRunner(t, cam, testFrameConfig(t), func(event Event) {
		if event.Type == "failed" {
			failedEvent = event
		}
	})
	runner.Output = output.NewCoordinator(insufficientStorageOutput{})

	result, err := runner.Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("Run unexpectedly succeeded with simulated insufficient storage")
	}
	if !errors.Is(err, output.ErrInsufficientStorage) {
		t.Fatalf("Run error %v must preserve ErrInsufficientStorage", err)
	}
	if result.State != session.Failed {
		t.Fatalf("session state=%s, want failed", result.State)
	}
	if len(result.CapturePaths) != cfg.CaptureCount {
		t.Fatalf("capture count=%d, want %d", len(result.CapturePaths), cfg.CaptureCount)
	}
	for i, capturePath := range result.CapturePaths {
		info, statErr := os.Stat(capturePath)
		if statErr != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			t.Fatalf("capture %d not preserved: path=%s err=%v", i+1, capturePath, statErr)
		}
	}
	if result.RenderedPath == "" {
		t.Fatal("rendered source path is empty after output failure")
	}
	renderedInfo, statErr := os.Stat(result.RenderedPath)
	if statErr != nil || !renderedInfo.Mode().IsRegular() || renderedInfo.Size() == 0 {
		t.Fatalf("rendered session file not preserved: path=%s err=%v", result.RenderedPath, statErr)
	}
	if result.OutputPath != "" {
		t.Fatalf("failed local output must not publish a path: %q", result.OutputPath)
	}
	if len(result.OutputResults) != 1 || result.OutputResults[0].Status != output.TargetFailed {
		t.Fatalf("unexpected output results: %+v", result.OutputResults)
	}
	if !strings.Contains(result.OutputResults[0].Error, "insufficient storage") {
		t.Fatalf("target error lacks disk-full classification: %+v", result.OutputResults[0])
	}
	if failedEvent.Type != "failed" || !strings.Contains(failedEvent.Message, "free disk space") {
		t.Fatalf("operator failure event lacks actionable storage guidance: %+v", failedEvent)
	}
	if _, err := os.Stat(filepath.Join(workDir, result.SessionID, "captures", "capture-03.jpg")); err != nil {
		t.Fatalf("third source capture was removed: %v", err)
	}
}
