package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dongsinhho/PhotoBooth/internal/booth/output"
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

func TestRecoveryItemViewConvertsPrintAttemptTimestampsToStrings(t *testing.T) {
	updated := time.Now().UTC().Truncate(time.Nanosecond)
	sessionID := "9123456789abcdef0123456789abcdef"
	attemptID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	attempt := session.PrintAttempt{
		AttemptID:    attemptID,
		QueueName:    "DS-RX1",
		DocumentName: session.PrintDocumentName(sessionID, attemptID),
		SourceSHA256: strings.Repeat("a", 64),
		Status:       session.PrintStatusQueued,
		JobID:        "29",
		RequestedAt:  updated,
		UpdatedAt:    updated,
	}
	view := recoveryItemView(session.RecoveryItem{
		SessionID:     sessionID,
		Status:        session.RecoveryFinalized,
		PrintAttempts: []session.PrintAttempt{attempt},
	})
	if len(view.PrintAttempts) != 1 {
		t.Fatalf("print attempt views=%d, want 1", len(view.PrintAttempts))
	}
	got := view.PrintAttempts[0]
	if got.AttemptID != attemptID || got.DocumentName != attempt.DocumentName || got.JobID != "29" || got.Status != session.PrintStatusQueued {
		t.Fatalf("print attempt view=%+v", got)
	}
	if _, err := time.Parse(time.RFC3339Nano, got.RequestedAt); err != nil {
		t.Fatalf("requested_at=%q is not RFC3339: %v", got.RequestedAt, err)
	}
	if _, err := time.Parse(time.RFC3339Nano, got.UpdatedAt); err != nil {
		t.Fatalf("updated_at=%q is not RFC3339: %v", got.UpdatedAt, err)
	}
}

func TestPrinterInspectionViewDoesNotInferPaperOutput(t *testing.T) {
	notFound := printerInspectionView(output.PrinterJobInspection{
		QueueName: "DS-RX1",
		Found:     false,
	})
	if notFound.Found || !strings.Contains(notFound.Note, "does not prove") {
		t.Fatalf("not-found inspection overclaims an outcome: %+v", notFound)
	}
	found := printerInspectionView(output.PrinterJobInspection{
		QueueName: "DS-RX1",
		Found:     true,
		Jobs: []output.PrinterQueueJob{{
			JobID: "29", DocumentName: "PhotoBooth-session-attempt",
			Status: []string{"printing"}, PagesPrinted: 1, TotalPages: 1,
		}},
	})
	if !found.Found || len(found.Jobs) != 1 || found.Jobs[0].JobID != "29" {
		t.Fatalf("found inspection was not mapped: %+v", found)
	}
	if !strings.Contains(found.Note, "not proof") {
		t.Fatalf("found inspection overclaims physical output: %+v", found)
	}
}

func TestRecoveryItemViewIncludesPreManifestPrintLockIdentity(t *testing.T) {
	createdAt := time.Now().UTC().Truncate(time.Nanosecond)
	lock := &session.PrintLockInfo{
		LeaseVersion: session.PrintLockLeaseVersion,
		OwnerPID:     1234,
		AttemptID:    "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		QueueName:    "DS-RX1",
		DocumentName: session.PrintDocumentName("9123456789abcdef0123456789abcdef", "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"),
		SourceSHA256: strings.Repeat("a", 64),
		CreatedAt:    createdAt,
	}
	view := recoveryItemView(session.RecoveryItem{
		SessionID:        "9123456789abcdef0123456789abcdef",
		Status:           session.RecoveryInspectOnly,
		PrintLockPresent: true,
		PrintLock:        lock,
	})
	if view.PrintLock == nil || view.PrintLock.AttemptID != lock.AttemptID ||
		view.PrintLock.QueueName != lock.QueueName || view.PrintLock.DocumentName != lock.DocumentName ||
		view.PrintLock.LeaseVersion != lock.LeaseVersion || view.PrintLock.OwnerPID != lock.OwnerPID ||
		view.PrintLock.SourceSHA256 != lock.SourceSHA256 {
		t.Fatalf("print lock view=%+v", view.PrintLock)
	}
	if _, err := time.Parse(time.RFC3339Nano, view.PrintLock.CreatedAt); err != nil {
		t.Fatalf("created_at=%q is not RFC3339: %v", view.PrintLock.CreatedAt, err)
	}
}

