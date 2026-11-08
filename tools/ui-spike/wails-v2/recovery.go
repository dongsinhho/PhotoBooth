package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
	"github.com/dongsinhho/PhotoBooth/internal/booth/session"
	"github.com/dongsinhho/PhotoBooth/internal/booth/workflow"
)

// RecoveryManifestView is the minimal, JSON-safe manifest metadata shown to
// the Wails UI. Keep time.Time and the full domain manifest inside Go.
type RecoveryManifestView struct {
	ExpectedShots int    `json:"expected_shots"`
	DemoMode      bool   `json:"demo_mode"`
	UpdatedAt     string `json:"updated_at"`
}

// RecoveryItemView is the Wails presentation DTO for read-only recovery discovery.
type RecoveryItemView struct {
	SessionID      string                `json:"session_id"`
	DirectoryPath  string                `json:"directory_path"`
	Status         string                `json:"status"`
	Reason         string                `json:"reason,omitempty"`
	CompletedShots int                   `json:"completed_shots"`
	ExpectedShots  int                   `json:"expected_shots"`
	Phase          string                `json:"phase,omitempty"`
	Manifest       *RecoveryManifestView `json:"manifest,omitempty"`
	CapturePaths   []string              `json:"capture_paths,omitempty"`
	RenderedPath   string                `json:"rendered_path,omitempty"`
}

func recoveryCanIgnoreFrameSettingsError(message string, manifest *session.Manifest) bool {
	return manifest != nil && manifest.Version == session.ManifestVersion &&
		manifest.FrameSnapshotFile == session.FrameSnapshotFileName && isFrameOnlySettingsError(message)
}

func isFrameOnlySettingsError(message string) bool {
	return strings.HasPrefix(message, "The saved frame JSON cannot be loaded:") ||
		strings.HasPrefix(message, "The saved capture count is ")
}

func recoveryItemView(item session.RecoveryItem) RecoveryItemView {
	view := RecoveryItemView{
		SessionID: item.SessionID, DirectoryPath: item.DirectoryPath,
		Status: string(item.Status), Reason: item.Reason,
		CompletedShots: item.CompletedShots, ExpectedShots: item.ExpectedShots,
		Phase: string(item.Phase), CapturePaths: append([]string(nil), item.CapturePaths...),
		RenderedPath: item.RenderedPath,
	}
	if item.Manifest != nil {
		updatedAt := ""
		if !item.Manifest.UpdatedAt.IsZero() {
			updatedAt = item.Manifest.UpdatedAt.UTC().Format(time.RFC3339Nano)
		}
		view.Manifest = &RecoveryManifestView{
			ExpectedShots: item.Manifest.ExpectedShots,
			DemoMode:      item.Manifest.DemoMode,
			UpdatedAt:     updatedAt,
		}
	}
	return view
}

func playerWorkflowConfig(runConfig playerRuntimeConfig, sessionID string, resumeCount int, demo, resume bool) workflow.Config {
	cameraProvider := strings.TrimSpace(runConfig.stationConfig.Camera.Provider)
	cameraDevice := strings.TrimSpace(runConfig.stationConfig.Camera.Device)
	cameraFPS := runConfig.stationConfig.Camera.FPS
	if demo {
		cameraProvider, cameraDevice, cameraFPS = "fake", "", 0
	}
	return workflow.Config{
		FrameConfigPath:     runConfig.framePath,
		WorkDir:             runConfig.workDir,
		OutputDir:           runConfig.outputDir,
		CaptureCount:        runConfig.shotCount,
		Countdown:           runConfig.countdown,
		Interval:            runConfig.interval,
		CaptureWidth:        runConfig.stationConfig.Camera.Width,
		CaptureHeight:       runConfig.stationConfig.Camera.Height,
		ConfigSchemaVersion: runConfig.stationConfig.SchemaVersion,
		CameraProvider:      cameraProvider,
		CameraDevice:        cameraDevice,
		CameraFPS:           cameraFPS,
		MediaDeliveryMode:   runConfig.stationConfig.MediaDelivery.Mode,
		PaymentMode:         runConfig.stationConfig.Payment.Mode,
		DemoMode:            demo,
		SessionID:           sessionID,
		ResumeSession:       resume,
		ResumeCaptureCount:  resumeCount,
	}
}

