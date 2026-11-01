package main

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dongsinhho/PhotoBooth/internal/booth/camera"
	fakecamera "github.com/dongsinhho/PhotoBooth/internal/booth/camera/fake"
	"github.com/dongsinhho/PhotoBooth/internal/booth/camera/recovery"
	"github.com/dongsinhho/PhotoBooth/internal/booth/output"
	"github.com/dongsinhho/PhotoBooth/internal/booth/render"
	"github.com/dongsinhho/PhotoBooth/internal/booth/session"
	"github.com/dongsinhho/PhotoBooth/internal/booth/workflow"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed assets/photo-strip-v1.json
var embeddedSampleFrame []byte

type PlayerSession struct {
	ID               string   `json:"id"`
	OutputRetrying   bool     `json:"output_retrying,omitempty"`
	State            string   `json:"state"`
	Captured         int      `json:"captured"`
	Total            int      `json:"total"`
	CurrentShot      int      `json:"current_shot"`
	CountdownSeconds int      `json:"countdown_seconds"`
	Message          string   `json:"message"`
	ErrorMessage     string   `json:"error_message,omitempty"`
	CameraMode       string   `json:"camera_mode"`
	OutputPath       string   `json:"output_path,omitempty"`
	RenderedPath     string   `json:"rendered_path,omitempty"`
	PreviewDataURL   string   `json:"preview_data_url,omitempty"`
	CapturePreviews  []string `json:"capture_previews,omitempty"`
	DemoMode         bool     `json:"demo_mode"`
}

const (
	playerEventName = "photo-session"
	playerShotCount = 3
	playerWidth     = 1280
	playerHeight    = 720
)

// StartPhotoSession starts real Windows webcam capture, rendering and local output.
// It returns immediately; the UI receives progress via the "photo-session" event.
func (a *App) StartPhotoSession() (PlayerSession, error) {
	if runtime.GOOS != "windows" {
		return PlayerSession{}, errors.New("real webcam capture in this UI build is currently supported on Windows only")
	}
	return a.startPhotoSession(false)
}

// StartDemoPhotoSession runs the same workflow with generated images, without a webcam.
func (a *App) StartDemoPhotoSession() (PlayerSession, error) {
	return a.startPhotoSession(true)
}

func (a *App) startPhotoSession(demo bool) (PlayerSession, error) {
	a.cameraLifecycleMu.Lock()
	defer a.cameraLifecycleMu.Unlock()
	if err := a.stopCameraPreviewLocked(); err != nil {
		return PlayerSession{}, fmt.Errorf("stop live camera preview before capture: %w", err)
	}
	if err := a.preparePlayerPaths(); err != nil {
		return PlayerSession{}, err
	}
	a.mu.Lock()
	if a.settingsError != "" {
		message := a.settingsError
		a.mu.Unlock()
		return PlayerSession{}, fmt.Errorf("station settings need attention before capture: %s", message)
	}
	if a.settingsSaving {
		a.mu.Unlock()
		return PlayerSession{}, errors.New("station settings are being saved; try again in a moment")
	}
	if a.playerRunning {
		a.mu.Unlock()
		return PlayerSession{}, errors.New("a photo session is already running")
	}
	if a.outputRetrying {
		a.mu.Unlock()
		return PlayerSession{}, errors.New("wait for the local output retry to finish before starting another session")
	}
	id, err := newPlayerSessionID()
	if err != nil {
		a.mu.Unlock()
		return PlayerSession{}, err
	}
	mode, message := "Windows webcam", "Preparing camera and frame…"
	if demo {
		mode, message = "Synthetic camera", "Preparing a demo capture…"
	}
	a.player = PlayerSession{ID: id, State: "preflight", Total: a.playerShotCount, Message: message, CameraMode: mode, DemoMode: demo}
	ctx, cancel := context.WithCancel(a.playerBaseContext())
	a.playerCancel, a.playerRunning = cancel, true
	snapshot := clonePlayerSession(a.player)
	a.mu.Unlock()
	a.emitPlayerSession(snapshot)
	go func() {
		defer cancel()
		a.runPhotoSession(ctx, id, 0, demo)
	}()
	return snapshot, nil
}

