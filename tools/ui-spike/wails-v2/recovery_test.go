package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dongsinhho/PhotoBooth/internal/booth/session"
	"github.com/dongsinhho/PhotoBooth/internal/booth/workflow"
)

func truncateRecoveryJPEGScanPayload(t *testing.T, data []byte) []byte {
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

func newRecoveryTestApp(t *testing.T) *App {
	t.Helper()
	app := NewApp()
	app.playerDataDir = t.TempDir()
	if _, err := app.GetPlayerSettings(); err != nil {
		t.Fatalf("initialize station profile: %v", err)
	}
	app.mu.Lock()
	app.playerCountdown = 0
	app.playerInterval = 0
	app.mu.Unlock()
	return app
}

func writeRecoveryTestJPEG(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: uint8(30 + x*10), G: uint8(40 + y*10), B: 130, A: 255})
		}
	}
	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: 85}); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func seedInterruptedWailsSession(t *testing.T, app *App, id string, phase session.Phase, completed int, demo bool) (string, session.Manifest) {
	t.Helper()
	app.mu.Lock()
	runConfig := app.playerRuntimeConfigLocked()
	app.mu.Unlock()

	sessionDir := filepath.Join(runConfig.workDir, id)
	captureDir := filepath.Join(sessionDir, "captures")
	if err := os.MkdirAll(captureDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= completed; i++ {
		writeRecoveryTestJPEG(t, filepath.Join(captureDir, fmt.Sprintf("capture-%02d.jpg", i)))
	}
	cfg := playerWorkflowConfig(runConfig, id, completed, demo, true)
	frameBytes, err := os.ReadFile(cfg.FrameConfigPath)
	if err != nil {
		t.Fatalf("read session frame fixture: %v", err)
	}
	manifest, err := workflow.NewSessionManifestFromFrameBytes(cfg, frameBytes)
	if err != nil {
		t.Fatalf("create session manifest fixture: %v", err)
	}
	_, snapshotHash, err := session.WriteFrameSnapshot(sessionDir, frameBytes)
	if err != nil {
		t.Fatalf("write session frame snapshot fixture: %v", err)
	}
	manifest.FrameSnapshotFile = session.FrameSnapshotFileName
	manifest.FrameSnapshotSHA256 = snapshotHash
	manifest.Phase = phase
	manifest.CompletedShots = completed
	manifest.UpdatedAt = time.Now().UTC()
	if phase == session.PhaseRendered || phase == session.PhaseOutputFinalized {
		writeRecoveryTestJPEG(t, filepath.Join(sessionDir, "rendered.jpg"))
		manifest.RenderedAt = manifest.UpdatedAt
	}
	if phase == session.PhaseOutputFinalized {
		outputPath := filepath.Join(runConfig.outputDir, id, "photo.jpg")
		writeRecoveryTestJPEG(t, outputPath)
		manifest.OutputPath = outputPath
		manifest.OutputFinalizedAt = manifest.UpdatedAt
	}
	if err := session.WriteManifest(sessionDir, manifest); err != nil {
		t.Fatalf("persist session manifest fixture: %v", err)
	}
	return sessionDir, manifest
}

func waitForPlayerTerminalState(t *testing.T, app *App) PlayerSession {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		current := app.CurrentPhotoSession()
		switch current.State {
		case string(session.Completed), string(session.PartialSuccess), string(session.Failed), string(session.Cancelled), string(session.TimedOut):
			return current
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for recovered player session; last state=%+v", app.CurrentPhotoSession())
	return PlayerSession{}
}

func TestSyntheticOnlyModeBlocksAllRealCameraCaptureEntryPoints(t *testing.T) {
	t.Setenv(syntheticOnlyEnv, "1")
	app := newRecoveryTestApp(t)

	if _, err := app.StartPhotoSession(); err == nil || !strings.Contains(err.Error(), "synthetic-only") {
		t.Fatalf("StartPhotoSession error=%v, want synthetic-only refusal", err)
	}
	if app.playerRunning || app.player.ID != "" {
		t.Fatalf("rejected real-camera start changed the player state: %+v", app.player)
	}
	if entries, err := os.ReadDir(app.workDir); err != nil || len(entries) != 0 {
		t.Fatalf("rejected real-camera start created session workspace entries=%d err=%v", len(entries), err)
	}

	app.mu.Lock()
	app.player = PlayerSession{ID: "91111111111111111111111111111111", State: string(session.Failed), Captured: 1, Total: 3, DemoMode: false}
	app.mu.Unlock()
	if _, err := app.ContinuePhotoSession(app.player.ID); err == nil || !strings.Contains(err.Error(), "synthetic-only") {
		t.Fatalf("ContinuePhotoSession error=%v, want synthetic-only refusal", err)
	}
	if app.playerRunning {
		t.Fatal("rejected continuation started a real-camera session")
	}
	if _, err := app.RestartPhotoSession(app.player.ID); err == nil || !strings.Contains(err.Error(), "synthetic-only") {
		t.Fatalf("RestartPhotoSession error=%v, want synthetic-only refusal", err)
	}
	if app.playerRunning {
		t.Fatal("rejected restart started a real-camera session")
	}

	const recoveredID = "92111111111111111111111111111111"
	seedInterruptedWailsSession(t, app, recoveredID, session.PhaseCapturing, 1, false)
	if _, err := app.ContinueRecoveredPhotoSession(recoveredID); err == nil || !strings.Contains(err.Error(), "synthetic-only") {
		t.Fatalf("ContinueRecoveredPhotoSession error=%v, want synthetic-only refusal", err)
	}
	if _, err := app.RestartRecoveredPhotoSession(recoveredID); err == nil || !strings.Contains(err.Error(), "synthetic-only") {
		t.Fatalf("RestartRecoveredPhotoSession error=%v, want synthetic-only refusal", err)
	}
	if app.playerRunning {
		t.Fatal("rejected recovered real-camera action started a session")
	}
}

func TestListInterruptedPhotoSessionsIsReadOnlyAndLegacyIsInspectOnly(t *testing.T) {
	app := newRecoveryTestApp(t)
	interruptedID := "0123456789abcdef0123456789abcdef"
	sessionDir, _ := seedInterruptedWailsSession(t, app, interruptedID, session.PhaseCapturing, 1, true)
	legacyID := "1123456789abcdef0123456789abcdef"
	legacyDir := filepath.Join(app.workDir, legacyID)
	if err := os.MkdirAll(filepath.Join(legacyDir, "captures"), 0o700); err != nil {
		t.Fatal(err)
	}

	manifestPath := filepath.Join(sessionDir, session.ManifestFileName)
	beforeManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	capturePath := filepath.Join(sessionDir, "captures", "capture-01.jpg")
	beforeCapture, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatal(err)
	}

	items, err := app.ListInterruptedPhotoSessions()
	if err != nil {
		t.Fatalf("ListInterruptedPhotoSessions: %v", err)
	}
	statuses := make(map[string]string)
	for _, item := range items {
		statuses[item.SessionID] = item.Status
	}
	if statuses[interruptedID] != string(session.RecoveryResumeAvailable) {
		t.Fatalf("interrupted session status=%q, want resume_available; items=%+v", statuses[interruptedID], items)
	}
	if statuses[legacyID] != string(session.RecoveryInspectOnly) {
		t.Fatalf("legacy session status=%q, want inspect_only; items=%+v", statuses[legacyID], items)
	}
	viewFound := false
	for _, item := range items {
		if item.SessionID != interruptedID {
			continue
		}
		viewFound = true
		if item.Manifest == nil || item.Manifest.ExpectedShots != 3 || !item.Manifest.DemoMode {
			t.Fatalf("recovery manifest view lost required UI metadata: %+v", item.Manifest)
		}
		if _, parseErr := time.Parse(time.RFC3339Nano, item.Manifest.UpdatedAt); parseErr != nil {
			t.Fatalf("recovery manifest updated_at is not RFC3339: %q err=%v", item.Manifest.UpdatedAt, parseErr)
		}
	}
	if !viewFound {
		t.Fatalf("recovery view did not contain the interrupted session: %+v", items)
	}
	afterManifest, err := os.ReadFile(manifestPath)
	if err != nil || !bytes.Equal(beforeManifest, afterManifest) {
		t.Fatalf("discovery changed manifest: err=%v", err)
	}
	afterCapture, err := os.ReadFile(capturePath)
	if err != nil || !bytes.Equal(beforeCapture, afterCapture) {
		t.Fatalf("discovery changed capture: err=%v", err)
	}
	if app.playerRunning {
		t.Fatal("discovery must not start a player session")
	}
}

func TestContinueRecoveredDemoSessionAfterProcessRestart(t *testing.T) {
	app := newRecoveryTestApp(t)
	const id = "2123456789abcdef0123456789abcdef"
	sessionDir, _ := seedInterruptedWailsSession(t, app, id, session.PhaseCapturing, 1, true)
	firstCapture := filepath.Join(sessionDir, "captures", "capture-01.jpg")
	before, err := os.ReadFile(firstCapture)
	if err != nil {
		t.Fatal(err)
	}

	started, err := app.ContinueRecoveredPhotoSession(id)
	if err != nil {
		t.Fatalf("ContinueRecoveredPhotoSession: %v", err)
	}
	if started.ID != id || started.Captured != 1 || !started.DemoMode {
		t.Fatalf("unexpected recovery start snapshot: %+v", started)
	}
	final := waitForPlayerTerminalState(t, app)
	if final.ID != id || final.State != string(session.Completed) || final.Captured != 3 {
		t.Fatalf("unexpected recovered completion: %+v", final)
	}
	after, err := os.ReadFile(firstCapture)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("original capture was not preserved: err=%v", err)
	}
	manifest, err := session.ReadManifest(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Phase != session.PhaseOutputFinalized || manifest.OutputPath != final.OutputPath {
		t.Fatalf("finalized manifest=%+v final=%+v", manifest, final)
	}
	items, err := app.ListInterruptedPhotoSessions()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.SessionID == id {
			t.Fatalf("finalized session should not remain in recovery prompts: %+v", item)
		}
	}
}

