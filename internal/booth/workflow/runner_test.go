package workflow

import (
	"bytes"
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
	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
	"github.com/dongsinhho/PhotoBooth/internal/booth/output"
	"github.com/dongsinhho/PhotoBooth/internal/booth/payment"
	"github.com/dongsinhho/PhotoBooth/internal/booth/render"
	"github.com/dongsinhho/PhotoBooth/internal/booth/session"
)

func truncateWorkflowJPEGScanPayload(t *testing.T, data []byte) []byte {
	t.Helper()
	marker := bytes.Index(data, []byte{0xff, 0xda})
	if marker < 0 || marker+4 > len(data) {
		t.Fatal("JPEG fixture has no complete start-of-scan marker")
	}
	segmentLength := int(data[marker+2])<<8 | int(data[marker+3])
	scanStart := marker + 2 + segmentLength
	if scanStart >= len(data)-2 {
		t.Fatalf("JPEG fixture has no scan payload to truncate: size=%d scan_start=%d", len(data), scanStart)
	}
	truncated := append([]byte(nil), data[:scanStart]...)
	return append(truncated, 0xff, 0xd9)
}

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
	cfg := Config{FrameConfigPath: cfgPath, WorkDir: workDir, OutputDir: outputDir, CaptureCount: 3, Countdown: 0, Interval: 0, CaptureWidth: 640, CaptureHeight: 480,
		SessionID: "0123456789abcdef0123456789abcdef"}
	return runner, cfg, workDir, outputDir
}