func seedQueuedPrinterAttemptForResolutionTest(t *testing.T, app *App, sessionID, attemptID string) string {
	t.Helper()
	sessionDir, _ := seedInterruptedWailsSession(t, app, sessionID, session.PhaseOutputFinalized, 3, true)
	manifest, err := session.ReadManifest(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	// Use a past rendered/request timestamp so the real queue inspection and
	// operator resolution calls occur after the persisted attempt update.
	base := time.Now().UTC().Add(-10 * time.Second)
	manifest.CreatedAt = base.Add(-time.Minute)
	manifest.RenderedAt = base
	manifest.OutputFinalizedAt = base.Add(time.Millisecond)
	manifest.UpdatedAt = manifest.OutputFinalizedAt
	requested := manifest.RenderedAt.Add(time.Second)
	attempt := session.PrintAttempt{
		AttemptID: attemptID, QueueName: "DS-RX1",
		DocumentName: session.PrintDocumentName(sessionID, attemptID),
		SourceSHA256: strings.Repeat("b", 64),
		Status:       session.PrintStatusSubmitting,
		RequestedAt:  requested, UpdatedAt: requested,
	}
	if err := session.BeginPrintAttempt(&manifest, attempt); err != nil {
		t.Fatalf("BeginPrintAttempt fixture: %v", err)
	}
	if err := session.FinishPrintAttempt(&manifest, attemptID, session.PrintStatusQueued, "501", "", requested.Add(time.Second)); err != nil {
		t.Fatalf("FinishPrintAttempt fixture: %v", err)
	}
	if err := session.WriteManifest(sessionDir, manifest); err != nil {
		t.Fatalf("persist queued print fixture: %v", err)
	}
	return sessionDir
}

func TestResolvePrinterAttemptRequiresQueueAbsenceAndWritesAudit(t *testing.T) {
	app := newRecoveryTestApp(t)
	const sessionID = "a123456789abcdef0123456789abcdef"
	const attemptID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	sessionDir := seedQueuedPrinterAttemptForResolutionTest(t, app, sessionID, attemptID)
	calls := 0
	app.inspectPrinterJob = func(queueName, documentName, jobID string) (output.PrinterJobInspection, error) {
		calls++
		if queueName != "DS-RX1" || documentName != session.PrintDocumentName(sessionID, attemptID) || jobID != "501" {
			t.Fatalf("queue inspection did not use persisted identity: queue=%q document=%q job=%q", queueName, documentName, jobID)
		}
		return output.PrinterJobInspection{QueueName: queueName, Found: false}, nil
	}

	resolved, err := app.ResolvePrinterAttempt(sessionID, attemptID, session.PrintResolutionNotPrinted,
		"Checked printer output tray: no paper emerged; queue no longer lists the job")
	if err != nil {
		t.Fatalf("ResolvePrinterAttempt: %v", err)
	}
	if resolved.Decision != session.PrintResolutionNotPrinted || resolved.QueueFound {
		t.Fatalf("resolution view=%+v", resolved)
	}
	if calls != 1 {
		t.Fatalf("queue inspections=%d, want 1 final server-side recheck", calls)
	}
	manifest, err := session.ReadManifest(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.PrintAttempts) != 1 || manifest.PrintAttempts[0].Status != session.PrintStatusQueued ||
		manifest.PrintAttempts[0].JobID != "501" {
		t.Fatalf("operator resolution must preserve the original queued attempt: %+v", manifest.PrintAttempts)
	}
	if len(manifest.PrintResolutions) != 1 || manifest.PrintResolutions[0].Decision != session.PrintResolutionNotPrinted ||
		manifest.PrintResolutions[0].Note != resolved.Note {
		t.Fatalf("operator decision was not durably saved: %+v", manifest.PrintResolutions)
	}
	if _, err := os.Stat(filepath.Join(sessionDir, ".print.lock")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("resolution created or retained a print lock: %v", err)
	}
}

func TestResolvePrinterAttemptRefusesWhileQueueJobIsStillListed(t *testing.T) {
	app := newRecoveryTestApp(t)
	const sessionID = "b123456789abcdef0123456789abcdef"
	const attemptID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	sessionDir := seedQueuedPrinterAttemptForResolutionTest(t, app, sessionID, attemptID)
	app.inspectPrinterJob = func(queueName, documentName, jobID string) (output.PrinterJobInspection, error) {
		return output.PrinterJobInspection{QueueName: queueName, Found: true, Jobs: []output.PrinterQueueJob{{
			JobID: "501", DocumentName: documentName, Status: []string{"printing"},
		}}}, nil
	}

	if _, err := app.ResolvePrinterAttempt(sessionID, attemptID, session.PrintResolutionNotPrinted,
		"no paper seen"); err == nil || !strings.Contains(err.Error(), "still present") {
		t.Fatalf("ResolvePrinterAttempt error=%v, want queue-still-present refusal", err)
	}
	manifest, err := session.ReadManifest(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.PrintResolutions) != 0 {
		t.Fatalf("failed resolution changed the manifest: %+v", manifest.PrintResolutions)
	}
}

func TestResolvePrinterAttemptKeepsStalePrintLockInspectOnly(t *testing.T) {
	app := newRecoveryTestApp(t)
	const sessionID = "c123456789abcdef0123456789abcdef"
	const attemptID = "cccccccccccccccccccccccccccccccc"
	sessionDir := seedQueuedPrinterAttemptForResolutionTest(t, app, sessionID, attemptID)
	lockInfo := session.PrintLockInfo{
		AttemptID: attemptID, QueueName: "DS-RX1",
		DocumentName: session.PrintDocumentName(sessionID, attemptID), CreatedAt: time.Now().UTC(),
	}
	lockData, err := json.Marshal(lockInfo)
	if err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(sessionDir, ".print.lock")
	if err := os.WriteFile(lockPath, lockData, 0o600); err != nil {
		t.Fatal(err)
	}
	queueCalls := 0
	app.inspectPrinterJob = func(string, string, string) (output.PrinterJobInspection, error) {
		queueCalls++
		return output.PrinterJobInspection{Found: false}, nil
	}

	if _, err := app.ResolvePrinterAttempt(sessionID, attemptID, session.PrintResolutionUnknown,
		"unable to establish paper outcome"); err == nil || !strings.Contains(err.Error(), "lock remains") {
		t.Fatalf("ResolvePrinterAttempt error=%v, want stale-lock inspect-only refusal", err)
	}
	if queueCalls != 0 {
		t.Fatalf("queue was inspected %d times despite an unresolved lock; want 0", queueCalls)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("stale lock was modified or removed: %v", err)
	}
	manifest, err := session.ReadManifest(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.PrintResolutions) != 0 {
		t.Fatalf("stale lock resolution changed manifest: %+v", manifest.PrintResolutions)
	}
}

func TestResolvePrinterAttemptCanCloseAsUnknownEvenWhenQueueJobRemains(t *testing.T) {
	app := newRecoveryTestApp(t)
	const sessionID = "d123456789abcdef0123456789abcdef"
	const attemptID = "dddddddddddddddddddddddddddddddd"
	sessionDir := seedQueuedPrinterAttemptForResolutionTest(t, app, sessionID, attemptID)
	app.inspectPrinterJob = func(queueName, documentName, jobID string) (output.PrinterJobInspection, error) {
		return output.PrinterJobInspection{QueueName: queueName, Found: true}, nil
	}
	resolved, err := app.ResolvePrinterAttempt(sessionID, attemptID, session.PrintResolutionUnknown,
		"Could not establish whether this job produced paper; permanently closing without retry")
	if err != nil {
		t.Fatalf("ResolvePrinterAttempt unknown: %v", err)
	}
	if resolved.Decision != session.PrintResolutionUnknown || !resolved.QueueFound {
		t.Fatalf("unknown resolution did not preserve the queue observation: %+v", resolved)
	}
	manifest, err := session.ReadManifest(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.PrintResolutions) != 1 || manifest.PrintResolutions[0].Decision != session.PrintResolutionUnknown {
		t.Fatalf("unknown resolution was not recorded: %+v", manifest.PrintResolutions)
	}
}

func TestRecoveryItemViewIncludesPrintResolutionAuditRecord(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Nanosecond)
	resolution := session.PrintResolution{
		AttemptID:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Decision:       session.PrintResolutionNotPrinted,
		QueueFound:     false,
		QueueCheckedAt: now.Add(-time.Second),
		RecordedAt:     now,
		Note:           "checked physical output tray; no paper was produced",
	}
	view := recoveryItemView(session.RecoveryItem{
		SessionID:        "a123456789abcdef0123456789abcdef",
		Status:           session.RecoveryFinalized,
		PrintResolutions: []session.PrintResolution{resolution},
	})
	if len(view.PrintResolutions) != 1 {
		t.Fatalf("print resolutions=%d, want 1", len(view.PrintResolutions))
	}
	got := view.PrintResolutions[0]
	if got.AttemptID != resolution.AttemptID || got.Decision != resolution.Decision ||
		got.QueueFound || got.Note != resolution.Note {
		t.Fatalf("print resolution view=%+v", got)
	}
	if _, err := time.Parse(time.RFC3339Nano, got.QueueCheckedAt); err != nil {
		t.Fatalf("queue_checked_at=%q is not RFC3339: %v", got.QueueCheckedAt, err)
	}
	if _, err := time.Parse(time.RFC3339Nano, got.RecordedAt); err != nil {
		t.Fatalf("recorded_at=%q is not RFC3339: %v", got.RecordedAt, err)
	}
}