// ListInterruptedPhotoSessions exposes read-only recovery metadata to the UI.
// The full domain manifest and filesystem inspection types remain internal to Go.
func (a *App) ListInterruptedPhotoSessions() ([]RecoveryItemView, error) {
	items, err := a.listInterruptedPhotoSessions()
	if err != nil {
		return nil, err
	}
	views := make([]RecoveryItemView, 0, len(items))
	for _, item := range items {
		views = append(views, recoveryItemView(item))
	}
	return views, nil
}

// listInterruptedPhotoSessions inspects only the configured sessions root.
// It never opens a camera, rewrites a manifest, repairs a folder or deletes data.
func (a *App) listInterruptedPhotoSessions() ([]session.RecoveryItem, error) {
	if err := a.preparePlayerPaths(); err != nil {
		return nil, err
	}

	a.mu.Lock()
	runConfig := a.playerRuntimeConfigLocked()
	settingsError := a.settingsError
	a.mu.Unlock()

	scan, err := session.InspectRecoveryRoot(runConfig.workDir)
	if err != nil {
		return nil, err
	}
	items := make([]session.RecoveryItem, 0, len(scan.Items))
	for _, item := range scan.Items {
		outputSessionDir := filepath.Join(runConfig.outputDir, item.SessionID)
		if err := stationconfig.ValidateStorageChild(runConfig.outputDir, outputSessionDir); err != nil {
			item.Status = session.RecoveryInspectOnly
			item.Reason = "The matching output directory resolves through a redirected path. The session was not changed."
			items = append(items, item)
			continue
		}
		outputPath := filepath.Join(outputSessionDir, "photo.jpg")
		outputExists, outputIssue, outputErr := inspectRecoveredOutputDirectory(outputSessionDir)
		if outputErr != nil {
			item.Status = session.RecoveryInspectOnly
			item.Reason = "The matching output directory could not be inspected safely."
			items = append(items, item)
			continue
		}
		if outputIssue != "" {
			item.Status = session.RecoveryInspectOnly
			item.Reason = outputIssue
			items = append(items, item)
			continue
		}

		switch item.Status {
		case session.RecoveryFinalized:
			if outputExists && item.Manifest != nil && sameLocalPath(item.Manifest.OutputPath, outputPath) {
				// Successfully finalized sessions are not recovery prompts.
				continue
			}
			item.Status = session.RecoveryInspectOnly
			item.Reason = "The manifest says output was finalized, but its exact final file is missing or does not match. The original session was preserved."
		case session.RecoveryOutputPending:
			if outputExists {
				item.Status = session.RecoveryInspectOnly
				item.Reason = "A final output already exists although the manifest is not finalized. It will not be overwritten or retried automatically."
			}
		case session.RecoveryResumeAvailable:
			if outputExists {
				item.Status = session.RecoveryInspectOnly
				item.Reason = "A final output already exists for this session ID. Capture resume is disabled to prevent duplicate publication."
				break
			}
			if settingsError != "" && !recoveryCanIgnoreFrameSettingsError(settingsError, item.Manifest) {
				item.Status = session.RecoveryInspectOnly
				item.Reason = "Station settings are invalid. Review and save a valid profile before resuming this session."
				break
			}
			if item.Manifest == nil {
				item.Status = session.RecoveryInspectOnly
				item.Reason = "A valid recovery manifest is required before this session can continue."
				break
			}
			expectedConfig := playerWorkflowConfig(
				runConfig, item.SessionID, item.Manifest.CompletedShots, item.Manifest.DemoMode, true,
			)
			if item.FrameConfigPath != "" {
				expectedConfig.FrameConfigIdentityPath = item.Manifest.FrameConfigPath
				expectedConfig.FrameConfigPath = item.FrameConfigPath
			}
			expected, manifestErr := workflow.NewSessionManifest(expectedConfig)
			if manifestErr != nil || expected.RunConfigFingerprint != item.Manifest.RunConfigFingerprint {
				item.Status = session.RecoveryInspectOnly
				item.Reason = "The current frame or capture configuration differs from the interrupted session. Its files remain available for inspection only."
			}
		}
		if item.Status != session.RecoveryFinalized {
			items = append(items, item)
		}
	}
	return items, nil
}