func TestRecoveredRenderedSessionRetriesOnlyLocalOutput(t *testing.T) {
	app := newRecoveryTestApp(t)
	const id = "3123456789abcdef0123456789abcdef"
	sessionDir, _ := seedInterruptedWailsSession(t, app, id, session.PhaseRendered, 3, true)
	renderedPath := filepath.Join(sessionDir, "rendered.jpg")
	before, err := os.ReadFile(renderedPath)
	if err != nil {
		t.Fatal(err)
	}

	items, err := app.ListInterruptedPhotoSessions()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range items {
		if item.SessionID == id && item.Status == string(session.RecoveryOutputPending) {
			found = true
		}
	}
	if !found {
		t.Fatalf("rendered session was not offered for local-output retry: %+v", items)
	}
	final, err := app.RetryRecoveredPhotoOutput(id)
	if err != nil {
		t.Fatalf("RetryRecoveredPhotoOutput: %v", err)
	}
	if final.ID != id || final.State != string(session.Completed) || final.OutputPath == "" {
		t.Fatalf("unexpected recovered output result: %+v", final)
	}
	after, err := os.ReadFile(renderedPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("rendered source was changed during retry: err=%v", err)
	}
	if _, err := os.Stat(final.OutputPath); err != nil {
		t.Fatalf("retried local output is missing: %v", err)
	}
	if app.playerRunning {
		t.Fatal("local output retry must not start a capture session")
	}
}