func writeWorkflowTestManifest(t *testing.T, sessionDir string, cfg Config, phase session.Phase, completed int) {
	t.Helper()
	manifest, err := NewSessionManifest(cfg)
	if err != nil {
		t.Fatalf("build test manifest: %v", err)
	}
	// Existing resume fixtures exercise backwards-compatible manifest v1.
	manifest.Version = session.LegacyManifestVersion
	manifest.Phase = phase
	manifest.CompletedShots = completed
	manifest.UpdatedAt = time.Now().UTC()
	if phase == session.PhaseRendered {
		manifest.RenderedAt = manifest.UpdatedAt
	}
	if phase == session.PhaseOutputFinalized {
		manifest.RenderedAt = manifest.UpdatedAt
		manifest.OutputFinalizedAt = manifest.UpdatedAt
		manifest.OutputPath = filepath.Join(filepath.Dir(sessionDir), "output", cfg.SessionID, "photo.jpg")
	}
	if err := session.WriteManifest(sessionDir, manifest); err != nil {
		t.Fatalf("write test manifest: %v", err)
	}
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
	gate := &recordingPaymentGate{}
	runner.PaymentGate = gate
	result, err := runner.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(gate.requests) != 1 || gate.requests[0].SessionID != result.SessionID || gate.requests[0].Resume || gate.requests[0].DemoMode {
		t.Fatalf("new-session payment request=%+v, want one request for the new live session", gate.requests)
	}
	if result.State != session.Completed {
		t.Fatalf("state=%s, want completed", result.State)
	}
	sessionDir := filepath.Join(workDir, result.SessionID)
	manifest, manifestErr := session.ReadManifest(sessionDir)
	if manifestErr != nil {
		t.Fatalf("read new-session manifest: %v", manifestErr)
	}
	if manifest.Version != session.ManifestVersion || manifest.FrameSnapshotFile != session.FrameSnapshotFileName {
		t.Fatalf("new session must persist schema-v2 frame snapshot metadata: %+v", manifest)
	}
	_, snapshotBytes, snapshotErr := session.ReadFrameSnapshot(sessionDir, manifest)
	if snapshotErr != nil {
		t.Fatalf("read new-session frame snapshot: %v", snapshotErr)
	}
	originalFrameBytes, frameErr := os.ReadFile(cfgPath)
	if frameErr != nil || !bytes.Equal(snapshotBytes, originalFrameBytes) {
		t.Fatalf("new-session frame snapshot differs from selected frame: err=%v", frameErr)
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
	for index, capturePath := range result.CapturePaths {
		wantPath := filepath.Join(workDir, result.SessionID, "captures", fmt.Sprintf("capture-%02d.jpg", index+1))
		if filepath.Clean(capturePath) != filepath.Clean(wantPath) {
			t.Fatalf("capture %d path=%q, want committed path %q", index+1, capturePath, wantPath)
		}
	}
	stagingPath := filepath.Join(workDir, result.SessionID, "captures", session.CaptureStagingDirectoryName)
	if _, err := os.Lstat(stagingPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("successful session left capture staging artifacts: %v", err)
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

func TestManifestFailureAfterCameraOpenClosesCamera(t *testing.T) {
	runner, cfg, _, _ := newTestRunner(t, fake.New(), testFrameConfig(t), nil)
	cam := fake.New()
	runner.Camera = cam
	runner.writeManifest = func(sessionDir string, manifest session.Manifest) error {
		if manifest.Phase == session.PhaseCapturing {
			return errors.New("injected manifest persistence failure")
		}
		return session.WriteManifest(sessionDir, manifest)
	}

	result, err := runner.Run(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "injected manifest persistence failure") {
		t.Fatalf("Run error=%v, want injected capturing-manifest failure", err)
	}
	if result.State != session.Failed {
		t.Fatalf("state=%s, want failed", result.State)
	}
	if _, captureErr := cam.Capture(context.Background(), camera.CaptureRequest{
		Sequence: 1, Width: 640, Height: 480, OutputPath: filepath.Join(t.TempDir(), "must-not-exist.jpg"),
	}); captureErr == nil {
		t.Fatal("camera remained open after capturing-manifest persistence failed")
	}
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

func TestNewSessionRefusesExistingWorkspaceWithoutOverwritingIt(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "sessions")
	outputDir := filepath.Join(root, "output")
	sessionID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	sessionDir := filepath.Join(workDir, sessionID)
	captureDir := filepath.Join(sessionDir, "captures")
	if err := os.MkdirAll(captureDir, 0o700); err != nil {
		t.Fatal(err)
	}
	prior := []byte("previous session artifact")
	priorPath := filepath.Join(sessionDir, "rendered.jpg")
	if err := os.WriteFile(priorPath, prior, 0o600); err != nil {
		t.Fatal(err)
	}

	cam := fake.New()
	runner := &Runner{Camera: cam, Renderer: render.New(), Output: output.NewLocal(outputDir)}
	result, err := runner.Run(context.Background(), Config{
		FrameConfigPath: testFrameConfig(t), WorkDir: workDir, OutputDir: outputDir,
		CaptureCount: 3, CaptureWidth: 640, CaptureHeight: 480, SessionID: sessionID,
	})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Run error=%v, want existing workspace refusal", err)
	}
	if result.State != session.Failed {
		t.Fatalf("result state=%s, want failed", result.State)
	}
	got, readErr := os.ReadFile(priorPath)
	if readErr != nil || string(got) != string(prior) {
		t.Fatalf("existing session artifact changed: got=%q err=%v", got, readErr)
	}
	if _, err := cam.Capture(context.Background(), camera.CaptureRequest{
		Sequence: 1, Width: 640, Height: 480, OutputPath: filepath.Join(root, "must-not-exist.jpg"),
	}); err == nil {
		t.Fatal("camera should not have been opened for a colliding session ID")
	}
}

func TestResumeSessionWithZeroCompletedCapturesUsesExistingEmptyWorkspace(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "sessions")
	outputDir := filepath.Join(root, "output")
	sessionID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	captureDir := filepath.Join(workDir, sessionID, "captures")
	if err := os.MkdirAll(captureDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cam := fake.New()
	runner := &Runner{Camera: cam, Renderer: render.New(), Output: output.NewLocal(outputDir),
		Sleep: func(ctx context.Context, _ time.Duration) error { return ctx.Err() }}
	gate := &recordingPaymentGate{}
	runner.PaymentGate = gate
	cfg := Config{
		FrameConfigPath: testFrameConfig(t), WorkDir: workDir, OutputDir: outputDir,
		CaptureCount: 3, CaptureWidth: 640, CaptureHeight: 480,
		SessionID: sessionID, ResumeSession: true, ResumeCaptureCount: 0,
	}
	writeWorkflowTestManifest(t, filepath.Dir(captureDir), cfg, session.PhasePreflight, 0)
	result, err := runner.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("resume zero-capture session: %v", err)
	}
	if result.State != session.Completed || len(result.CapturePaths) != 3 {
		t.Fatalf("unexpected resumed result: state=%s captures=%d", result.State, len(result.CapturePaths))
	}
	if len(gate.requests) != 1 || gate.requests[0].SessionID != sessionID || !gate.requests[0].Resume || gate.requests[0].DemoMode {
		t.Fatalf("resume payment request=%+v, want one idempotent authorization request for the stable session ID", gate.requests)
	}
}

type openTrackingCamera struct {
	provider  *fake.Provider
	openCalls int
}

func (c *openTrackingCamera) Info() camera.Info { return c.provider.Info() }
func (c *openTrackingCamera) Open(ctx context.Context) error {
	c.openCalls++
	return c.provider.Open(ctx)
}
func (c *openTrackingCamera) Close() error { return c.provider.Close() }
func (c *openTrackingCamera) Capture(ctx context.Context, req camera.CaptureRequest) (camera.CaptureResult, error) {
	return c.provider.Capture(ctx, req)
}

func TestResumeSessionRejectsInvalidJPEGBeforeOpeningCamera(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "sessions")
	outputDir := filepath.Join(root, "output")
	sessionID := "cccccccccccccccccccccccccccccccc"
	captureDir := filepath.Join(workDir, sessionID, "captures")
	if err := os.MkdirAll(captureDir, 0o700); err != nil {
		t.Fatal(err)
	}
	capturePath := filepath.Join(captureDir, "capture-01.jpg")
	fixtureCamera := fake.New()
	if err := fixtureCamera.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := fixtureCamera.Capture(context.Background(), camera.CaptureRequest{
		Sequence: 1, Width: 640, Height: 480, OutputPath: capturePath,
	}); err != nil {
		t.Fatal(err)
	}
	if err := fixtureCamera.Close(); err != nil {
		t.Fatal(err)
	}
	validJPEG, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatal(err)
	}
	truncatedJPEG := truncateWorkflowJPEGScanPayload(t, validJPEG)
	if err := os.WriteFile(capturePath, truncatedJPEG, 0o600); err != nil {
		t.Fatal(err)
	}

	cam := &openTrackingCamera{provider: fake.New()}
	runner := &Runner{Camera: cam, Renderer: render.New(), Output: output.NewLocal(outputDir)}
	cfg := Config{
		FrameConfigPath: testFrameConfig(t), WorkDir: workDir, OutputDir: outputDir,
		CaptureCount: 3, CaptureWidth: 640, CaptureHeight: 480,
		SessionID: sessionID, ResumeSession: true, ResumeCaptureCount: 1,
	}
	writeWorkflowTestManifest(t, filepath.Dir(captureDir), cfg, session.PhaseCapturing, 1)
	if _, err := runner.Run(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "JPEG") {
		t.Fatalf("Run error=%v, want invalid JPEG refusal", err)
	}
	if cam.openCalls != 0 {
		t.Fatalf("camera Open calls=%d, want 0 for a truncated preserved JPEG", cam.openCalls)
	}
	if _, err := os.Stat(filepath.Join(captureDir, "capture-02.jpg")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("resume wrote a new capture before validating the existing JPEG: %v", err)
	}
}