// ContinuePhotoSession preserves earlier shots and retries only the unfinished ones.
func (a *App) ContinuePhotoSession(sessionID string) (PlayerSession, error) {
	a.mu.Lock()
	if a.outputRetrying {
		a.mu.Unlock()
		return PlayerSession{}, errors.New("wait for the local output retry to finish")
	}
	if a.playerRunning {
		a.mu.Unlock()
		return PlayerSession{}, errors.New("the current session is still running")
	}
	if sessionID == "" || sessionID != a.player.ID {
		a.mu.Unlock()
		return PlayerSession{}, errors.New("session ID does not match the current photo session")
	}
	if a.player.State != string(session.Failed) && a.player.State != string(session.TimedOut) {
		a.mu.Unlock()
		return PlayerSession{}, fmt.Errorf("cannot continue a session in state %q", a.player.State)
	}
	if a.player.Captured >= a.player.Total {
		a.mu.Unlock()
		return PlayerSession{}, errors.New("all photos have already been captured; start a new session")
	}
	resumeCount, demo := a.player.Captured, a.player.DemoMode
	a.player.State = "reconnecting"
	a.player.Message = "Reopening the camera; completed photos will be kept…"
	a.player.ErrorMessage = ""
	ctx, cancel := context.WithCancel(a.playerBaseContext())
	a.playerCancel, a.playerRunning = cancel, true
	snapshot := clonePlayerSession(a.player)
	a.mu.Unlock()
	a.emitPlayerSession(snapshot)
	go func() {
		defer cancel()
		a.runPhotoSession(ctx, sessionID, resumeCount, demo)
	}()
	return snapshot, nil
}

// RestartPhotoSession creates a clean session with a new ID after a failure.
func (a *App) RestartPhotoSession(sessionID string) (PlayerSession, error) {
	a.mu.Lock()
	if a.outputRetrying {
		a.mu.Unlock()
		return PlayerSession{}, errors.New("wait for the local output retry to finish")
	}
	if a.playerRunning {
		a.mu.Unlock()
		return PlayerSession{}, errors.New("the current session is still running")
	}
	if sessionID == "" || sessionID != a.player.ID {
		a.mu.Unlock()
		return PlayerSession{}, errors.New("session ID does not match the current photo session")
	}
	if a.player.State != string(session.Failed) && a.player.State != string(session.TimedOut) {
		a.mu.Unlock()
		return PlayerSession{}, fmt.Errorf("cannot restart a session in state %q", a.player.State)
	}
	demo := a.player.DemoMode
	a.mu.Unlock()
	return a.startPhotoSession(demo)
}

func (a *App) CancelPhotoSession(sessionID string) error {
	a.mu.Lock()
	if sessionID == "" || sessionID != a.player.ID {
		a.mu.Unlock()
		return errors.New("session ID does not match the current photo session")
	}
	if !a.playerRunning || a.playerCancel == nil {
		a.mu.Unlock()
		return errors.New("there is no running photo session to cancel")
	}
	cancel := a.playerCancel
	a.player.State, a.player.Message = "cancelling", "Cancelling the session and closing the camera…"
	snapshot := clonePlayerSession(a.player)
	a.mu.Unlock()
	a.emitPlayerSession(snapshot)
	cancel()
	return nil
}

func (a *App) DismissPhotoSession(sessionID string) (PlayerSession, error) {
	a.mu.Lock()
	if a.settingsSaving {
		a.mu.Unlock()
		return PlayerSession{}, errors.New("wait for settings to finish saving before returning to the start screen")
	}
	if a.playerRunning {
		a.mu.Unlock()
		return PlayerSession{}, errors.New("cancel the running session before returning to the start screen")
	}
	if a.outputRetrying {
		a.mu.Unlock()
		return PlayerSession{}, errors.New("wait for the local output retry to finish before returning to the start screen")
	}
	if a.player.ID != "" && sessionID != a.player.ID {
		a.mu.Unlock()
		return PlayerSession{}, errors.New("session ID does not match the current photo session")
	}
	a.player = PlayerSession{State: "idle", Total: a.playerShotCount, Message: "Ready to take photos."}
	snapshot := clonePlayerSession(a.player)
	a.mu.Unlock()
	a.emitPlayerSession(snapshot)
	return snapshot, nil
}

func (a *App) CurrentPhotoSession() PlayerSession {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.player.State == "idle" || a.player.Total == 0 {
		a.player.Total = a.playerShotCount
	}
	if a.player.State == "" {
		a.player.State = "idle"
	}
	return clonePlayerSession(a.player)
}

