package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dongsinhho/PhotoBooth/internal/booth/framepackage"
	"github.com/dongsinhho/PhotoBooth/internal/booth/session"
	"github.com/dongsinhho/PhotoBooth/internal/booth/workflow"
)

func TestDemoSessionLifecycle(t *testing.T) {
	app := NewApp()
	started := app.StartDemoSession()
	if started.ID == "" || started.State != "capturing" || started.Total != 3 || started.Captured != 0 {
		t.Fatalf("unexpected start state: %+v", started)
	}

	for shot := 1; shot <= 3; shot++ {
		got, err := app.RecordDemoShot(started.ID)
		if err != nil {
			t.Fatalf("RecordDemoShot %d: %v", shot, err)
		}
		if got.Captured != shot {
			t.Fatalf("captured=%d, want %d", got.Captured, shot)
		}
	}
	if app.session.State != "ready_to_finish" {
		t.Fatalf("state=%q, want ready_to_finish", app.session.State)
	}
	if _, err := app.RecordDemoShot(started.ID); err == nil {
		t.Fatal("expected fourth shot to be rejected")
	}
	finished, err := app.FinishDemoSession(started.ID)
	if err != nil {
		t.Fatalf("FinishDemoSession: %v", err)
	}
	if finished.State != "completed" || finished.Captured != 3 {
		t.Fatalf("unexpected finished state: %+v", finished)
	}
	reset := app.ResetDemoSession()
	if reset.State != "idle" || reset.Captured != 0 || reset.ID != "" {
		t.Fatalf("unexpected reset state: %+v", reset)
	}
}

func TestDemoSessionRejectsWrongSessionID(t *testing.T) {
	app := NewApp()
	app.StartDemoSession()
	if _, err := app.RecordDemoShot("not-the-session"); err == nil {
		t.Fatal("expected wrong session ID to be rejected")
	}
	if _, err := app.FinishDemoSession("not-the-session"); err == nil {
		t.Fatal("expected wrong session ID to be rejected for finish")
	}
}

func TestPingEchoesSequenceAndBenchmarkModeDefaultsOff(t *testing.T) {
	app := NewApp()
	if got := app.Ping(42); got != 42 {
		t.Fatalf("Ping returned %d, want 42", got)
	}
	if app.IsBenchmarkMode() {
		t.Fatal("benchmark mode must be disabled unless explicitly requested")
	}
}

func TestCameraDisconnectCanContinueSameSession(t *testing.T) {
	app := NewApp()
	started := app.StartDemoSession()
	shot, err := app.RecordDemoShot(started.ID)
	if err != nil {
		t.Fatalf("record first shot: %v", err)
	}
	failed, err := app.SimulateCameraDisconnect(started.ID)
	if err != nil {
		t.Fatalf("simulate camera disconnect: %v", err)
	}
	if failed.State != "failed" || failed.Captured != 1 || failed.FailureReason == "" {
		t.Fatalf("unexpected failed session: %+v", failed)
	}
	if _, err := app.RecordDemoShot(started.ID); err == nil {
		t.Fatal("capture must be rejected while camera session is failed")
	}
	continued, err := app.ContinueDemoSession(started.ID)
	if err != nil {
		t.Fatalf("continue session: %v", err)
	}
	if continued.ID != started.ID || continued.State != "capturing" || continued.Captured != shot.Captured || continued.FailureReason != "" {
		t.Fatalf("unexpected continued session: %+v", continued)
	}
	for shotNumber := 2; shotNumber <= 3; shotNumber++ {
		continued, err = app.RecordDemoShot(started.ID)
		if err != nil {
			t.Fatalf("record shot %d after recovery: %v", shotNumber, err)
		}
	}
	finished, err := app.FinishDemoSession(started.ID)
	if err != nil {
		t.Fatalf("finish session after recovery: %v", err)
	}
	if finished.State != "completed" || finished.Captured != 3 {
		t.Fatalf("unexpected completed session: %+v", finished)
	}
}