func TestResumeSessionRejectsUnexpectedExistingCaptureEntries(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "sessions")
	outputDir := filepath.Join(root, "output")
	sessionID := "cccccccccccccccccccccccccccccccc"
	captureDir := filepath.Join(workDir, sessionID, "captures")
	if err := os.MkdirAll(captureDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(captureDir, "capture-01.jpg"), []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(captureDir, "unexpected.tmp"), []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	cam := fake.New()
	runner := &Runner{Camera: cam, Renderer: render.New(), Output: output.NewLocal(outputDir)}
	cfg := Config{
		FrameConfigPath: testFrameConfig(t), WorkDir: workDir, OutputDir: outputDir,
		CaptureCount: 3, CaptureWidth: 640, CaptureHeight: 480,
		SessionID: sessionID, ResumeSession: true, ResumeCaptureCount: 1,
	}
	writeWorkflowTestManifest(t, filepath.Dir(captureDir), cfg, session.PhaseCapturing, 1)
	_, err := runner.Run(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "unexpected capture entry") {
		t.Fatalf("Run error=%v, want unexpected capture entry refusal", err)
	}
	if _, err := cam.Capture(context.Background(), camera.CaptureRequest{
		Sequence: 1, Width: 640, Height: 480, OutputPath: filepath.Join(root, "must-not-exist.jpg"),
	}); err == nil {
		t.Fatal("camera should not have been opened for an unsafe resume workspace")
	}
}

