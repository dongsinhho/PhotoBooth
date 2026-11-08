package workflow

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dongsinhho/PhotoBooth/internal/booth/camera"
	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
	"github.com/dongsinhho/PhotoBooth/internal/booth/output"
	"github.com/dongsinhho/PhotoBooth/internal/booth/payment"
	"github.com/dongsinhho/PhotoBooth/internal/booth/session"
)

type Renderer interface {
	SlotCount(ctx context.Context, configPath string) (int, error)
	Render(ctx context.Context, configPath string, inputPaths []string, outputPath string) error
}

type Config struct {
	FrameConfigPath         string
	FrameConfigIdentityPath string // Original configured path when rendering from a durable session snapshot.
	WorkDir                 string
	OutputDir               string // Optional when the dispatcher does not expose a local root.
	CaptureCount            int
	Countdown               time.Duration
	Interval                time.Duration
	CaptureWidth            int
	CaptureHeight           int
	ConfigSchemaVersion     int
	CameraProvider          string
	CameraDevice            string
	CameraFPS               float64
	MediaDeliveryMode       string
	PaymentMode             string
	DemoMode                bool
	SessionID               string // Optional. Generated automatically when empty.
	// ResumeSession permits reusing an existing, verified session workspace,
	// including the case where the previous attempt failed before the first
	// complete capture. It always requires an explicit SessionID.
	ResumeSession bool
	// ResumeCaptureCount reuses validated capture-01.jpg..capture-N.jpg from
	// the same session directory and captures only the remaining shots.
	ResumeCaptureCount int
}

type Event struct {
	Type      string
	State     session.State
	SessionID string
	Shot      int
	Total     int
	Seconds   int
	Path      string
	Target    string
	Status    string
	JobID     string
	Message   string
	Duration  time.Duration
}

type Result struct {
	SessionID        string
	State            session.State
	CapturePaths     []string
	RenderedPath     string
	OutputPath       string
	OutputResults    []output.TargetResult
	CaptureDurations []time.Duration
	RenderDuration   time.Duration
	OutputDuration   time.Duration
	TotalDuration    time.Duration
	StartedAt        time.Time
	FinishedAt       time.Time
}

type Runner struct {
	Camera      camera.Provider
	Renderer    Renderer
	Output      output.Dispatcher
	PaymentGate payment.Gate
	// OnEvent runs on the session goroutine. Handlers must return quickly and must
	// not block on UI rendering, network or disk; UI adapters should enqueue events.
	OnEvent func(Event)
	// Sleep can be injected by tests to exercise countdown/interval paths instantly.
	Sleep func(context.Context, time.Duration) error
	// writeManifest and publishCapture are narrow persistence seams for
	// failure-injection tests around crash-consistency boundaries.
	writeManifest  func(string, session.Manifest) error
	publishCapture func(string, string) error
}