func TestRecoveredOutputRetryFailsClosedWhenIdlePreviewCleanupIsUncertain(t *testing.T) {
	app := newRecoveryTestApp(t)
	const id = "41111111111111111111111111111111"
	sessionDir, _ := seedInterruptedWailsSession(t, app, id, session.PhaseRendered, 3, true)
	app.previewCloseErr = errors.New("preview worker did not release camera")

	if _, err := app.RetryRecoveredPhotoOutput(id); err == nil || !strings.Contains(err.Error(), "preview worker did not release camera") {
		t.Fatalf("RetryRecoveredPhotoOutput error=%v, want preview cleanup refusal", err)
	}
	outputPath := filepath.Join(app.outputDir, id, "photo.jpg")
	if _, err := os.Stat(outputPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("recovered output was published despite uncertain preview cleanup: err=%v", err)
	}
	if app.playerRunning || app.outputRetrying {
		t.Fatal("recovered output retry flags changed after preview cleanup refusal")
	}
	if _, err := session.ReadManifest(sessionDir); err != nil {
		t.Fatalf("original rendered session manifest should remain intact: %v", err)
	}
}

func TestRecoveredOutputRetryRejectsTruncatedRenderedJPEG(t *testing.T) {
	app := newRecoveryTestApp(t)
	const id = "51111111111111111111111111111111"
	sessionDir, _ := seedInterruptedWailsSession(t, app, id, session.PhaseRendered, 3, true)
	renderedPath := filepath.Join(sessionDir, "rendered.jpg")
	data, err := os.ReadFile(renderedPath)
	if err != nil {
		t.Fatal(err)
	}
	truncatedJPEG := truncateRecoveryJPEGScanPayload(t, data)
	if err := os.WriteFile(renderedPath, truncatedJPEG, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := app.RetryRecoveredPhotoOutput(id); err == nil || !strings.Contains(err.Error(), "complete JPEG payload") {
		t.Fatalf("RetryRecoveredPhotoOutput error=%v, want truncated JPEG refusal", err)
	}
	outputPath := filepath.Join(app.outputDir, id, "photo.jpg")
	if _, err := os.Stat(outputPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("truncated rendered JPEG was published: err=%v", err)
	}
}

func TestRestartRecoveredSessionUsesNewIDAndPreservesOriginalFolder(t *testing.T) {
	app := newRecoveryTestApp(t)
	const originalID = "8123456789abcdef0123456789abcdef"
	originalDir, _ := seedInterruptedWailsSession(t, app, originalID, session.PhaseCapturing, 1, true)
	manifestPath := filepath.Join(originalDir, session.ManifestFileName)
	capturePath := filepath.Join(originalDir, "captures", "capture-01.jpg")
	beforeManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeCapture, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatal(err)
	}

	started, err := app.RestartRecoveredPhotoSession(originalID)
	if err != nil {
		t.Fatalf("RestartRecoveredPhotoSession: %v", err)
	}
	if started.ID == originalID || started.ID == "" {
		t.Fatalf("fresh session ID=%q, want a new ID", started.ID)
	}
	final := waitForPlayerTerminalState(t, app)
	if final.ID != started.ID || final.State != string(session.Completed) {
		t.Fatalf("fresh session did not complete: %+v", final)
	}
	afterManifest, err := os.ReadFile(manifestPath)
	if err != nil || !bytes.Equal(beforeManifest, afterManifest) {
		t.Fatalf("restart changed original manifest: err=%v", err)
	}
	afterCapture, err := os.ReadFile(capturePath)
	if err != nil || !bytes.Equal(beforeCapture, afterCapture) {
		t.Fatalf("restart changed original capture: err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(app.workDir, started.ID)); err != nil {
		t.Fatalf("fresh session workspace is missing: %v", err)
	}
}

func TestRecoveryLeavesAmbiguousOutputDirectoriesForInspection(t *testing.T) {
	app := newRecoveryTestApp(t)
	const tempID = "6123456789abcdef0123456789abcdef"
	const publishedID = "7123456789abcdef0123456789abcdef"
	seedInterruptedWailsSession(t, app, tempID, session.PhaseRendered, 3, true)
	seedInterruptedWailsSession(t, app, publishedID, session.PhaseRendered, 3, true)

	app.mu.Lock()
	outputRoot := app.outputDir
	app.mu.Unlock()

	tempDir := filepath.Join(outputRoot, tempID)
	if err := os.MkdirAll(tempDir, 0o700); err != nil {
		t.Fatal(err)
	}
	tempPath := filepath.Join(tempDir, ".photo.jpg.partial")
	if err := os.WriteFile(tempPath, []byte("unfinished output"), 0o600); err != nil {
		t.Fatal(err)
	}

	publishedDir := filepath.Join(outputRoot, publishedID)
	publishedPath := filepath.Join(publishedDir, "photo.jpg")
	writeRecoveryTestJPEG(t, publishedPath)
	beforePublished, err := os.ReadFile(publishedPath)
	if err != nil {
		t.Fatal(err)
	}

	items, err := app.ListInterruptedPhotoSessions()
	if err != nil {
		t.Fatalf("ListInterruptedPhotoSessions: %v", err)
	}
	statuses := make(map[string]string)
	for _, item := range items {
		statuses[item.SessionID] = item.Status
	}
	if statuses[tempID] != string(session.RecoveryInspectOnly) {
		t.Fatalf("temporary-output session status=%q, want inspect_only; items=%+v", statuses[tempID], items)
	}
	if statuses[publishedID] != string(session.RecoveryInspectOnly) {
		t.Fatalf("already-published session status=%q, want inspect_only; items=%+v", statuses[publishedID], items)
	}
	afterTemp, err := os.ReadFile(tempPath)
	if err != nil || string(afterTemp) != "unfinished output" {
		t.Fatalf("discovery changed temp output: err=%v bytes=%q", err, afterTemp)
	}
	afterPublished, err := os.ReadFile(publishedPath)
	if err != nil || !bytes.Equal(beforePublished, afterPublished) {
		t.Fatalf("discovery changed existing final output: err=%v", err)
	}
}

func TestRecoveryRejectsCurrentConfigurationDrift(t *testing.T) {
	app := newRecoveryTestApp(t)
	const id = "4123456789abcdef0123456789abcdef"
	_, _ = seedInterruptedWailsSession(t, app, id, session.PhaseCapturing, 1, true)
	app.mu.Lock()
	app.playerCountdown = 500 * time.Millisecond
	app.mu.Unlock()

	items, err := app.ListInterruptedPhotoSessions()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.SessionID == id {
			if item.Status != string(session.RecoveryInspectOnly) {
				t.Fatalf("configuration drift status=%s, want inspect_only", item.Status)
			}
			return
		}
	}
	t.Fatalf("configuration-drifted session was not surfaced: %+v", items)
}