func createReleasedPrinterLeaseForTest(t *testing.T, sessionDir, sessionID, attemptID string) session.PrintLockInfo {
	t.Helper()
	renderedPath := filepath.Join(sessionDir, "rendered.jpg")
	sourceHash, err := hashRecoveredRenderedFile(renderedPath)
	if err != nil {
		t.Fatal(err)
	}
	info := session.PrintLockInfo{
		AttemptID: attemptID, QueueName: "DS-RX1",
		DocumentName: session.PrintDocumentName(sessionID, attemptID),
		SourceSHA256: sourceHash, CreatedAt: time.Now().UTC(),
	}
	lease, err := session.CreatePrintLockLease(session.PrintLockPath(sessionDir), sessionID, info)
	if err != nil {
		t.Fatalf("CreatePrintLockLease: %v", err)
	}
	if err := lease.Close(); err != nil {
		t.Fatalf("Close lease to simulate process exit: %v", err)
	}
	return info
}

func TestReconcileOrphanedLockOnlyCrashCreatesUncertainAttempt(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("kernel-held printer lock reconciliation is Windows-only")
	}
	app := newRecoveryTestApp(t)
	const sessionID = "f123456789abcdef0123456789abcdef"
	const attemptID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	sessionDir, _ := seedInterruptedWailsSession(t, app, sessionID, session.PhaseOutputFinalized, 3, true)
	lockInfo := createReleasedPrinterLeaseForTest(t, sessionDir, sessionID, attemptID)

	// A point-in-time read should now report orphaned; Reconcile claims it again
	// and holds the exclusive handle through the manifest write and removal.
	items, err := app.listInterruptedPhotoSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].PrintLockOwner != session.PrintLockOwnerOrphaned {
		t.Fatalf("orphan lock discovery=%+v", items)
	}

	reconciled, err := app.ReconcileOrphanedPrinterLock(sessionID)
	if err != nil {
		t.Fatalf("ReconcileOrphanedPrinterLock: %v", err)
	}
	if reconciled.AttemptID != attemptID || reconciled.Status != session.PrintStatusUncertain ||
		!reconciled.Created || !reconciled.Removed {
		t.Fatalf("reconciliation result=%+v", reconciled)
	}
	manifest, err := session.ReadManifest(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.PrintAttempts) != 1 {
		t.Fatalf("attempt count=%d, want 1", len(manifest.PrintAttempts))
	}
	attempt := manifest.PrintAttempts[0]
	if attempt.AttemptID != lockInfo.AttemptID || attempt.Status != session.PrintStatusUncertain ||
		attempt.QueueName != lockInfo.QueueName || attempt.DocumentName != lockInfo.DocumentName ||
		attempt.SourceSHA256 != lockInfo.SourceSHA256 {
		t.Fatalf("recovered attempt=%+v", attempt)
	}
	if _, err := os.Stat(session.PrintLockPath(sessionDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("exclusive orphan lock was not removed: %v", err)
	}
	items, err = app.listInterruptedPhotoSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].PrintLockPresent || len(items[0].PrintAttempts) != 1 ||
		items[0].PrintAttempts[0].Status != session.PrintStatusUncertain {
		t.Fatalf("post-reconciliation recovery state=%+v", items)
	}
}