func TestRunRejectsOverlappingSessionAndOutputRootsBeforeCreatingSession(t *testing.T) {
	root := t.TempDir()
	sessions := filepath.Join(root, "sessions")
	outputDir := filepath.Join(sessions, "photos")
	sessionID := "dddddddddddddddddddddddddddddddd"
	cam := fake.New()
	runner := &Runner{Camera: cam, Renderer: render.New(), Output: output.NewLocal(outputDir)}
	_, err := runner.Run(context.Background(), Config{
		FrameConfigPath: testFrameConfig(t), WorkDir: sessions, OutputDir: outputDir,
		CaptureCount: 3, CaptureWidth: 640, CaptureHeight: 480, SessionID: sessionID,
	})
	if err == nil || !strings.Contains(err.Error(), "non-overlapping roots") {
		t.Fatalf("Run error=%v, want root overlap rejection", err)
	}
	if _, err := os.Stat(filepath.Join(sessions, sessionID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("session workspace created despite root overlap: %v", err)
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
			OutputDir:       outputDir,
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
	root := t.TempDir()
	outputDir := filepath.Join(root, "output")
	runner := &Runner{
		Camera: cam, Renderer: render.New(), Output: output.NewLocal(outputDir),
		Sleep: func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
	}
	cfg := Config{
		FrameConfigPath: testFrameConfig(t),
		WorkDir:         filepath.Join(root, "sessions"),
		OutputDir:       filepath.Join(root, "output"),
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
		OutputDir:       outputDir,
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
	cfg := Config{
		FrameConfigPath: cfgPath, WorkDir: workDir, OutputDir: outputDir, CaptureCount: 3,
		Countdown: 0, Interval: 0, CaptureWidth: 640, CaptureHeight: 480,
		SessionID: sessionID, ResumeSession: true, ResumeCaptureCount: 1,
	}
	writeWorkflowTestManifest(t, filepath.Dir(captureDir), cfg, session.PhaseCapturing, 1)
	result, err := runner.Run(context.Background(), cfg)
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

type partialWriteCaptureProvider struct {
	base *fake.Provider
}

func (p *partialWriteCaptureProvider) Info() camera.Info              { return p.base.Info() }
func (p *partialWriteCaptureProvider) Open(ctx context.Context) error { return p.base.Open(ctx) }
func (p *partialWriteCaptureProvider) Close() error                   { return p.base.Close() }
func (p *partialWriteCaptureProvider) Capture(_ context.Context, req camera.CaptureRequest) (camera.CaptureResult, error) {
	if err := os.WriteFile(req.OutputPath, []byte("partial JPEG bytes left by a crashed camera write"), 0o600); err != nil {
		return camera.CaptureResult{}, err
	}
	return camera.CaptureResult{}, errors.New("simulated camera interruption after a partial staging write")
}

func TestPartialCaptureRemainsStagedAndExplicitContinueRetakesIt(t *testing.T) {
	runner, cfg, workDir, outputDir := newTestRunner(t, &fake.Provider{}, testFrameConfig(t), nil)
	runner.Camera = &partialWriteCaptureProvider{base: fake.New()}
	result, err := runner.Run(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "partial staging write") {
		t.Fatalf("Run error=%v, want simulated partial capture failure", err)
	}
	if len(result.CapturePaths) != 0 {
		t.Fatalf("partial capture was committed: %+v", result.CapturePaths)
	}
	capturesDir := filepath.Join(workDir, result.SessionID, "captures")
	finalPath := filepath.Join(capturesDir, "capture-01.jpg")
	if _, err := os.Stat(finalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial image escaped staging into the committed capture namespace: %v", err)
	}
	scan, err := session.InspectRecoveryRoot(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Items) != 1 || scan.Items[0].Status != session.RecoveryResumeAvailable || scan.Items[0].CompletedShots != 0 {
		t.Fatalf("recovery scan=%+v, want zero-committed-shot resumable session", scan)
	}

	partialPath := filepath.Join(capturesDir, session.CaptureStagingDirectoryName, "shot-01-")
	entries, err := os.ReadDir(filepath.Join(capturesDir, session.CaptureStagingDirectoryName))
	if err != nil || len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), filepath.Base(partialPath)) {
		t.Fatalf("staged partial write was not retained for explicit recovery: entries=%+v err=%v", entries, err)
	}

	resume := &Runner{Camera: fake.New(), Renderer: render.New(), Output: output.NewLocal(outputDir),
		Sleep: func(ctx context.Context, _ time.Duration) error { return ctx.Err() }}
	cfg.ResumeSession = true
	cfg.ResumeCaptureCount = 0
	resumed, err := resume.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("explicit Continue after partial capture: %v", err)
	}
	if resumed.State != session.Completed || len(resumed.CapturePaths) != cfg.CaptureCount {
		t.Fatalf("resumed result=%+v, want completed session with %d committed captures", resumed, cfg.CaptureCount)
	}
	if _, err := os.Stat(filepath.Join(capturesDir, session.CaptureStagingDirectoryName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("explicit Continue did not clean validated staging artifacts: %v", err)
	}
}

func TestPublishedCaptureWithStaleManifestIsReconciledOnExplicitContinue(t *testing.T) {
	runner, cfg, workDir, outputDir := newTestRunner(t, fake.New(), testFrameConfig(t), nil)
	injected := errors.New("simulated interruption before manifest progress update")
	runner.writeManifest = func(sessionDir string, manifest session.Manifest) error {
		if manifest.Phase == session.PhaseCapturing && manifest.CompletedShots == 1 {
			return injected
		}
		return session.WriteManifest(sessionDir, manifest)
	}
	interrupted, err := runner.Run(context.Background(), cfg)
	if !errors.Is(err, injected) {
		t.Fatalf("Run error=%v, want injected manifest interruption", err)
	}
	firstCapture := filepath.Join(workDir, interrupted.SessionID, "captures", "capture-01.jpg")
	if err := session.ValidateJPEGFile(firstCapture); err != nil {
		t.Fatalf("atomically published capture is invalid: %v", err)
	}

	scan, err := session.InspectRecoveryRoot(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Items) != 1 || scan.Items[0].Status != session.RecoveryResumeAvailable ||
		scan.Items[0].Manifest == nil || scan.Items[0].Manifest.CompletedShots != 0 || scan.Items[0].CompletedShots != 1 {
		t.Fatalf("recovery scan=%+v, want one durable capture ahead of manifest", scan)
	}

	resume := &Runner{Camera: fake.New(), Renderer: render.New(), Output: output.NewLocal(outputDir),
		Sleep: func(ctx context.Context, _ time.Duration) error { return ctx.Err() }}
	cfg.ResumeSession = true
	cfg.ResumeCaptureCount = 1
	resumed, err := resume.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Continue should reconcile the atomically published capture: %v", err)
	}
	if resumed.State != session.Completed || len(resumed.CapturePaths) != cfg.CaptureCount {
		t.Fatalf("resumed result=%+v, want completed session", resumed)
	}
	manifest, err := session.ReadManifest(filepath.Join(workDir, resumed.SessionID))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.CompletedShots != cfg.CaptureCount || manifest.Phase != session.PhaseOutputFinalized {
		t.Fatalf("reconciled manifest=%+v, want finalized manifest with all captures", manifest)
	}
}

func TestCapturePublicationNeverOverwritesAnExistingFinalPath(t *testing.T) {
	runner, cfg, workDir, _ := newTestRunner(t, fake.New(), testFrameConfig(t), nil)
	sentinel := []byte("pre-existing artifact must never be replaced")
	runner.publishCapture = func(stagedPath, finalPath string) error {
		if err := os.WriteFile(finalPath, sentinel, 0o600); err != nil {
			return err
		}
		return publishCaptureNoReplace(stagedPath, finalPath)
	}
	result, err := runner.Run(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "without replacement") {
		t.Fatalf("Run error=%v, want no-replace publication collision", err)
	}
	finalPath := filepath.Join(workDir, result.SessionID, "captures", "capture-01.jpg")
	got, readErr := os.ReadFile(finalPath)
	if readErr != nil || string(got) != string(sentinel) {
		t.Fatalf("pre-existing final path was modified: got=%q err=%v", got, readErr)
	}
}

func TestResumeUsesImmutableFrameSnapshotWhenOriginalFrameIsMissing(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "sessions")
	outputDir := filepath.Join(root, "output")
	framePath := testFrameConfig(t)
	sessionID := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	sessionDir := filepath.Join(workDir, sessionID)
	captureDir := filepath.Join(sessionDir, "captures")
	if err := os.MkdirAll(captureDir, 0o700); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		FrameConfigPath: framePath, WorkDir: workDir, OutputDir: outputDir,
		CaptureCount: 3, CaptureWidth: 640, CaptureHeight: 480,
		SessionID: sessionID, ResumeSession: true, ResumeCaptureCount: 1,
	}
	frameBytes, err := readFrameConfigBytes(framePath)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := NewSessionManifestFromFrameBytes(cfg, frameBytes)
	if err != nil {
		t.Fatal(err)
	}
	snapshotPath, snapshotHash, err := session.WriteFrameSnapshot(sessionDir, frameBytes)
	if err != nil {
		t.Fatal(err)
	}
	manifest.FrameSnapshotFile = session.FrameSnapshotFileName
	manifest.FrameSnapshotSHA256 = snapshotHash

	fixtureCamera := fake.New()
	if err := fixtureCamera.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := fixtureCamera.Capture(context.Background(), camera.CaptureRequest{
		Sequence: 1, Width: 640, Height: 480,
		OutputPath: filepath.Join(captureDir, "capture-01.jpg"),
	}); err != nil {
		t.Fatal(err)
	}
	if err := fixtureCamera.Close(); err != nil {
		t.Fatal(err)
	}
	manifest.Phase = session.PhaseCapturing
	manifest.CompletedShots = 1
	manifest.UpdatedAt = time.Now().UTC()
	if err := session.WriteManifest(sessionDir, manifest); err != nil {
		t.Fatal(err)
	}

	// Simulate the operator changing or removing the active frame after the
	// interrupted session was created. Recovery must render from its snapshot.
	if err := os.Remove(framePath); err != nil {
		t.Fatal(err)
	}

	runner := &Runner{
		Camera: fake.New(), Renderer: render.New(), Output: output.NewLocal(outputDir),
		Sleep: func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
	}
	result, err := runner.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("resume with original frame missing: %v", err)
	}
	if result.State != session.Completed || len(result.CapturePaths) != 3 {
		t.Fatalf("result state=%s captures=%d, want completed with 3 captures", result.State, len(result.CapturePaths))
	}
	if _, err := os.Stat(snapshotPath); err != nil {
		t.Fatalf("durable frame snapshot was removed during resume: %v", err)
	}
	if err := session.ValidateJPEGFile(result.RenderedPath); err != nil {
		t.Fatalf("rendered output was not produced from the snapshot: %v", err)
	}
	if err := session.ValidateJPEGFile(result.OutputPath); err != nil {
		t.Fatalf("local output missing/invalid after resume: %v", err)
	}
}