// RetryPhotoOutput republishes a preserved rendered session without opening
// the camera or rerunning the renderer. It is only available after all shots
// and the rendered image are present but the local output is still missing.
func (a *App) RetryPhotoOutput(sessionID string) (PlayerSession, error) {
	if err := a.preparePlayerPaths(); err != nil {
		return PlayerSession{}, err
	}

	a.mu.Lock()
	if a.playerRunning {
		a.mu.Unlock()
		return PlayerSession{}, errors.New("cannot retry local output while a photo session is running")
	}
	if a.outputRetrying {
		a.mu.Unlock()
		return PlayerSession{}, errors.New("a local output retry is already running")
	}
	if a.settingsSaving {
		a.mu.Unlock()
		return PlayerSession{}, errors.New("wait for settings/package operations to finish before retrying local output")
	}
	if sessionID == "" || sessionID != a.player.ID || !isPlayerSessionID(sessionID) {
		a.mu.Unlock()
		return PlayerSession{}, errors.New("session ID does not match the current photo session")
	}
	if a.player.State == string(session.Completed) || a.player.OutputPath != "" {
		a.mu.Unlock()
		return PlayerSession{}, errors.New("this session already has a finalized local output")
	}
	if a.player.State != string(session.Failed) && a.player.State != string(session.PartialSuccess) && a.player.State != string(session.TimedOut) {
		state := a.player.State
		a.mu.Unlock()
		return PlayerSession{}, fmt.Errorf("cannot retry local output for session state %q", state)
	}
	if a.player.Total < 1 || a.player.Captured != a.player.Total {
		captured, total := a.player.Captured, a.player.Total
		a.mu.Unlock()
		return PlayerSession{}, fmt.Errorf("local output retry requires all photos to be captured (captured %d of %d)", captured, total)
	}
	expectedRendered := filepath.Join(a.workDir, sessionID, "rendered.jpg")
	if strings.TrimSpace(a.workDir) == "" || strings.TrimSpace(a.player.RenderedPath) == "" ||
		!sameLocalPath(a.player.RenderedPath, expectedRendered) {
		a.mu.Unlock()
		return PlayerSession{}, errors.New("the preserved rendered image for this session is unavailable")
	}

	a.outputRetrying = true
	a.player.OutputRetrying = true
	a.player.Message = "Retrying the local save from the preserved rendered image…"
	a.player.ErrorMessage = ""
	sessionsRoot, outputRoot, captureCount := a.workDir, a.outputDir, a.player.Total
	ctx, cancel := context.WithTimeout(a.playerBaseContext(), 2*time.Minute)
	snapshot := clonePlayerSession(a.player)
	a.mu.Unlock()
	a.emitPlayerSession(snapshot)

	savedPath, retryErr := output.RetryLocalFromSession(ctx, sessionsRoot, outputRoot, sessionID, captureCount)
	cancel()

	a.mu.Lock()
	a.outputRetrying = false
	a.player.OutputRetrying = false
	if a.player.ID != sessionID {
		a.mu.Unlock()
		if retryErr != nil {
			return PlayerSession{}, retryErr
		}
		return PlayerSession{}, errors.New("the active photo session changed while local output was being retried")
	}
	if retryErr != nil {
		a.player.ErrorMessage = retryErr.Error()
		a.player.Message = "All photos were captured and rendered, but the local save could not be completed. Resolve the storage/output issue, then retry the local save; no new photos are needed."
	} else {
		a.player.State = string(session.Completed)
		a.player.OutputPath = savedPath
		a.player.ErrorMessage = ""
		a.player.Message = fmt.Sprintf("All %d photos were captured, rendered, and saved locally.", captureCount)
	}
	snapshot = clonePlayerSession(a.player)
	a.mu.Unlock()
	a.emitPlayerSession(snapshot)
	if retryErr != nil {
		return snapshot, retryErr
	}
	return snapshot, nil
}

// OpenPhotoOutputFolder opens Explorer on the finalized image for this session.
func (a *App) OpenPhotoOutputFolder(sessionID string) error {
	if runtime.GOOS != "windows" {
		return errors.New("opening a Windows output folder is only supported on Windows")
	}
	if !isPlayerSessionID(sessionID) {
		return errors.New("invalid session ID")
	}
	if err := a.preparePlayerPaths(); err != nil {
		return err
	}
	path := filepath.Join(a.outputDir, sessionID, "photo.jpg")
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("find saved photo: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("saved photo path is not a regular file")
	}
	return exec.Command("explorer.exe", "/select,", path).Start()
}

