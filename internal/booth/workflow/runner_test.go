package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dongsinhho/PhotoBooth/internal/booth/camera"
	"github.com/dongsinhho/PhotoBooth/internal/booth/camera/fake"
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