type recordingPaymentGate struct {
	requests []payment.Request
}

func (g *recordingPaymentGate) EnsureAuthorized(ctx context.Context, request payment.Request) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	g.requests = append(g.requests, request)
	return nil
}

type rejectingPaymentGate struct {
	err error
}

func (g rejectingPaymentGate) EnsureAuthorized(context.Context, payment.Request) error {
	return g.err
}

func TestPaymentGateDenialHappensBeforeWorkspaceCreationOrCameraOpen(t *testing.T) {
	framePath := testFrameConfig(t)
	cameraProvider := fake.New()
	runner, cfg, workDir, _ := newTestRunner(t, cameraProvider, framePath, nil)
	runner.PaymentGate = rejectingPaymentGate{err: errors.New("payment not verified")}

	result, err := runner.Run(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "payment authorization failed before capture") {
		t.Fatalf("Run error=%v, want payment denial", err)
	}
	if result.State != session.Failed {
		t.Fatalf("result state=%s, want failed", result.State)
	}
	sessionDir := filepath.Join(workDir, cfg.SessionID)
	if _, statErr := os.Stat(sessionDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("payment denial must happen before claiming a session workspace: %v", statErr)
	}
	if _, captureErr := cameraProvider.Capture(context.Background(), camera.CaptureRequest{
		Sequence: 1, Width: 640, Height: 480, OutputPath: filepath.Join(t.TempDir(), "probe.jpg"),
	}); captureErr == nil || !strings.Contains(strings.ToLower(captureErr.Error()), "open") {
		t.Fatalf("camera unexpectedly became usable before payment authorization: %v", captureErr)
	}
}