// inspectRecoveredOutputDirectory accepts only an absent output directory or an
// empty one. A sole photo.jpg means publication may already have succeeded; any
// other entry (including temp files) makes recovery inspection-only.
func inspectRecoveredOutputDirectory(path string) (exists bool, issue string, err error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false, "The matching output session path is not a real directory.", nil
	}
	directory, err := os.Open(path)
	if err != nil {
		return false, "", err
	}
	entries, readErr := directory.ReadDir(2)
	closeErr := directory.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return false, "", errors.Join(readErr, closeErr)
	}
	if closeErr != nil {
		return false, "", closeErr
	}
	if len(entries) == 0 {
		return false, "", nil
	}
	if len(entries) != 1 || entries[0].Name() != "photo.jpg" {
		return false, "The output folder contains an unexpected or temporary file. It is available for manual inspection only.", nil
	}
	photoPath := filepath.Join(path, "photo.jpg")
	if jpegErr := session.ValidateJPEGFile(photoPath); jpegErr != nil {
		return false, "A final output exists but is not a valid JPEG. It will not be replaced.", nil
	}
	return true, "", nil
}

// ContinueRecoveredPhotoSession requires a fresh read-only scan and an explicit
// operator action. The workflow re-reads and verifies the manifest once more.
func (a *App) ContinueRecoveredPhotoSession(sessionID string) (PlayerSession, error) {
	items, err := a.listInterruptedPhotoSessions()
	if err != nil {
		return PlayerSession{}, err
	}
	var selected *session.RecoveryItem
	for i := range items {
		if items[i].SessionID == sessionID {
			selected = &items[i]
			break
		}
	}
	if selected == nil || selected.Status != session.RecoveryResumeAvailable || selected.Manifest == nil {
		return PlayerSession{}, errors.New("this interrupted session is not eligible to continue; inspect it or start a new session")
	}
	if syntheticOnlyMode() && !selected.Manifest.DemoMode {
		return PlayerSession{}, errors.New("continuing a real-camera recovery is disabled for this synthetic-only run")
	}

	a.cameraLifecycleMu.Lock()
	defer a.cameraLifecycleMu.Unlock()
	if err := a.stopCameraPreviewLocked(); err != nil {
		return PlayerSession{}, fmt.Errorf("stop idle camera preview before session recovery: %w", err)
	}

	a.mu.Lock()
	if a.settingsError != "" && !recoveryCanIgnoreFrameSettingsError(a.settingsError, selected.Manifest) {
		message := a.settingsError
		a.mu.Unlock()
		return PlayerSession{}, fmt.Errorf("station settings need attention before continuing: %s", message)
	}
	if a.settingsSaving || a.playerRunning || a.outputRetrying || a.retentionBusy {
		a.mu.Unlock()
		return PlayerSession{}, errors.New("another station operation is active; finish it before recovering a session")
	}
	runConfig := a.playerRuntimeConfigLocked()
	manifest := selected.Manifest
	demo := manifest.DemoMode
	previews := make([]string, len(selected.CapturePaths))
	for i, path := range selected.CapturePaths {
		previews[i], _ = fileImageDataURL(path)
	}
	mode := "Windows webcam"
	message := "Reopening the camera; previously captured photos will be preserved…"
	if demo {
		mode = "Synthetic camera"
		message = "Continuing the interrupted demo session…"
	}
	a.player = PlayerSession{
		ID: sessionID, State: "reconnecting", Captured: selected.CompletedShots,
		Total: selected.ExpectedShots, Message: message, CameraMode: mode,
		DemoMode: demo, CapturePreviews: previews,
	}
	ctx, cancel := context.WithCancel(a.playerBaseContext())
	a.playerCancel, a.playerRunning = cancel, true
	snapshot := clonePlayerSession(a.player)
	a.mu.Unlock()
	a.emitPlayerSession(snapshot)

	go func() {
		defer cancel()
		a.runPhotoSession(ctx, sessionID, selected.CompletedShots, demo, true, runConfig)
	}()
	return snapshot, nil
}