func (r *Runner) Run(ctx context.Context, cfg Config) (result Result, runErr error) {
	result.StartedAt = time.Now()
	result.State = session.Idle
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			result.State = session.TimedOut
		} else {
			result.State = session.Cancelled
		}
		result.FinishedAt = time.Now()
		return result, err
	}
	if r == nil || r.Renderer == nil || r.Output == nil {
		return result, errors.New("renderer and output providers are required")
	}
	fullCaptureResume := cfg.ResumeSession && cfg.ResumeCaptureCount == cfg.CaptureCount
	if r.Camera == nil && !fullCaptureResume {
		return result, errors.New("camera provider is required unless every capture is already preserved for recovery")
	}
	if cfg.CaptureCount < 1 || cfg.CaptureCount > 12 {
		return result, errors.New("capture count must be between 1 and 12")
	}
	if cfg.ResumeCaptureCount < 0 || cfg.ResumeCaptureCount > cfg.CaptureCount ||
		(cfg.ResumeCaptureCount == cfg.CaptureCount && !cfg.ResumeSession) {
		return result, errors.New("resume capture count must be between zero and capture count, and a full-count resume must be explicit")
	}
	if (cfg.ResumeSession || cfg.ResumeCaptureCount > 0) && cfg.SessionID == "" {
		return result, errors.New("an explicit session ID is required when resuming a session")
	}
	if strings.TrimSpace(cfg.WorkDir) == "" {
		return result, errors.New("work directory is required")
	}
	if strings.TrimSpace(cfg.FrameConfigPath) == "" {
		return result, errors.New("frame config path is required")
	}
	if cfg.Countdown < 0 || cfg.Interval < 0 {
		return result, errors.New("countdown and interval cannot be negative")
	}
	if cfg.SessionID == "" {
		var err error
		cfg.SessionID, err = newSessionID()
		if err != nil {
			return result, fmt.Errorf("generate session ID: %w", err)
		}
	}
	if !validSessionID(cfg.SessionID) {
		return result, errors.New("session ID must be 32 lowercase hexadecimal characters")
	}
	result.SessionID = cfg.SessionID

	if err := r.transition(&result, session.Preflight, "validate frame configuration and prepare local session"); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return r.fail(result, err)
	}
	if err := stationconfig.ValidateLocalStoragePath(cfg.WorkDir); err != nil {
		return r.fail(result, fmt.Errorf("validate local sessions root: %w", err))
	}
	if strings.TrimSpace(cfg.OutputDir) != "" {
		if err := stationconfig.ValidateLocalStoragePath(cfg.OutputDir); err != nil {
			return r.fail(result, fmt.Errorf("validate local output root: %w", err))
		}
		if err := stationconfig.ValidateDistinctStorageRoots(cfg.WorkDir, cfg.OutputDir); err != nil {
			return r.fail(result, fmt.Errorf("validate isolated session/output roots: %w", err))
		}
	}
	if err := os.MkdirAll(cfg.WorkDir, 0o700); err != nil {
		return r.fail(result, fmt.Errorf("prepare sessions root: %w", err))
	}
	sessionDir := filepath.Join(cfg.WorkDir, cfg.SessionID)
	captureDir := filepath.Join(sessionDir, "captures")
	if err := stationconfig.ValidateStorageChild(cfg.WorkDir, sessionDir); err != nil {
		return r.fail(result, fmt.Errorf("validate session workspace identity: %w", err))
	}
	if err := stationconfig.ValidateStorageChild(sessionDir, captureDir); err != nil {
		return r.fail(result, fmt.Errorf("validate capture directory identity: %w", err))
	}
	resumeExisting := cfg.ResumeSession || cfg.ResumeCaptureCount > 0
	var manifest session.Manifest
	var err error
	if !resumeExisting {
		frameBytes, frameErr := readFrameConfigBytes(cfg.FrameConfigPath)
		if frameErr != nil {
			return r.fail(result, fmt.Errorf("read frame config before creating session: %w", frameErr))
		}
		// Preserve the old preflight guarantee: invalid frame definitions do not
		// claim a session workspace. Revalidate the exact snapshot after publish.
		sourceSlots, slotErr := r.Renderer.SlotCount(ctx, cfg.FrameConfigPath)
		if slotErr != nil {
			return r.fail(result, fmt.Errorf("validate frame config before creating session: %w", slotErr))
		}
		if sourceSlots != cfg.CaptureCount {
			return r.fail(result, fmt.Errorf("frame defines %d photo slots, but session is configured for %d captures", sourceSlots, cfg.CaptureCount))
		}
		manifest, err = NewSessionManifestFromFrameBytes(cfg, frameBytes)
		if err != nil {
			return r.fail(result, fmt.Errorf("prepare durable session manifest: %w", err))
		}
		if err := r.authorizePayment(ctx, cfg); err != nil {
			return r.fail(result, fmt.Errorf("payment authorization failed before capture: %w", err))
		}
		// New sessions must claim a previously unused directory atomically.
		// MkdirAll would silently reuse a stale/colliding session ID.
		if err := os.Mkdir(sessionDir, 0o700); err != nil {
			if errors.Is(err, os.ErrExist) {
				return r.fail(result, fmt.Errorf("session workspace %q already exists; refusing to reuse or overwrite it", sessionDir))
			}
			return r.fail(result, fmt.Errorf("create new session workspace: %w", err))
		}
		if err := os.Mkdir(captureDir, 0o700); err != nil {
			return r.fail(result, fmt.Errorf("create new session capture directory: %w", err))
		}
		snapshotPath, snapshotHash, snapshotErr := session.WriteFrameSnapshot(sessionDir, frameBytes)
		if snapshotErr != nil {
			return r.fail(result, fmt.Errorf("persist immutable frame snapshot: %w", snapshotErr))
		}
		manifest.FrameSnapshotFile = session.FrameSnapshotFileName
		manifest.FrameSnapshotSHA256 = snapshotHash
		cfg.FrameConfigPath = snapshotPath
		slots, slotErr := r.Renderer.SlotCount(ctx, cfg.FrameConfigPath)
		if slotErr != nil {
			return r.fail(result, fmt.Errorf("validate snapshotted frame config: %w", slotErr))
		}
		if slots != cfg.CaptureCount {
			return r.fail(result, fmt.Errorf("frame defines %d photo slots, but session is configured for %d captures", slots, cfg.CaptureCount))
		}
		if err := r.persistManifest(sessionDir, manifest); err != nil {
			return r.fail(result, fmt.Errorf("persist initial session manifest: %w", err))
		}
	} else {
		if err := requireSessionDirectory(sessionDir, "session workspace"); err != nil {
			return r.fail(result, err)
		}
		sessionEntries, readErr := os.ReadDir(sessionDir)
		if readErr != nil {
			return r.fail(result, fmt.Errorf("inspect resume session workspace: %w", readErr))
		}
		persisted, manifestErr := session.ReadManifest(sessionDir)
		if manifestErr != nil {
			return r.fail(result, fmt.Errorf("inspect durable resume manifest: %w", manifestErr))
		}
		allowedEntries := map[string]bool{"captures": true, session.ManifestFileName: true}
		if persisted.FrameSnapshotFile != "" {
			allowedEntries[session.FrameSnapshotFileName] = true
		}
		if len(sessionEntries) != len(allowedEntries) {
			return r.fail(result, errors.New("resume session workspace contains missing or unexpected durable artifacts; refusing to continue"))
		}
		for _, entry := range sessionEntries {
			if !allowedEntries[entry.Name()] {
				return r.fail(result, fmt.Errorf("resume session workspace contains unexpected artifact %q", entry.Name()))
			}
		}
		var frameBytes []byte
		if persisted.FrameSnapshotFile != "" {
			snapshotPath, snapshotBytes, snapshotErr := session.ReadFrameSnapshot(sessionDir, persisted)
			if snapshotErr != nil {
				return r.fail(result, fmt.Errorf("validate durable frame snapshot: %w", snapshotErr))
			}
			cfg.FrameConfigPath = snapshotPath
			cfg.FrameConfigIdentityPath = persisted.FrameConfigPath
			frameBytes = snapshotBytes
		} else {
			frameBytes, err = readFrameConfigBytes(cfg.FrameConfigPath)
			if err != nil {
				return r.fail(result, fmt.Errorf("read legacy session frame config: %w", err))
			}
		}
		manifest, err = NewSessionManifestFromFrameBytes(cfg, frameBytes)
		if err != nil {
			return r.fail(result, fmt.Errorf("rebuild resume configuration fingerprint: %w", err))
		}
		if persisted.SessionID != manifest.SessionID ||
			persisted.ExpectedShots != manifest.ExpectedShots ||
			persisted.FrameConfigSHA256 != manifest.FrameConfigSHA256 ||
			persisted.RunConfigFingerprint != manifest.RunConfigFingerprint ||
			persisted.ConfigSchemaVersion != manifest.ConfigSchemaVersion ||
			persisted.FrameConfigPath != manifest.FrameConfigPath {
			return r.fail(result, errors.New("saved frame or capture configuration differs from the interrupted session; refusing to resume"))
		}
		if err := r.authorizePayment(ctx, cfg); err != nil {
			return r.fail(result, fmt.Errorf("payment authorization failed before capture resume: %w", err))
		}
		slots, slotErr := r.Renderer.SlotCount(ctx, cfg.FrameConfigPath)
		if slotErr != nil || slots != cfg.CaptureCount {
			if slotErr != nil {
				return r.fail(result, fmt.Errorf("validate snapshotted resume frame config: %w", slotErr))
			}
			return r.fail(result, fmt.Errorf("frame defines %d photo slots, but session is configured for %d captures", slots, cfg.CaptureCount))
		}
		if persisted.Phase != session.PhasePreflight && persisted.Phase != session.PhaseCapturing {
			return r.fail(result, fmt.Errorf("session phase %q is not eligible for capture resume", persisted.Phase))
		}
		manifest = persisted
		if err := requireSessionDirectory(captureDir, "capture directory"); err != nil {
			return r.fail(result, err)
		}
		entries, readErr := os.ReadDir(captureDir)
		if readErr != nil {
			return r.fail(result, fmt.Errorf("inspect existing captures: %w", readErr))
		}
		committedEntries := 0
		for _, entry := range entries {
			if entry.Name() == session.CaptureStagingDirectoryName {
				continue
			}
			committedEntries++
			if entry.Name() != fmt.Sprintf("capture-%02d.jpg", committedEntries) {
				return r.fail(result, fmt.Errorf("resume workspace contains unexpected capture entry %q", entry.Name()))
			}
		}
		if committedEntries != cfg.ResumeCaptureCount {
			return r.fail(result, fmt.Errorf("resume workspace contains %d committed captures; expected exactly %d", committedEntries, cfg.ResumeCaptureCount))
		}
		if _, err := session.ValidateCaptureStagingDirectory(captureDir); err != nil {
			return r.fail(result, fmt.Errorf("inspect interrupted capture staging: %w", err))
		}
		if persisted.CompletedShots != cfg.ResumeCaptureCount {
			// A complete capture may have been atomically published immediately
			// before a crash prevented its manifest progress update. Reconcile
			// exactly one such capture only after full JPEG validation below.
			if persisted.Phase != session.PhaseCapturing || persisted.CompletedShots+1 != cfg.ResumeCaptureCount {
				return r.fail(result, fmt.Errorf("manifest records %d completed captures but the requested resume count is %d", persisted.CompletedShots, cfg.ResumeCaptureCount))
			}
		}
	}
	for index := 0; index < cfg.ResumeCaptureCount; index++ {
		expectedPath := filepath.Join(captureDir, fmt.Sprintf("capture-%02d.jpg", index+1))
		if err := session.ValidateJPEGFile(expectedPath); err != nil {
			return r.fail(result, fmt.Errorf("validate existing capture %d JPEG: %w", index+1, err))
		}
		result.CapturePaths = append(result.CapturePaths, expectedPath)
	}
	if resumeExisting {
		if manifest.CompletedShots != cfg.ResumeCaptureCount {
			manifest.CompletedShots = cfg.ResumeCaptureCount
			manifest.UpdatedAt = time.Now().UTC()
			if err := r.persistManifest(sessionDir, manifest); err != nil {
				return r.fail(result, fmt.Errorf("reconcile atomically published capture progress: %w", err))
			}
		}
		if err := session.CleanupCaptureStagingDirectory(captureDir); err != nil {
			return r.fail(result, fmt.Errorf("clean interrupted capture staging after explicit Continue: %w", err))
		}
	}

	cameraOpen := false
	// Register cleanup before any operation that can fail after Open succeeds.
	// In particular, persisting the capturing phase must not leak an open camera.
	defer func() {
		if cameraOpen {
			if closeErr := r.Camera.Close(); closeErr != nil && runErr == nil {
				runErr = fmt.Errorf("close camera: %w", closeErr)
				if session.CanTransition(result.State, session.Failed) {
					result.State = session.Failed
				}
				result.FinishedAt = time.Now()
				r.emit(Event{Type: "failed", State: result.State, SessionID: result.SessionID, Message: runErr.Error()})
			}
		}
	}()

	if len(result.CapturePaths) < cfg.CaptureCount {
		if r.Camera == nil {
			return r.fail(result, errors.New("camera provider is required to capture the remaining photos"))
		}
		if err := r.Camera.Open(ctx); err != nil {
			return r.fail(result, fmt.Errorf("open camera %q: %w", r.Camera.Info().Name, err))
		}
		cameraOpen = true
		manifest.Phase = session.PhaseCapturing
		manifest.UpdatedAt = time.Now().UTC()
		if err := r.persistManifest(sessionDir, manifest); err != nil {
			return r.fail(result, fmt.Errorf("persist capturing session phase: %w", err))
		}
	}
	if cameraOpen {
		if err := r.transition(&result, session.Capturing, "camera opened; begin countdown and capture"); err != nil {
			return r.fail(result, err)
		}
	}
	for shot := len(result.CapturePaths) + 1; shot <= cfg.CaptureCount; shot++ {
		if err := ctx.Err(); err != nil {
			return r.fail(result, err)
		}
		if err := r.countdown(ctx, &result, cfg.Countdown, shot, cfg.CaptureCount); err != nil {
			return r.fail(result, err)
		}
		if err := stationconfig.ValidateStorageChild(cfg.WorkDir, sessionDir); err != nil {
			return r.fail(result, fmt.Errorf("revalidate session workspace before capture: %w", err))
		}
		if err := stationconfig.ValidateStorageChild(sessionDir, captureDir); err != nil {
			return r.fail(result, fmt.Errorf("revalidate capture directory before capture: %w", err))
		}
		finalPath := filepath.Join(captureDir, fmt.Sprintf("capture-%02d.jpg", shot))
		attemptDir, stagedPath, err := r.newCaptureStagingAttempt(captureDir, shot)
		if err != nil {
			return r.fail(result, fmt.Errorf("prepare staging area for capture %d: %w", shot, err))
		}
		captureStarted := time.Now()
		captured, err := r.Camera.Capture(ctx, camera.CaptureRequest{
			Sequence: shot, Width: cfg.CaptureWidth, Height: cfg.CaptureHeight, OutputPath: stagedPath,
		})
		captureDuration := time.Since(captureStarted)
		if err != nil {
			return r.fail(result, fmt.Errorf("capture shot %d/%d: %w", shot, cfg.CaptureCount, err))
		}
		if captured.Sequence != shot {
			return r.fail(result, fmt.Errorf("camera returned sequence %d for requested shot %d", captured.Sequence, shot))
		}
		if strings.TrimSpace(captured.Path) == "" {
			return r.fail(result, fmt.Errorf("camera returned an empty path for shot %d", shot))
		}
		if filepath.Clean(captured.Path) != filepath.Clean(stagedPath) {
			return r.fail(result, fmt.Errorf("camera returned an unexpected path for shot %d: got %q, requested staging path %q", shot, captured.Path, stagedPath))
		}
		info, err := os.Lstat(captured.Path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() == 0 {
			if err == nil {
				err = errors.New("staged capture output is not a non-empty regular file")
			}
			return r.fail(result, fmt.Errorf("validate staged capture shot %d output: %w", shot, err))
		}
		if err := session.ValidateJPEGFile(captured.Path); err != nil {
			return r.fail(result, fmt.Errorf("validate staged capture shot %d JPEG: %w", shot, err))
		}
		if err := syncStagedCapture(captured.Path); err != nil {
			return r.fail(result, fmt.Errorf("sync staged capture shot %d before publication: %w", shot, err))
		}
		if err := r.publishCaptureFile(stagedPath, finalPath); err != nil {
			return r.fail(result, fmt.Errorf("atomically publish capture shot %d: %w", shot, err))
		}
		if err := os.Remove(attemptDir); err != nil {
			return r.fail(result, fmt.Errorf("remove completed capture staging directory for shot %d: %w", shot, err))
		}
		stagingRoot := filepath.Dir(attemptDir)
		if err := syncCaptureDirectory(stagingRoot); err != nil {
			return r.fail(result, fmt.Errorf("sync capture staging root after shot %d cleanup: %w", shot, err))
		}
		stagingEntries, readErr := os.ReadDir(stagingRoot)
		if readErr != nil {
			return r.fail(result, fmt.Errorf("inspect capture staging root after shot %d: %w", shot, readErr))
		}
		if len(stagingEntries) == 0 {
			if err := os.Remove(stagingRoot); err != nil {
				return r.fail(result, fmt.Errorf("remove empty capture staging root: %w", err))
			}
			if err := syncCaptureDirectory(captureDir); err != nil {
				return r.fail(result, fmt.Errorf("sync captures directory after staging cleanup: %w", err))
			}
		}
		result.CapturePaths = append(result.CapturePaths, finalPath)
		result.CaptureDurations = append(result.CaptureDurations, captureDuration)
		manifest.CompletedShots = len(result.CapturePaths)
		manifest.UpdatedAt = time.Now().UTC()
		if err := r.persistManifest(sessionDir, manifest); err != nil {
			return r.fail(result, fmt.Errorf("persist completed capture %d: %w", shot, err))
		}
		r.emit(Event{Type: "captured", State: result.State, SessionID: result.SessionID, Shot: shot, Total: cfg.CaptureCount, Path: finalPath, Duration: captureDuration})
		if shot < cfg.CaptureCount && cfg.Interval > 0 {
			if err := r.sleep(ctx, cfg.Interval); err != nil {
				return r.fail(result, err)
			}
		}
	}

	// Close the device before rendering; hardware is no longer needed.
	if cameraOpen {
		if err := r.Camera.Close(); err != nil {
			cameraOpen = false
			return r.fail(result, fmt.Errorf("close camera after capture: %w", err))
		}
		cameraOpen = false
	}

	if err := r.transition(&result, session.Rendering, "compose captured images with selected frame"); err != nil {
		return r.fail(result, err)
	}
	if err := stationconfig.ValidateStorageChild(cfg.WorkDir, sessionDir); err != nil {
		return r.fail(result, fmt.Errorf("revalidate session workspace before render: %w", err))
	}
	renderedPath := filepath.Join(sessionDir, "rendered.jpg")
	renderStarted := time.Now()
	renderErr := r.Renderer.Render(ctx, cfg.FrameConfigPath, result.CapturePaths, renderedPath)
	result.RenderDuration = time.Since(renderStarted)
	if renderErr != nil {
		return r.fail(result, fmt.Errorf("render session output: %w", renderErr))
	}
	info, err := os.Stat(renderedPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		if err == nil {
			err = errors.New("rendered output is not a non-empty regular file")
		}
		return r.fail(result, fmt.Errorf("validate rendered output: %w", err))
	}
	result.RenderedPath = renderedPath
	manifest.Phase = session.PhaseRendered
	manifest.CompletedShots = len(result.CapturePaths)
	manifest.RenderedAt = time.Now().UTC()
	manifest.UpdatedAt = manifest.RenderedAt
	if err := r.persistManifest(sessionDir, manifest); err != nil {
		return r.fail(result, fmt.Errorf("persist rendered session phase: %w", err))
	}
	r.emit(Event{Type: "rendered", State: result.State, SessionID: result.SessionID, Path: renderedPath, Duration: result.RenderDuration})

	if err := r.transition(&result, session.Output, "save finalized photo locally"); err != nil {
		return r.fail(result, err)
	}
	outputStarted := time.Now()
	dispatched, err := r.Output.Dispatch(ctx, output.Job{SessionID: result.SessionID, SourcePath: renderedPath})
	result.OutputDuration = time.Since(outputStarted)
	if err != nil {
		return r.fail(result, fmt.Errorf("dispatch output targets: %w", err))
	}
	result.OutputResults = dispatched.Results
	for _, targetResult := range dispatched.Results {
		r.emit(Event{
			Type: "output_target", State: result.State, SessionID: result.SessionID,
			Target: targetResult.Name, Status: string(targetResult.Status), Path: targetResult.OutputPath,
			JobID: targetResult.JobID, Message: targetResult.Error, Duration: targetResult.Duration,
		})
		if targetResult.Name == output.TargetLocalStorage && targetResult.Status == output.TargetSucceeded {
			result.OutputPath = targetResult.OutputPath
		}
	}
	if result.OutputPath != "" {
		manifest.Phase = session.PhaseOutputFinalized
		manifest.OutputPath = result.OutputPath
		manifest.OutputFinalizedAt = time.Now().UTC()
		manifest.UpdatedAt = manifest.OutputFinalizedAt
		if err := r.persistManifest(sessionDir, manifest); err != nil {
			return r.fail(result, fmt.Errorf("persist finalized output manifest: %w", err))
		}
	}
	successfulCount := dispatched.Succeeded + dispatched.Queued
	if successfulCount == 0 {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return r.fail(result, ctxErr)
		}
		failures := make([]string, 0, len(dispatched.Results))
		targetCauses := make([]error, 0, len(dispatched.Results))
		for _, targetResult := range dispatched.Results {
			if targetResult.Status == output.TargetSucceeded || targetResult.Status == output.TargetQueued {
				continue
			}
			failures = append(failures, fmt.Sprintf("%s=%s", targetResult.Name, targetResult.Error))
			if targetResult.Cause != nil {
				targetCauses = append(targetCauses, fmt.Errorf("%s: %w", targetResult.Name, targetResult.Cause))
			}
		}
		message := strings.Join(failures, "; ")
		if joined := errors.Join(targetCauses...); joined != nil {
			return r.fail(result, fmt.Errorf("no output target succeeded: %s: %w", message, joined))
		}
		return r.fail(result, fmt.Errorf("no output target succeeded: %s", message))
	}
	nextState := session.Completed
	transitionMessage := fmt.Sprintf("%d output target(s) completed or queued", successfulCount)
	if failedCount := dispatched.Failed + dispatched.Cancelled + dispatched.TimedOut; failedCount > 0 {
		nextState = session.PartialSuccess
		transitionMessage = fmt.Sprintf("partial output: %d completed or queued, %d failed or cancelled", successfulCount, failedCount)
	}
	if err := r.transition(&result, nextState, transitionMessage); err != nil {
		return r.fail(result, err)
	}
	result.FinishedAt = time.Now()
	result.TotalDuration = result.FinishedAt.Sub(result.StartedAt)
	finishedEvent := "completed"
	if result.State == session.PartialSuccess {
		finishedEvent = "partial_success"
	}
	r.emit(Event{Type: finishedEvent, State: result.State, SessionID: result.SessionID, Path: result.OutputPath, Duration: result.TotalDuration})
	return result, nil
}