func TestSessionManifestRecordsCurrentDeliveryAndPaymentPolicies(t *testing.T) {
	cfg := Config{
		SessionID:       "0123456789abcdef0123456789abcdef",
		FrameConfigPath: testFrameConfig(t),
		CaptureCount:    3,
		CaptureWidth:    640,
		CaptureHeight:   480,
	}
	manifest, err := NewSessionManifest(cfg)
	if err != nil {
		t.Fatalf("NewSessionManifest: %v", err)
	}
	if manifest.MediaDeliveryMode != stationconfig.MediaDeliveryModeLocal ||
		manifest.PaymentMode != stationconfig.PaymentModeDisabled {
		t.Fatalf("manifest policies=%q/%q, want local/disabled", manifest.MediaDeliveryMode, manifest.PaymentMode)
	}

	cfg.MediaDeliveryMode = "lan"
	if _, err := NewSessionManifest(cfg); err == nil || !strings.Contains(err.Error(), "unsupported media delivery mode") {
		t.Fatalf("unsupported delivery mode error=%v", err)
	}
	cfg.MediaDeliveryMode = stationconfig.MediaDeliveryModeLocal
	cfg.PaymentMode = "qr"
	if _, err := NewSessionManifest(cfg); err == nil || !strings.Contains(err.Error(), "unsupported payment mode") {
		t.Fatalf("unsupported payment mode error=%v", err)
	}
}