func TestFailedSessionCanRestartWithNewIdentity(t *testing.T) {
	app := NewApp()
	started := app.StartDemoSession()
	if _, err := app.RecordDemoShot(started.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SimulateCameraDisconnect(started.ID); err != nil {
		t.Fatal(err)
	}
	restarted, err := app.RestartDemoSession(started.ID)
	if err != nil {
		t.Fatalf("restart failed session: %v", err)
	}
	if restarted.ID == "" || restarted.ID == started.ID || restarted.State != "capturing" || restarted.Captured != 0 {
		t.Fatalf("restart should produce a clean session with new identity: old=%+v new=%+v", started, restarted)
	}
	if _, err := app.ContinueDemoSession(started.ID); err == nil {
		t.Fatal("stale session ID must not continue the replacement session")
	}
	if _, err := app.RestartDemoSession(started.ID); err == nil {
		t.Fatal("stale session ID must not restart the replacement session")
	}
	if app.session.ID != restarted.ID || app.session.Captured != 0 {
		t.Fatalf("stale operation changed active session: %+v", app.session)
	}
}

func TestCameraDisconnectRejectsInvalidTransitions(t *testing.T) {
	app := NewApp()
	idle := app.session
	if _, err := app.SimulateCameraDisconnect(idle.ID); err == nil {
		t.Fatal("idle session cannot become a camera-failure session")
	}
	started := app.StartDemoSession()
	if _, err := app.ContinueDemoSession(started.ID); err == nil {
		t.Fatal("active session cannot be continued from a failed-session action")
	}
	if _, err := app.RestartDemoSession(started.ID); err == nil {
		t.Fatal("active session cannot be restarted through failed-session action")
	}
	if _, err := app.SimulateCameraDisconnect("wrong-session"); err == nil {
		t.Fatal("camera failure must validate the session ID")
	}
}

func TestDemoPhotoSessionRunsCaptureRenderAndLocalSave(t *testing.T) {
	app := NewApp()
	framePath, err := filepath.Abs(filepath.Join("assets", "photo-strip-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(framePath); err != nil {
		t.Fatalf("embedded sample frame missing: %v", err)
	}
	tmp := t.TempDir()
	app.framePath = framePath
	app.workDir = filepath.Join(tmp, "sessions")
	app.outputDir = filepath.Join(tmp, "photos")
	app.playerCountdown = 0
	app.playerInterval = 0

	started, err := app.StartDemoPhotoSession()
	if err != nil {
		t.Fatalf("StartDemoPhotoSession: %v", err)
	}
	deadline := time.Now().Add(15 * time.Second)
	var final PlayerSession
	for time.Now().Before(deadline) {
		final = app.CurrentPhotoSession()
		switch final.State {
		case "completed", "partial_success", "failed", "cancelled", "timed_out":
			goto finished
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("demo session timed out waiting for completion; last state=%+v", final)

finished:
	if final.ID != started.ID {
		t.Fatalf("session ID changed unexpectedly: started=%s final=%s", started.ID, final.ID)
	}
	if final.State != "completed" {
		t.Fatalf("session ended in %q: %s", final.State, final.ErrorMessage)
	}
	if final.Captured != 3 {
		t.Fatalf("captured=%d, want 3", final.Captured)
	}
	if final.OutputPath == "" {
		t.Fatal("workflow completed without a local output path")
	}
	if info, err := os.Stat(final.OutputPath); err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		t.Fatalf("output missing/empty: path=%s info=%v err=%v", final.OutputPath, info, err)
	}
	if !strings.HasPrefix(final.PreviewDataURL, "data:image/jpeg;base64,") {
		info, statErr := os.Stat(final.RenderedPath)
		probe, previewErr := fileImageDataURL(final.RenderedPath)
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		t.Fatalf("final strip preview data URL is missing: rendered=%q size=%d statErr=%v previewErr=%v probeLen=%d storedLen=%d state=%s output=%q", final.RenderedPath, size, statErr, previewErr, len(probe), len(final.PreviewDataURL), final.State, final.OutputPath)
	}
	if len(final.CapturePreviews) != 3 {
		t.Fatalf("capture preview count=%d, want 3", len(final.CapturePreviews))
	}
}

func TestPlayerSettingsPersistAndReload(t *testing.T) {
	dataDir := t.TempDir()
	app := NewApp()
	app.playerDataDir = dataDir
	settings, err := app.GetPlayerSettings()
	if err != nil {
		t.Fatalf("GetPlayerSettings: %v", err)
	}
	if settings.ShotCount != 3 || settings.Countdown != "3s" || settings.Interval != "1s" {
		t.Fatalf("unexpected default settings: %+v", settings)
	}
	if settings.ConfigPath == "" {
		t.Fatal("settings config path is empty")
	}
	if _, err := os.Stat(settings.FramePath); err != nil {
		t.Fatalf("default frame file missing: %v", err)
	}

	settings.Countdown = "0s"
	settings.Interval = "250ms"
	saved, err := app.SavePlayerSettings(settings)
	if err != nil {
		t.Fatalf("SavePlayerSettings: %v", err)
	}
	if saved.LoadWarning != "" || saved.Countdown != "0s" || saved.Interval != "250ms" {
		t.Fatalf("unexpected saved settings: %+v", saved)
	}

	reloadedApp := NewApp()
	reloadedApp.playerDataDir = dataDir
	reloaded, err := reloadedApp.GetPlayerSettings()
	if err != nil {
		t.Fatalf("reload GetPlayerSettings: %v", err)
	}
	if reloaded.Countdown != "0s" || reloaded.Interval != "250ms" || reloaded.ShotCount != 3 {
		t.Fatalf("settings did not persist: %+v", reloaded)
	}
	if reloaded.LoadWarning != "" {
		t.Fatalf("unexpected warning after reload: %s", reloaded.LoadWarning)
	}
}

func TestPlayerSettingsRejectShotCountWhenFrameSlotCountDiffers(t *testing.T) {
	app := NewApp()
	app.playerDataDir = t.TempDir()
	settings, err := app.GetPlayerSettings()
	if err != nil {
		t.Fatal(err)
	}
	frameBytes, err := os.ReadFile(settings.FramePath)
	if err != nil {
		t.Fatal(err)
	}
	var frame map[string]any
	if err := json.Unmarshal(frameBytes, &frame); err != nil {
		t.Fatal(err)
	}
	slots, ok := frame["slots"].([]any)
	if !ok || len(slots) < 2 {
		t.Fatalf("sample frame does not have enough slots: %#v", frame["slots"])
	}
	frame["slots"] = slots[:2]
	twoSlotPath := filepath.Join(app.playerDataDir, "two-slot-frame.json")
	twoSlotBytes, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(twoSlotPath, twoSlotBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	settings.FramePath = twoSlotPath
	settings.ShotCount = 3
	if _, err := app.SavePlayerSettings(settings); err == nil || !strings.Contains(err.Error(), "frame defines 2 slots") {
		t.Fatalf("SavePlayerSettings error=%v, want slot mismatch", err)
	}
	reloaded, err := app.GetPlayerSettings()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.FramePath != filepath.Clean(filepath.Join(app.playerDataDir, "frames", "photo-strip-v1.json")) || reloaded.ShotCount != 3 {
		t.Fatalf("invalid settings should not replace the saved profile: %+v", reloaded)
	}
}

func TestInvalidStationConfigIsNotSilentlyOverwrittenAndCanBeRepaired(t *testing.T) {
	dataDir := t.TempDir()
	app := NewApp()
	app.playerDataDir = dataDir
	initial, err := app.GetPlayerSettings()
	if err != nil {
		t.Fatal(err)
	}
	corrupt := []byte("{not valid json")
	if err := os.WriteFile(initial.ConfigPath, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}

	restarted := NewApp()
	restarted.playerDataDir = dataDir
	settings, err := restarted.GetPlayerSettings()
	if err != nil {
		t.Fatalf("settings UI should remain available to repair invalid config: %v", err)
	}
	if settings.LoadWarning == "" {
		t.Fatal("expected a visible config warning")
	}
	actual, err := os.ReadFile(settings.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != string(corrupt) {
		t.Fatal("invalid config was overwritten without operator action")
	}
	if _, err := restarted.StartDemoPhotoSession(); err == nil || !strings.Contains(err.Error(), "station settings need attention") {
		t.Fatalf("capture should be blocked until config is repaired, got %v", err)
	}
	if _, err := restarted.SavePlayerSettings(settings); err != nil {
		t.Fatalf("operator could not save a repaired profile: %v", err)
	}
	repaired, err := restarted.GetPlayerSettings()
	if err != nil {
		t.Fatal(err)
	}
	if repaired.LoadWarning != "" {
		t.Fatalf("repaired profile still has warning: %s", repaired.LoadWarning)
	}
}

func TestPlayerSettingsCannotBeChangedDuringSession(t *testing.T) {
	app := NewApp()
	app.playerDataDir = t.TempDir()
	settings, err := app.GetPlayerSettings()
	if err != nil {
		t.Fatal(err)
	}
	app.mu.Lock()
	app.playerRunning = true
	app.mu.Unlock()
	if _, err := app.SavePlayerSettings(settings); err == nil || !strings.Contains(err.Error(), "while a photo session is running") {
		t.Fatalf("SavePlayerSettings error=%v, want running-session guard", err)
	}
}
func TestSavedShotCountDrivesDemoWorkflow(t *testing.T) {
	dataDir := t.TempDir()
	app := NewApp()
	app.playerDataDir = dataDir
	settings, err := app.GetPlayerSettings()
	if err != nil {
		t.Fatal(err)
	}
	frameBytes, err := os.ReadFile(settings.FramePath)
	if err != nil {
		t.Fatal(err)
	}
	var frame map[string]any
	if err := json.Unmarshal(frameBytes, &frame); err != nil {
		t.Fatal(err)
	}
	slots := frame["slots"].([]any)
	frame["slots"] = slots[:2]
	twoSlotPath := filepath.Join(dataDir, "two-slot-frame.json")
	twoSlotBytes, err := json.MarshalIndent(frame, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(twoSlotPath, twoSlotBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	settings.ShotCount = 2
	settings.Countdown = "0s"
	settings.Interval = "0s"
	settings.FramePath = twoSlotPath
	if _, err := app.SavePlayerSettings(settings); err != nil {
		t.Fatalf("SavePlayerSettings: %v", err)
	}
	if got := app.CurrentPhotoSession().Total; got != 2 {
		t.Fatalf("idle session total=%d, want 2 after saving settings", got)
	}

	started, err := app.StartDemoPhotoSession()
	if err != nil {
		t.Fatalf("StartDemoPhotoSession: %v", err)
	}
	deadline := time.Now().Add(15 * time.Second)
	var final PlayerSession
	for time.Now().Before(deadline) {
		final = app.CurrentPhotoSession()
		switch final.State {
		case "completed", "partial_success", "failed", "cancelled", "timed_out":
			goto finishedTwoShot
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("two-shot demo session timed out; last state=%+v", final)

finishedTwoShot:
	if final.ID != started.ID || final.State != "completed" {
		t.Fatalf("two-shot demo ended unexpectedly: started=%+v final=%+v", started, final)
	}
	if final.Total != 2 || final.Captured != 2 || len(final.CapturePreviews) != 2 {
		t.Fatalf("saved shot count was not used through the workflow: %+v", final)
	}
	if final.OutputPath == "" || !strings.HasPrefix(final.PreviewDataURL, "data:image/jpeg;base64,") {
		info, _ := os.Stat(final.RenderedPath)
		_, previewErr := fileImageDataURL(final.RenderedPath)
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		t.Fatalf("two-shot session missing final local output or preview: state=%s captured=%d total=%d output=%q rendered=%q rendered_size=%d preview_len=%d preview_err=%v", final.State, final.Captured, final.Total, final.OutputPath, final.RenderedPath, size, len(final.PreviewDataURL), previewErr)
	}
}
func TestPlayerSettingsCannotSaveWithoutInitializedProfile(t *testing.T) {
	app := NewApp()
	app.framePath = filepath.Join(t.TempDir(), "frame.json")
	app.workDir = t.TempDir()
	app.outputDir = t.TempDir()
	if err := os.WriteFile(app.framePath, embeddedSampleFrame, 0o600); err != nil {
		t.Fatal(err)
	}
	settings := PlayerSettings{ShotCount: 3, Countdown: "3s", Interval: "1s", FramePath: app.framePath}
	if _, err := app.SavePlayerSettings(settings); err == nil || !strings.Contains(err.Error(), "settings path is not initialized") {
		t.Fatalf("SavePlayerSettings error=%v, want uninitialized-profile guard", err)
	}
}
func TestFramePackageInstallActivateAndRollbackThroughWailsAPI(t *testing.T) {
	dataDir := t.TempDir()
	app := NewApp()
	app.playerDataDir = dataDir
	initial, err := app.GetPlayerSettings()
	if err != nil {
		t.Fatalf("GetPlayerSettings: %v", err)
	}
	originalFramePath := initial.FramePath
	frameBytes, err := os.ReadFile(originalFramePath)
	if err != nil {
		t.Fatal(err)
	}
	var frame map[string]any
	if err := json.Unmarshal(frameBytes, &frame); err != nil {
		t.Fatal(err)
	}
	slots, ok := frame["slots"].([]any)
	if !ok || len(slots) != 3 {
		t.Fatalf("unexpected sample-frame slots: %#v", frame["slots"])
	}
	frame["slots"] = slots[:2]
	twoSlotPath := filepath.Join(dataDir, "candidate-two-slot.json")
	twoSlotBytes, err := json.MarshalIndent(frame, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(twoSlotPath, twoSlotBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(dataDir, "two-photo-frame"+framepackage.ArchiveExtension)
	if _, err := framepackage.CreatePackage(context.Background(), framepackage.CreateOptions{
		FramePath: twoSlotPath, OutputPath: archivePath, PackageID: "two-photo-frame",
		PackageVersion: "1.0.0", Name: "Two photo frame", Description: "Wails integration fixture",
		Author: "PhotoBooth test", License: "CC0-1.0",
	}); err != nil {
		t.Fatalf("CreatePackage: %v", err)
	}

	validated, err := app.ValidateFramePackage(archivePath)
	if err != nil {
		t.Fatalf("ValidateFramePackage: %v", err)
	}
	if validated.SlotCount != 2 || validated.FramePath != "" {
		t.Fatalf("validation should not install or activate the package: %+v", validated)
	}
	beforeInstall, err := app.GetFramePackageState()
	if err != nil {
		t.Fatal(err)
	}
	if len(beforeInstall.Packages) != 0 {
		t.Fatalf("ValidateFramePackage must not install content: %+v", beforeInstall.Packages)
	}

	installed, err := app.InstallFramePackage(archivePath)
	if err != nil {
		t.Fatalf("InstallFramePackage: %v", err)
	}
	if installed.FramePath == "" || installed.SlotCount != 2 {
		t.Fatalf("installed package missing frame path/slot count: %+v", installed)
	}
	afterInstall, err := app.GetPlayerSettings()
	if err != nil {
		t.Fatal(err)
	}
	if afterInstall.FramePath != originalFramePath || afterInstall.ShotCount != 3 {
		t.Fatalf("install must not activate a package: %+v", afterInstall)
	}
	bypass := afterInstall
	bypass.FramePath = installed.FramePath
	bypass.ShotCount = 2
	if _, err := app.SavePlayerSettings(bypass); err == nil || !strings.Contains(err.Error(), "rollback point is created") {
		t.Fatalf("manual settings must not bypass package activation/rollback: %v", err)
	}
	stillOriginal, err := app.GetPlayerSettings()
	if err != nil {
		t.Fatal(err)
	}
	if stillOriginal.FramePath != originalFramePath || stillOriginal.ShotCount != 3 {
		t.Fatalf("rejected package-path bypass changed active settings: %+v", stillOriginal)
	}

	activeSettings, err := app.ActivateFramePackage(installed.PackageID, installed.PackageVersion)
	if err != nil {
		t.Fatalf("ActivateFramePackage: %v", err)
	}
	if activeSettings.ShotCount != 2 || activeSettings.FramePath != installed.FramePath {
		t.Fatalf("activation did not align settings with frame slots: %+v package=%+v", activeSettings, installed)
	}
	state, err := app.GetFramePackageState()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Packages) != 1 || !state.Packages[0].IsActive || !state.CanRollback {
		t.Fatalf("active package/rollback state incorrect: %+v", state)
	}
	if err := app.UninstallFramePackage(installed.PackageID, installed.PackageVersion); err == nil || !strings.Contains(err.Error(), "currently active") {
		t.Fatalf("UninstallFramePackage should reject active package: %v", err)
	}

	restored, err := app.RollbackActiveFrame()
	if err != nil {
		t.Fatalf("RollbackActiveFrame: %v", err)
	}
	if restored.ShotCount != 3 || restored.FramePath == installed.FramePath {
		t.Fatalf("rollback did not restore previous 3-slot frame: %+v", restored)
	}
	state, err = app.GetFramePackageState()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Packages) != 1 || state.Packages[0].IsActive || state.CanRollback {
		t.Fatalf("rollback must keep installed package but deactivate it and clear rollback: %+v", state)
	}
	if _, err := os.Stat(installed.FramePath); err != nil {
		t.Fatalf("rollback deleted the installed package definition: %v", err)
	}
	if err := app.UninstallFramePackage(installed.PackageID, installed.PackageVersion); err != nil {
		t.Fatalf("UninstallFramePackage after rollback: %v", err)
	}
	state, err = app.GetFramePackageState()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Packages) != 0 {
		t.Fatalf("package should be removed from the catalog after uninstall: %+v", state.Packages)
	}
}

func TestFramePackageOperationsAreBlockedDuringCapture(t *testing.T) {
	app := NewApp()
	app.playerDataDir = t.TempDir()
	if _, err := app.GetPlayerSettings(); err != nil {
		t.Fatal(err)
	}
	app.mu.Lock()
	app.playerRunning = true
	app.mu.Unlock()
	if _, err := app.InstallFramePackage("not-read.pbframe"); err == nil || !strings.Contains(err.Error(), "while a photo session is running") {
		t.Fatalf("InstallFramePackage error=%v, want running-session guard", err)
	}
	if err := app.UninstallFramePackage("unknown-frame", "1.0.0"); err == nil || !strings.Contains(err.Error(), "while a photo session is running") {
		t.Fatalf("UninstallFramePackage error=%v, want running-session guard", err)
	}
	if _, err := app.ActivateFramePackage("unknown-frame", "1.0.0"); err == nil || !strings.Contains(err.Error(), "while a photo session is running") {
		t.Fatalf("ActivateFramePackage error=%v, want running-session guard", err)
	}
	if _, err := app.RollbackActiveFrame(); err == nil || !strings.Contains(err.Error(), "while a photo session is running") {
		t.Fatalf("RollbackActiveFrame error=%v, want running-session guard", err)
	}
}
func TestTerminalWorkflowEventDoesNotPublishFinalStateBeforePreview(t *testing.T) {
	terminalStates := []session.State{session.Completed, session.PartialSuccess, session.Failed, session.Cancelled, session.TimedOut}
	for _, terminal := range terminalStates {
		t.Run(string(terminal), func(t *testing.T) {
			app := NewApp()
			app.mu.Lock()
			app.player = PlayerSession{ID: "session-fixture", State: string(session.Output), Total: 3, Message: "Writing local output…"}
			app.playerRunning = true
			app.mu.Unlock()

			app.handleWorkflowEvent("session-fixture", workflow.Event{
				Type: "state", State: terminal, Message: "terminal state emitted by runner",
			})
			got := app.CurrentPhotoSession()
			if got.State == string(terminal) {
				t.Fatalf("terminal state %q leaked before runPhotoSession finalized preview/output", terminal)
			}
			if !strings.Contains(got.Message, "Finalizing") {
				t.Fatalf("message=%q, want finalization message", got.Message)
			}
		})
	}
}

func seedFailedLocalOutputSession(t *testing.T, app *App, sessionID string) string {
	t.Helper()
	settings, err := app.GetPlayerSettings()
	if err != nil {
		t.Fatalf("GetPlayerSettings: %v", err)
	}
	sessionDir := filepath.Join(app.workDir, sessionID)
	captureDir := filepath.Join(sessionDir, "captures")
	if err := os.MkdirAll(captureDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for shot := 1; shot <= settings.ShotCount; shot++ {
		path := filepath.Join(captureDir, fmt.Sprintf("capture-%02d.jpg", shot))
		if err := os.WriteFile(path, []byte(fmt.Sprintf("preserved-capture-%d", shot)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	renderedPath := filepath.Join(sessionDir, "rendered.jpg")
	renderedBytes := []byte("preserved-rendered-photo")
	if err := os.WriteFile(renderedPath, renderedBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	previews := make([]string, settings.ShotCount)
	for i := range previews {
		previews[i] = "data:image/jpeg;base64,cHJldmlldw=="
	}
	app.mu.Lock()
	app.player = PlayerSession{
		ID: sessionID, State: string(session.Failed),
		Captured: settings.ShotCount, Total: settings.ShotCount,
		Message:      "Local output failed because storage is full.",
		ErrorMessage: "insufficient storage space",
		RenderedPath: renderedPath, CapturePreviews: previews,
		PreviewDataURL: "data:image/jpeg;base64,cHJldmlldw==",
	}
	app.mu.Unlock()
	return renderedPath
}

func TestRetryPhotoOutputSavesPreservedRenderWithoutRecapturing(t *testing.T) {
	app := NewApp()
	app.playerDataDir = t.TempDir()
	sessionID := "0123456789abcdef0123456789abcdef"
	renderedPath := seedFailedLocalOutputSession(t, app, sessionID)

	final, err := app.RetryPhotoOutput(sessionID)
	if err != nil {
		t.Fatalf("RetryPhotoOutput: %v", err)
	}
	if final.State != string(session.Completed) || final.OutputRetrying || final.OutputPath == "" {
		t.Fatalf("unexpected retry result: %+v", final)
	}
	if final.Captured != final.Total || final.ID != sessionID {
		t.Fatalf("retry changed session identity/progress: %+v", final)
	}
	got, err := os.ReadFile(final.OutputPath)
	if err != nil {
		t.Fatalf("read final photo: %v", err)
	}
	source, err := os.ReadFile(renderedPath)
	if err != nil {
		t.Fatalf("read preserved render: %v", err)
	}
	if string(got) != string(source) {
		t.Fatalf("final photo=%q, want preserved render=%q", got, source)
	}
	for shot := 1; shot <= final.Total; shot++ {
		path := filepath.Join(app.workDir, sessionID, "captures", fmt.Sprintf("capture-%02d.jpg", shot))
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("retry lost capture %d: %v", shot, err)
		}
	}
	if got := app.CurrentPhotoSession(); got.State != string(session.Completed) || got.OutputPath != final.OutputPath {
		t.Fatalf("completed output retry was not persisted in player state: %+v", got)
	}
}

func TestRetryPhotoOutputRejectsWrongSessionAndExistingOutput(t *testing.T) {
	app := NewApp()
	app.playerDataDir = t.TempDir()
	sessionID := "0123456789abcdef0123456789abcdef"
	seedFailedLocalOutputSession(t, app, sessionID)

	if _, err := app.RetryPhotoOutput("fedcba9876543210fedcba9876543210"); err == nil || !strings.Contains(err.Error(), "session ID") {
		t.Fatalf("wrong-session retry error=%v", err)
	}
	if _, err := app.RetryPhotoOutput(sessionID); err != nil {
		t.Fatalf("first retry: %v", err)
	}
	previous, err := os.ReadFile(app.player.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.RetryPhotoOutput(sessionID); err == nil || !strings.Contains(err.Error(), "already has a finalized local output") {
		t.Fatalf("second retry error=%v, want already-completed guard", err)
	}
	got, err := os.ReadFile(app.player.OutputPath)
	if err != nil || string(got) != string(previous) {
		t.Fatalf("second retry changed existing output: got=%q err=%v", got, err)
	}
}

func TestRetryPhotoOutputRequiresAllCapturesAndBlocksConcurrentMutations(t *testing.T) {
	app := NewApp()
	app.playerDataDir = t.TempDir()
	sessionID := "0123456789abcdef0123456789abcdef"
	seedFailedLocalOutputSession(t, app, sessionID)

	app.mu.Lock()
	app.player.Captured--
	app.mu.Unlock()
	if _, err := app.RetryPhotoOutput(sessionID); err == nil || !strings.Contains(err.Error(), "requires all photos to be captured") {
		t.Fatalf("incomplete-session retry error=%v", err)
	}

	app.mu.Lock()
	app.player.Captured = app.player.Total
	app.outputRetrying = true
	app.mu.Unlock()
	if _, err := app.RetryPhotoOutput(sessionID); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("concurrent retry error=%v", err)
	}
	if _, err := app.StartDemoPhotoSession(); err == nil || !strings.Contains(err.Error(), "output retry") {
		t.Fatalf("starting a new session during retry error=%v", err)
	}
	app.mu.Lock()
	app.outputRetrying = false
	app.mu.Unlock()
}

func TestRetentionCleanupWailsFlowRequiresReviewedPlan(t *testing.T) {
	app := NewApp()
	app.playerDataDir = t.TempDir()

	settings, err := app.GetPlayerSettings()
	if err != nil {
		t.Fatalf("GetPlayerSettings: %v", err)
	}
	if settings.SessionRetentionDays != 0 || settings.OutputRetentionDays != 0 {
		t.Fatalf("retention must default to disabled: %+v", settings)
	}
	settings.SessionRetentionDays = 30
	settings.OutputRetentionDays = 30
	saved, err := app.SavePlayerSettings(settings)
	if err != nil {
		t.Fatalf("SavePlayerSettings with retention policy: %v", err)
	}
	if saved.SessionRetentionDays != 30 || saved.OutputRetentionDays != 30 {
		t.Fatalf("retention policy did not round-trip: %+v", saved)
	}

	const sessionID = "0123456789abcdef0123456789abcdef"
	sessionPath := seedExpiredRetentionSession(t, app.workDir, sessionID)
	outputPath := seedExpiredRetentionOutput(t, app.outputDir, sessionID)

	preview, err := app.PreviewRetentionCleanup()
	if err != nil {
		t.Fatalf("PreviewRetentionCleanup: %v", err)
	}
	if preview.Disabled || preview.CandidateCount != 2 || preview.TotalFiles != 3 || preview.TotalBytes == 0 {
		t.Fatalf("unexpected dry-run summary: %+v", preview)
	}
	if len(preview.Candidates) != 2 || preview.PlanDigest == "" {
		t.Fatalf("preview did not expose the exact candidate list and digest: %+v", preview)
	}
	if _, err := os.Stat(sessionPath); err != nil {
		t.Fatalf("dry-run changed session data: %v", err)
	}
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("dry-run changed output data: %v", err)
	}

	if _, err := app.ApplyRetentionCleanup(strings.Repeat("0", 64)); err == nil {
		t.Fatal("ApplyRetentionCleanup accepted a mismatched digest")
	}
	if _, err := os.Stat(sessionPath); err != nil {
		t.Fatalf("wrong digest changed session data: %v", err)
	}
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("wrong digest changed output data: %v", err)
	}

	result, err := app.ApplyRetentionCleanup(preview.PlanDigest)
	if err != nil {
		t.Fatalf("ApplyRetentionCleanup with reviewed digest: %v", err)
	}
	if result.RemovedCandidates != 2 || result.RemovedFiles != 3 || result.RemovedBytes != preview.TotalBytes {
		t.Fatalf("unexpected apply result: %+v (preview %+v)", result, preview)
	}
	for _, path := range []string{sessionPath, outputPath} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected reviewed expired folder %q to be removed, stat error=%v", path, err)
		}
	}
	if _, err := app.ApplyRetentionCleanup(preview.PlanDigest); err == nil {
		t.Fatal("a one-use retention plan was replayed")
	}

	reloaded := NewApp()
	reloaded.playerDataDir = app.playerDataDir
	persisted, err := reloaded.GetPlayerSettings()
	if err != nil {
		t.Fatalf("reload retention settings: %v", err)
	}
	if persisted.SessionRetentionDays != 30 || persisted.OutputRetentionDays != 30 {
		t.Fatalf("retention policy did not survive restart: %+v", persisted)
	}
}

func TestRetentionCleanupDefaultsRemainDisabled(t *testing.T) {
	app := NewApp()
	app.playerDataDir = t.TempDir()
	settings, err := app.GetPlayerSettings()
	if err != nil {
		t.Fatal(err)
	}
	seedExpiredRetentionSession(t, app.workDir, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

	preview, err := app.PreviewRetentionCleanup()
	if err != nil {
		t.Fatalf("PreviewRetentionCleanup with disabled policy: %v", err)
	}
	if !preview.Disabled || preview.CandidateCount != 0 || preview.TotalFiles != 0 {
		t.Fatalf("disabled policy should produce an empty, non-destructive plan: %+v", preview)
	}
	if settings.SessionRetentionDays != 0 || settings.OutputRetentionDays != 0 {
		t.Fatalf("default settings unexpectedly enable retention: %+v", settings)
	}
	if _, err := app.ApplyRetentionCleanup(preview.PlanDigest); err == nil {
		t.Fatal("disabled or empty cleanup plan must not be applicable")
	}
}

func TestRetentionOperationsRejectConcurrentSessionAndSettingsChanges(t *testing.T) {
	app := NewApp()
	app.playerDataDir = t.TempDir()
	settings, err := app.GetPlayerSettings()
	if err != nil {
		t.Fatal(err)
	}

	app.mu.Lock()
	app.retentionBusy = true
	app.mu.Unlock()
	if _, err := app.StartDemoPhotoSession(); err == nil || !strings.Contains(err.Error(), "retention cleanup") {
		t.Fatalf("StartDemoPhotoSession error=%v, want retention guard", err)
	}
	if _, err := app.SavePlayerSettings(settings); err == nil || !strings.Contains(err.Error(), "retention cleanup") {
		t.Fatalf("SavePlayerSettings error=%v, want retention guard", err)
	}
	app.mu.Lock()
	app.retentionBusy = false
	app.mu.Unlock()
}

func seedExpiredRetentionSession(t *testing.T, root, sessionID string) string {
	t.Helper()
	sessionPath := filepath.Join(root, sessionID)
	capturesPath := filepath.Join(sessionPath, "captures")
	if err := os.MkdirAll(capturesPath, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, file := range []struct {
		name string
		body string
	}{
		{name: "capture-01.jpg", body: "capture"},
	} {
		if err := os.WriteFile(filepath.Join(capturesPath, file.name), []byte(file.body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(sessionPath, "rendered.jpg"), []byte("rendered"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-45 * 24 * time.Hour)
	for _, path := range []string{
		filepath.Join(capturesPath, "capture-01.jpg"),
		filepath.Join(sessionPath, "rendered.jpg"),
		capturesPath,
		sessionPath,
	} {
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	return sessionPath
}

func seedExpiredRetentionOutput(t *testing.T, root, sessionID string) string {
	t.Helper()
	outputPath := filepath.Join(root, sessionID)
	if err := os.MkdirAll(outputPath, 0o700); err != nil {
		t.Fatal(err)
	}
	photoPath := filepath.Join(outputPath, "photo.jpg")
	if err := os.WriteFile(photoPath, []byte("final-photo"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-45 * 24 * time.Hour)
	if err := os.Chtimes(photoPath, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(outputPath, old, old); err != nil {
		t.Fatal(err)
	}
	return outputPath
}

func TestRetentionPreviewExpiresWhenPolicyChanges(t *testing.T) {
	app := NewApp()
	app.playerDataDir = t.TempDir()
	settings, err := app.GetPlayerSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.SessionRetentionDays = 30
	if _, err := app.SavePlayerSettings(settings); err != nil {
		t.Fatal(err)
	}
	sessionPath := seedExpiredRetentionSession(t, app.workDir, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")

	preview, err := app.PreviewRetentionCleanup()
	if err != nil {
		t.Fatal(err)
	}
	if preview.CandidateCount != 1 {
		t.Fatalf("candidate_count=%d, want 1", preview.CandidateCount)
	}

	settings.SessionRetentionDays = 31
	if _, err := app.SavePlayerSettings(settings); err != nil {
		t.Fatalf("change retention policy: %v", err)
	}
	if _, err := app.ApplyRetentionCleanup(preview.PlanDigest); err == nil {
		t.Fatal("stale preview remained applicable after policy change")
	}
	if _, err := os.Stat(sessionPath); err != nil {
		t.Fatalf("policy change or stale apply removed data: %v", err)
	}
}