func (a *App) runPhotoSession(ctx context.Context, sessionID string, resumeCount int, demo bool) {
	var provider camera.Provider
	if demo {
		provider = fakecamera.New()
	} else {
		config := a.stationConfig
		provider = newConfiguredCameraProvider(config, func(event recovery.Event) {
			a.handleRecoveryEvent(sessionID, event)
		})
	}
	runner := &workflow.Runner{
		Camera: provider, Renderer: render.New(), Output: output.NewLocal(a.outputDir),
		OnEvent: func(event workflow.Event) { a.handleWorkflowEvent(sessionID, event) },
	}
	result, runErr := runner.Run(ctx, workflow.Config{
		FrameConfigPath: a.framePath, WorkDir: a.workDir, CaptureCount: a.playerShotCount,
		CaptureWidth: a.stationConfig.Camera.Width, CaptureHeight: a.stationConfig.Camera.Height,
		SessionID: sessionID, ResumeCaptureCount: resumeCount,
		Countdown: a.playerCountdown, Interval: a.playerInterval,
	})
	previews := make([]string, len(result.CapturePaths))
	for i, path := range result.CapturePaths {
		previews[i], _ = fileImageDataURL(path)
	}
	finalPreview := ""
	if result.RenderedPath != "" {
		finalPreview, _ = fileImageDataURL(result.RenderedPath)
	}

	a.mu.Lock()
	if a.player.ID != sessionID {
		a.mu.Unlock()
		return
	}
	a.playerRunning, a.playerCancel = false, nil
	a.player.Captured, a.player.CurrentShot = len(result.CapturePaths), len(result.CapturePaths)
	a.player.CountdownSeconds = 0
	a.player.CapturePreviews = previews
	a.player.RenderedPath, a.player.OutputPath, a.player.PreviewDataURL = result.RenderedPath, result.OutputPath, finalPreview
	if runErr != nil {
		state := result.State
		if state == "" || state == session.Idle || state == session.Preflight || state == session.Capturing {
			state = session.Failed
		}
		a.player.State = string(state)
		a.player.ErrorMessage = runErr.Error()
		a.player.Message = friendlyPlayerError(runErr, demo)
	} else {
		a.player.State = string(result.State)
		a.player.ErrorMessage = ""
		if result.State == session.PartialSuccess {
			a.player.Message = "The photo strip was saved, but one or more output targets did not finish."
		} else {
			a.player.Message = fmt.Sprintf("All %d photos were captured, rendered, and saved locally.", a.playerShotCount)
		}
	}
	snapshot := clonePlayerSession(a.player)
	a.mu.Unlock()
	a.emitPlayerSession(snapshot)
}

func (a *App) handleWorkflowEvent(sessionID string, event workflow.Event) {
	preview := ""
	if event.Type == "captured" && event.Path != "" {
		preview, _ = fileImageDataURL(event.Path)
	}
	a.mu.Lock()
	if a.player.ID != sessionID || !a.playerRunning {
		a.mu.Unlock()
		return
	}
	switch event.Type {
	case "state":
		// The runner can emit a terminal domain state before its caller has
		// loaded the final preview/output metadata. Keep the UI in its current
		// non-terminal state until runPhotoSession publishes the final snapshot.
		if event.State.Terminal() {
			a.player.Message = "Finalizing photo previews and local output…"
		} else {
			a.player.State, a.player.Message = string(event.State), event.Message
		}
	case "countdown":
		a.player.State, a.player.CurrentShot = string(event.State), event.Shot
		a.player.CountdownSeconds = event.Seconds
		if event.Seconds > 0 {
			a.player.Message = fmt.Sprintf("Shot %d of %d in %d…", event.Shot, event.Total, event.Seconds)
		} else {
			a.player.Message = fmt.Sprintf("Capturing shot %d of %d…", event.Shot, event.Total)
		}
	case "captured":
		a.player.Captured = max(a.player.Captured, event.Shot)
		a.player.CurrentShot, a.player.CountdownSeconds = event.Shot, 0
		a.player.Message = fmt.Sprintf("Captured shot %d of %d.", event.Shot, event.Total)
		if len(a.player.CapturePreviews) <= event.Shot-1 {
			extended := make([]string, event.Shot)
			copy(extended, a.player.CapturePreviews)
			a.player.CapturePreviews = extended
		}
		if event.Shot > 0 && event.Shot <= len(a.player.CapturePreviews) {
			a.player.CapturePreviews[event.Shot-1] = preview
		}
	case "rendered":
		a.player.State, a.player.RenderedPath = string(event.State), event.Path
		a.player.Message = "Rendering the photo strip…"
	case "output_target":
		a.player.State = string(event.State)
		if event.Status == string(output.TargetSucceeded) && event.Path != "" {
			a.player.OutputPath = event.Path
		}
	case "failed", "completed", "partial_success":
		a.mu.Unlock()
		return
	}
	snapshot := clonePlayerSession(a.player)
	a.mu.Unlock()
	a.emitPlayerSession(snapshot)
}