// RestartRecoveredPhotoSession starts with a new ID and leaves the old folder
// untouched. The operator must explicitly select this action in the UI.
func (a *App) RestartRecoveredPhotoSession(sessionID string) (PlayerSession, error) {
	items, err := a.listInterruptedPhotoSessions()
	if err != nil {
		return PlayerSession{}, err
	}
	demo := false
	found := false
	for _, item := range items {
		if item.SessionID == sessionID {
			found = true
			if item.Manifest != nil {
				demo = item.Manifest.DemoMode
			}
			break
		}
	}
	if !found {
		return PlayerSession{}, errors.New("interrupted session was not found in the current recovery scan")
	}
	return a.startPhotoSession(demo)
}

// RetryRecoveredPhotoOutput reuses only the preserved rendered JPEG. It never
// calls the camera or renderer and relies on the exclusive output publisher.
func (a *App) RetryRecoveredPhotoOutput(sessionID string) (PlayerSession, error) {
	// A recovered output retry must be serialized with the idle preview/camera
	// lifecycle just like capture recovery. The output operation owns the UI
	// session/result view and must not leave a hidden preview worker running.
	a.cameraLifecycleMu.Lock()
	defer a.cameraLifecycleMu.Unlock()
	if err := a.stopCameraPreviewLocked(); err != nil {
		return PlayerSession{}, fmt.Errorf("stop idle camera preview before recovered output retry: %w", err)
	}

	items, err := a.listInterruptedPhotoSessions()
	if err != nil {
		return PlayerSession{}, err
	}
	var selected *session.RecoveryItem
	for i := range items {
		if items[i].SessionID == sessionID {
			selected = &items[i]
			break
		}
	}
	if selected == nil || selected.Status != session.RecoveryOutputPending || selected.Manifest == nil {
		return PlayerSession{}, errors.New("this interrupted session does not have a verified rendered photo awaiting local output")
	}
	// Discovery intentionally performs header-only checks for quick startup.
	// Before publishing an output, fully decode its exact preserved source.
	if err := session.ValidateJPEGFile(selected.RenderedPath); err != nil {
		return PlayerSession{}, fmt.Errorf("validate preserved rendered JPEG before output retry: %w", err)
	}
	return a.retryPhotoOutput(sessionID, selected)
}

// OpenPhotoSessionFolder is an explicit, read-only inspection/export affordance.
func (a *App) OpenPhotoSessionFolder(sessionID string) error {
	if runtime.GOOS != "windows" {
		return errors.New("opening a session folder in Explorer is available only in the Windows Preview")
	}
	if !isPlayerSessionID(sessionID) {
		return errors.New("invalid session ID")
	}
	if err := a.preparePlayerPaths(); err != nil {
		return err
	}
	a.mu.Lock()
	workDir := a.workDir
	busy := a.playerRunning || a.outputRetrying || a.settingsSaving || a.retentionBusy
	a.mu.Unlock()
	if busy {
		return errors.New("wait for the current station operation to finish before inspecting session files")
	}
	sessionDir := filepath.Join(workDir, sessionID)
	if err := stationconfig.ValidateStorageChild(workDir, sessionDir); err != nil {
		return fmt.Errorf("validate session folder identity: %w", err)
	}
	info, err := os.Lstat(sessionDir)
	if err != nil {
		return fmt.Errorf("inspect session folder: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("session path is not a real directory")
	}
	return exec.Command("explorer.exe", sessionDir).Start()
}