func TestReconcileOrphanedLockRefusesWhileOwnerLeaseIsActive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("kernel-held printer lock reconciliation is Windows-only")
	}
	app := newRecoveryTestApp(t)
	const sessionID = "a223456789abcdef0123456789abcdef"
	const attemptID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	sessionDir, _ := seedInterruptedWailsSession(t, app, sessionID, session.PhaseOutputFinalized, 3, true)
	sourceHash, err := hashRecoveredRenderedFile(filepath.Join(sessionDir, "rendered.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	lease, err := session.CreatePrintLockLease(session.PrintLockPath(sessionDir), sessionID, session.PrintLockInfo{
		AttemptID: attemptID, QueueName: "DS-RX1",
		DocumentName: session.PrintDocumentName(sessionID, attemptID),
		SourceSHA256: sourceHash, CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	items, err := app.listInterruptedPhotoSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].PrintLockOwner != session.PrintLockOwnerActive {
		t.Fatalf("active kernel lease owner state=%+v", items)
	}
	if _, err := app.ReconcileOrphanedPrinterLock(sessionID); err == nil || !strings.Contains(err.Error(), "still held") {
		t.Fatalf("active lock reconciliation error=%v, want active-owner refusal", err)
	}
	manifest, err := session.ReadManifest(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.PrintAttempts) != 0 {
		t.Fatalf("active lock reconciliation wrote an attempt: %+v", manifest.PrintAttempts)
	}
	if _, err := os.Stat(session.PrintLockPath(sessionDir)); err != nil {
		t.Fatalf("active lock marker was changed: %v", err)
	}
}