func (a *App) handleRecoveryEvent(sessionID string, event recovery.Event) {
	a.mu.Lock()
	if a.player.ID != sessionID || !a.playerRunning {
		a.mu.Unlock()
		return
	}
	switch event.Type {
	case "recovery_started":
		a.player.State, a.player.Message = "reconnecting", "The camera disconnected. Reconnect it now; PhotoBooth is trying automatically."
		a.player.ErrorMessage = ""
	case "recovery_open_attempt_failed":
		a.player.State = "reconnecting"
		a.player.Message = fmt.Sprintf("Waiting for the camera to return (attempt %d of %d)…", event.Attempt, event.Total)
	case "recovery_provider_opened":
		a.player.State, a.player.Message = "capturing", "Camera reconnected. Resuming the current capture…"
	case "recovery_succeeded":
		a.player.State, a.player.Message = "capturing", "Camera recovery succeeded."
	case "recovery_failed":
		if event.Err != nil {
			a.player.ErrorMessage = event.Err.Error()
		}
		a.player.Message = "Camera recovery timed out. Reconnect the webcam, then choose Continue to retry the remaining shots."
	default:
		a.mu.Unlock()
		return
	}
	snapshot := clonePlayerSession(a.player)
	a.mu.Unlock()
	a.emitPlayerSession(snapshot)
}

func (a *App) preparePlayerPaths() error {
	a.pathsMu.Lock()
	defer a.pathsMu.Unlock()
	if a.settingsPath != "" && a.framePath != "" && a.workDir != "" && a.outputDir != "" {
		return nil
	}
	// Existing workflow tests inject a complete set of paths; keep those tests
	// isolated from the user's real station profile.
	if a.settingsPath == "" && a.playerDataDir == "" && a.framePath != "" && a.workDir != "" && a.outputDir != "" {
		return nil
	}
	base := strings.TrimSpace(a.playerDataDir)
	if base == "" {
		// Explicitly scoped local data roots support portable setups and isolated
		// UI smoke tests. With no override, use the normal per-user cache folder.
		base = strings.TrimSpace(os.Getenv("PHOTOBOOTH_DATA_DIR"))
	}
	if base == "" {
		cacheDir, err := os.UserCacheDir()
		if err != nil {
			return fmt.Errorf("resolve local application data directory: %w", err)
		}
		base = filepath.Join(cacheDir, "PhotoBooth")
	}
	absoluteBase, err := filepath.Abs(base)
	if err != nil {
		return fmt.Errorf("resolve PhotoBooth data directory: %w", err)
	}
	if err := os.MkdirAll(absoluteBase, 0o700); err != nil {
		return fmt.Errorf("prepare PhotoBooth data directory: %w", err)
	}
	if err := a.loadStationConfig(absoluteBase); err != nil {
		return err
	}
	return nil
}

func (a *App) playerBaseContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func (a *App) emitPlayerSession(snapshot PlayerSession) {
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, playerEventName, snapshot)
	}
}

func clonePlayerSession(source PlayerSession) PlayerSession {
	source.CapturePreviews = append([]string(nil), source.CapturePreviews...)
	return source
}

func fileImageDataURL(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 32<<20 {
		return "", errors.New("image preview is unavailable or exceeds the 32 MiB preview limit")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data), nil
}

func newPlayerSessionID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", fmt.Errorf("generate photo session ID: %w", err)
	}
	return hex.EncodeToString(id[:]), nil
}

func friendlyPlayerError(err error, demo bool) string {
	if demo {
		return "The demo capture could not finish. You can retry or start a fresh demo session. " + err.Error()
	}
	if errors.Is(err, output.ErrInsufficientStorage) || strings.Contains(strings.ToLower(err.Error()), "no output target succeeded") {
		return "All photos were captured and the strip was rendered, but saving the final local photo failed. Free disk space or resolve the storage issue, then retry the local save without taking the photos again. " + err.Error()
	}
	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "no camera devices"), strings.Contains(lower, "device not found"):
		return "PhotoBooth could not find a webcam. Connect a supported camera, close other camera apps, then choose Continue to retry. " + err.Error()
	case strings.Contains(lower, "camera stream unavailable"), strings.Contains(lower, "invalidated"):
		return "The camera connection was lost. Reconnect it, then choose Continue to capture the remaining photos. " + err.Error()
	default:
		return "The photo session failed. Already captured photos are kept. Fix the issue and choose Continue, or start a fresh session. " + err.Error()
	}
}

func isPlayerSessionID(value string) bool {
	if len(value) != 32 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