func TestContinueRecoveredSessionRejectsLegacyWithoutManifest(t *testing.T) {
	app := newRecoveryTestApp(t)
	const id = "5123456789abcdef0123456789abcdef"
	if err := os.MkdirAll(filepath.Join(app.workDir, id, "captures"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ContinueRecoveredPhotoSession(id); err == nil {
		t.Fatal("legacy session without manifest must not be resumable")
	}
	if app.playerRunning {
		t.Fatal("rejected recovery must not start the camera/session")
	}
}

func TestRecoveredSessionUsesSnapshotWhenSelectedFrameIsUnavailable(t *testing.T) {
	app := newRecoveryTestApp(t)
	const sessionID = "a123456789abcdef0123456789abcdef"
	sessionDir, manifest := seedInterruptedWailsSession(t, app, sessionID, session.PhaseCapturing, 1, true)

	originalFrame := app.framePath
	if err := os.Remove(originalFrame); err != nil {
		t.Fatalf("remove current frame to simulate missing operator-selected frame: %v", err)
	}
	app.mu.Lock()
	app.settingsError = "The saved frame JSON cannot be loaded: simulated missing current frame"
	app.mu.Unlock()

	items, err := app.ListInterruptedPhotoSessions()
	if err != nil {
		t.Fatalf("ListInterruptedPhotoSessions: %v", err)
	}
	found := false
	for _, item := range items {
		if item.SessionID == sessionID {
			found = true
			if item.Status != string(session.RecoveryResumeAvailable) {
				t.Fatalf("snapshot-backed session status=%q reason=%q, want resume_available", item.Status, item.Reason)
			}
		}
	}
	if !found {
		t.Fatalf("snapshot-backed interrupted session was not discovered: %+v", items)
	}

	started, err := app.ContinueRecoveredPhotoSession(sessionID)
	if err != nil {
		t.Fatalf("ContinueRecoveredPhotoSession with missing original frame: %v", err)
	}
	if started.ID != sessionID || started.Captured != 1 {
		t.Fatalf("recovery started wrong session state: %+v", started)
	}
	terminal := waitForPlayerTerminalState(t, app)
	if terminal.State != string(session.Completed) || terminal.Captured != terminal.Total {
		t.Fatalf("snapshot-backed recovery did not complete: %+v", terminal)
	}
	if _, _, err := session.ReadFrameSnapshot(sessionDir, manifest); err != nil {
		t.Fatalf("session frame snapshot was not preserved: %v", err)
	}
}