func TestResumeRejectsUnsupportedPolicyChangeBeforeOpeningCamera(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*Config)
		want string
	}{
		{
			name: "delivery policy",
			edit: func(cfg *Config) { cfg.MediaDeliveryMode = "lan" },
			want: "unsupported media delivery mode",
		},
		{
			name: "payment policy",
			edit: func(cfg *Config) { cfg.PaymentMode = "qr" },
			want: "unsupported payment mode",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cameraProvider := fake.New()
			runner, cfg, workDir, _ := newTestRunner(t, cameraProvider, testFrameConfig(t), nil)
			cfg.MediaDeliveryMode = stationconfig.MediaDeliveryModeLocal
			cfg.PaymentMode = stationconfig.PaymentModeDisabled
			sessionDir := filepath.Join(workDir, cfg.SessionID)
			captureDir := filepath.Join(sessionDir, "captures")
			if err := os.MkdirAll(captureDir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := cameraProvider.Open(context.Background()); err != nil {
				t.Fatal(err)
			}
			capturePath := filepath.Join(captureDir, "capture-01.jpg")
			if _, err := cameraProvider.Capture(context.Background(), camera.CaptureRequest{
				Sequence: 1, Width: cfg.CaptureWidth, Height: cfg.CaptureHeight, OutputPath: capturePath,
			}); err != nil {
				t.Fatal(err)
			}
			if err := cameraProvider.Close(); err != nil {
				t.Fatal(err)
			}
			writeWorkflowTestManifest(t, sessionDir, cfg, session.PhaseCapturing, 1)
			manifestPath := filepath.Join(sessionDir, session.ManifestFileName)
			before, err := os.ReadFile(manifestPath)
			if err != nil {
				t.Fatal(err)
			}

			cfg.ResumeSession = true
			cfg.ResumeCaptureCount = 1
			tc.edit(&cfg)
			result, runErr := runner.Run(context.Background(), cfg)
			if runErr == nil || !strings.Contains(runErr.Error(), tc.want) {
				t.Fatalf("Run error=%v, want %q", runErr, tc.want)
			}
			if result.State != session.Failed {
				t.Fatalf("resume state=%s, want failed", result.State)
			}
			after, err := os.ReadFile(manifestPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("policy mismatch changed the interrupted manifest")
			}
			if _, err := cameraProvider.Capture(context.Background(), camera.CaptureRequest{
				Sequence: 2, Width: cfg.CaptureWidth, Height: cfg.CaptureHeight, OutputPath: filepath.Join(t.TempDir(), "probe.jpg"),
			}); err == nil {
				t.Fatal("policy mismatch opened or retained the camera")
			}
		})
	}
}