func (r *Runner) authorizePayment(ctx context.Context, cfg Config) error {
	gate := r.PaymentGate
	if gate == nil {
		mode := normalizedPaymentMode(cfg.PaymentMode)
		resolvedGate, err := payment.NewGate(stationconfig.PaymentSettings{Mode: mode})
		if err != nil {
			return fmt.Errorf("resolve payment policy: %w", err)
		}
		gate = resolvedGate
	}
	return gate.EnsureAuthorized(ctx, payment.Request{
		SessionID: cfg.SessionID,
		DemoMode:  cfg.DemoMode,
		Resume:    cfg.ResumeSession || cfg.ResumeCaptureCount > 0,
	})
}

func (r *Runner) persistManifest(sessionDir string, manifest session.Manifest) error {
	if r.writeManifest != nil {
		return r.writeManifest(sessionDir, manifest)
	}
	return session.WriteManifest(sessionDir, manifest)
}

func (r *Runner) countdown(ctx context.Context, result *Result, duration time.Duration, shot, total int) error {
	if duration <= 0 {
		r.emit(Event{Type: "countdown", State: result.State, SessionID: result.SessionID, Shot: shot, Total: total, Seconds: 0})
		return ctx.Err()
	}
	seconds := int((duration + time.Second - 1) / time.Second)
	deadline := time.Now().Add(duration)
	for remaining := seconds; remaining > 0; remaining-- {
		r.emit(Event{Type: "countdown", State: result.State, SessionID: result.SessionID, Shot: shot, Total: total, Seconds: remaining})
		until := time.Until(deadline)
		if until <= 0 {
			break
		}
		if err := r.sleep(ctx, min(time.Second, until)); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func (r *Runner) transition(result *Result, next session.State, message string) error {
	if !session.CanTransition(result.State, next) {
		return fmt.Errorf("invalid session transition %s -> %s", result.State, next)
	}
	result.State = next
	r.emit(Event{Type: "state", State: next, SessionID: result.SessionID, Message: message})
	return nil
}

func (r *Runner) fail(result Result, err error) (Result, error) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		if session.CanTransition(result.State, session.TimedOut) {
			result.State = session.TimedOut
		}
	case errors.Is(err, context.Canceled):
		if session.CanTransition(result.State, session.Cancelled) {
			result.State = session.Cancelled
		}
	default:
		if session.CanTransition(result.State, session.Failed) {
			result.State = session.Failed
		}
	}
	result.FinishedAt = time.Now()
	result.TotalDuration = result.FinishedAt.Sub(result.StartedAt)
	r.emit(Event{Type: "failed", State: result.State, SessionID: result.SessionID, Message: err.Error(), Duration: result.TotalDuration})
	return result, err
}