func TestReconcileOrphanedLockConvertsPersistedSubmittingAttemptWithoutDuplicate(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("kernel-held printer lock reconciliation is Windows-only")
	}
	app := newRecoveryTestApp(t)
	const sessionID = "b223456789abcdef0123456789abcdef"
	const attemptID = "cccccccccccccccccccccccccccccccc"
	sessionDir, _ := seedInterruptedWailsSession(t, app, sessionID, session.PhaseOutputFinalized, 3, true)
	lockInfo := createReleasedPrinterLeaseForTest(t, sessionDir, sessionID, attemptID)
	manifest, err := session.ReadManifest(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	requested := manifest.UpdatedAt.Add(time.Second)
	attempt := session.PrintAttempt{
		AttemptID: attemptID, QueueName: lockInfo.QueueName,
		DocumentName: lockInfo.DocumentName, SourceSHA256: lockInfo.SourceSHA256,
		Status: session.PrintStatusSubmitting, RequestedAt: requested, UpdatedAt: requested,
	}
	if err := session.BeginPrintAttempt(&manifest, attempt); err != nil {
		t.Fatal(err)
	}
	if err := session.WriteManifest(sessionDir, manifest); err != nil {
		t.Fatal(err)
	}

	result, err := app.ReconcileOrphanedPrinterLock(sessionID)
	if err != nil {
		t.Fatalf("ReconcileOrphanedPrinterLock: %v", err)
	}
	if result.Created || result.Status != session.PrintStatusUncertain {
		t.Fatalf("reconciliation result=%+v; expected conversion of existing intent, not duplicate", result)
	}
	manifest, err = session.ReadManifest(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.PrintAttempts) != 1 || manifest.PrintAttempts[0].Status != session.PrintStatusUncertain {
		t.Fatalf("reconciliation duplicated or failed to convert the attempt: %+v", manifest.PrintAttempts)
	}
}

func TestReconcileOrphanedPrinterLockKeepsLegacyLockUnverifiable(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("kernel-held printer lock reconciliation is Windows-only")
	}
	app := newRecoveryTestApp(t)
	const sessionID = "c223456789abcdef0123456789abcdef"
	const attemptID = "dddddddddddddddddddddddddddddddd"
	sessionDir, _ := seedInterruptedWailsSession(t, app, sessionID, session.PhaseOutputFinalized, 3, true)
	legacy := session.PrintLockInfo{
		AttemptID: attemptID, QueueName: "DS-RX1",
		DocumentName: session.PrintDocumentName(sessionID, attemptID),
		CreatedAt:    time.Now().UTC(),
	}
	legacyBytes, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	lockPath := session.PrintLockPath(sessionDir)
	if err := os.WriteFile(lockPath, legacyBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.ReconcileOrphanedPrinterLock(sessionID); err == nil || !strings.Contains(err.Error(), "legacy") {
		t.Fatalf("legacy lock reconciliation error=%v, want legacy refusal", err)
	}
	after, err := os.ReadFile(lockPath)
	if err != nil || string(after) != string(before) {
		t.Fatalf("legacy lock was modified: err=%v", err)
	}
	manifest, err := session.ReadManifest(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.PrintAttempts) != 0 {
		t.Fatalf("legacy lock reconciliation wrote a print attempt: %+v", manifest.PrintAttempts)
	}
}