func (r *Runner) emit(event Event) {
	if r.OnEvent != nil {
		r.OnEvent(event)
	}
}

func (r *Runner) sleep(ctx context.Context, duration time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if duration <= 0 {
		return nil
	}
	if r.Sleep != nil {
		return r.Sleep(ctx, duration)
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func newSessionID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}

func requireSessionDirectory(path, label string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect %s %q: %w", label, path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%s %q must be a real directory, not a symlink or file", label, path)
	}
	return nil
}

func validSessionID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, c := range id {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func (r *Runner) newCaptureStagingAttempt(capturesDir string, shot int) (string, string, error) {
	stagingRoot := filepath.Join(capturesDir, session.CaptureStagingDirectoryName)
	if err := stationconfig.ValidateStorageChild(capturesDir, stagingRoot); err != nil {
		return "", "", fmt.Errorf("validate staging root: %w", err)
	}
	if err := os.Mkdir(stagingRoot, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return "", "", fmt.Errorf("create staging root: %w", err)
	}
	info, err := os.Lstat(stagingRoot)
	if err != nil {
		return "", "", fmt.Errorf("inspect staging root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", "", errors.New("capture staging root is not a real directory")
	}
	token, err := newSessionID()
	if err != nil {
		return "", "", fmt.Errorf("generate capture staging ID: %w", err)
	}
	attemptDir := filepath.Join(stagingRoot, fmt.Sprintf("shot-%02d-%s", shot, token))
	if err := stationconfig.ValidateStorageChild(stagingRoot, attemptDir); err != nil {
		return "", "", fmt.Errorf("validate capture attempt directory: %w", err)
	}
	if err := os.Mkdir(attemptDir, 0o700); err != nil {
		return "", "", fmt.Errorf("create capture attempt directory: %w", err)
	}
	return attemptDir, filepath.Join(attemptDir, "capture.jpg"), nil
}

func syncStagedCapture(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func (r *Runner) publishCaptureFile(stagedPath, finalPath string) error {
	if r.publishCapture != nil {
		return r.publishCapture(stagedPath, finalPath)
	}
	return publishCaptureNoReplace(stagedPath, finalPath)
}
