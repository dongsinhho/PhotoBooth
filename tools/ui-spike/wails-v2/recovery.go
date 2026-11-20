package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	boothoutput "github.com/dongsinhho/PhotoBooth/internal/booth/output"
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

// PrintAttemptView is a JSON-safe, presentation-only summary of a durable
// printer attempt. Domain timestamps remain time.Time inside session.Manifest.
type PrintAttemptView struct {
	AttemptID    string `json:"attempt_id"`
	QueueName    string `json:"queue_name"`
	DocumentName string `json:"document_name"`
	SourceSHA256 string `json:"source_sha256"`
	Status       string `json:"status"`
	JobID        string `json:"job_id,omitempty"`
	Error        string `json:"error,omitempty"`
	RequestedAt  string `json:"requested_at"`
	UpdatedAt    string `json:"updated_at"`
}

// PrintLockView is a JSON-safe, pre-manifest identity for an interrupted
// print submission. It lets recovery inspect the spooler if the process
// crashed before the PrintAttempt was committed to manifest v3.
type PrintLockView struct {
	LeaseVersion int    `json:"lease_version"`
	OwnerPID     int    `json:"owner_pid,omitempty"`
	AttemptID    string `json:"attempt_id"`
	QueueName    string `json:"queue_name"`
	DocumentName string `json:"document_name"`
	SourceSHA256 string `json:"source_sha256,omitempty"`
	CreatedAt    string `json:"created_at"`
}

// PrinterQueueJobView is a JSON-safe, read-only snapshot of a matching
// Windows spooler job. It never claims physical print completion.
type PrinterQueueJobView struct {
	JobID        string   `json:"job_id"`
	DocumentName string   `json:"document_name"`
	DriverStatus string   `json:"driver_status,omitempty"`
	StatusFlags  uint32   `json:"status_flags"`
	Status       []string `json:"status,omitempty"`
	Position     uint32   `json:"position"`
	TotalPages   uint32   `json:"total_pages"`
	PagesPrinted uint32   `json:"pages_printed"`
}

type PrinterInspectionView struct {
	QueueName   string                `json:"queue_name"`
	Found       bool                  `json:"found"`
	InspectedAt string                `json:"inspected_at"`
	Jobs        []PrinterQueueJobView `json:"jobs,omitempty"`
	Note        string                `json:"note"`
}

// PrintResolutionView is a JSON-safe operator decision record.
type PrintResolutionView struct {
	AttemptID      string `json:"attempt_id"`
	Decision       string `json:"decision"`
	QueueFound     bool   `json:"queue_found"`
	QueueCheckedAt string `json:"queue_checked_at"`
	RecordedAt     string `json:"recorded_at"`
	Note           string `json:"note"`
}

// RecoveryItemView is the Wails presentation DTO for read-only recovery discovery.
type RecoveryItemView struct {
	SessionID        string                `json:"session_id"`
	DirectoryPath    string                `json:"directory_path"`
	Status           string                `json:"status"`
	Reason           string                `json:"reason,omitempty"`
	CompletedShots   int                   `json:"completed_shots"`
	ExpectedShots    int                   `json:"expected_shots"`
	Phase            string                `json:"phase,omitempty"`
	Manifest         *RecoveryManifestView `json:"manifest,omitempty"`
	CapturePaths     []string              `json:"capture_paths,omitempty"`
	RenderedPath     string                `json:"rendered_path,omitempty"`
	PrintAttempts    []PrintAttemptView    `json:"print_attempts,omitempty"`
	PrintResolutions []PrintResolutionView `json:"print_resolutions,omitempty"`
	PrintLockPresent bool                  `json:"print_lock_present,omitempty"`
	PrintLockOwner   string                `json:"print_lock_owner,omitempty"`
	PrintLock        *PrintLockView        `json:"print_lock,omitempty"`
}

func recoveryCanIgnoreFrameSettingsError(message string, manifest *session.Manifest) bool {
	return manifest != nil && manifest.Version >= session.PolicyManifestVersion &&
		manifest.FrameSnapshotFile == session.FrameSnapshotFileName && isFrameOnlySettingsError(message)
}

func isFrameOnlySettingsError(message string) bool {
	return strings.HasPrefix(message, "The saved frame JSON cannot be loaded:") ||
		strings.HasPrefix(message, "The saved capture count is ")
}

func printLockView(lock *session.PrintLockInfo) *PrintLockView {
	if lock == nil {
		return nil
	}
	return &PrintLockView{
		LeaseVersion: lock.LeaseVersion, OwnerPID: lock.OwnerPID,
		AttemptID: lock.AttemptID, QueueName: lock.QueueName,
		DocumentName: lock.DocumentName, SourceSHA256: lock.SourceSHA256,
		CreatedAt: lock.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func printAttemptViews(attempts []session.PrintAttempt) []PrintAttemptView {
	views := make([]PrintAttemptView, 0, len(attempts))
	for _, attempt := range attempts {
		views = append(views, PrintAttemptView{
			AttemptID: attempt.AttemptID, QueueName: attempt.QueueName,
			DocumentName: attempt.DocumentName, SourceSHA256: attempt.SourceSHA256,
			Status: attempt.Status, JobID: attempt.JobID, Error: attempt.Error,
			RequestedAt: attempt.RequestedAt.UTC().Format(time.RFC3339Nano),
			UpdatedAt:   attempt.UpdatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	return views
}

func printResolutionViews(resolutions []session.PrintResolution) []PrintResolutionView {
	views := make([]PrintResolutionView, 0, len(resolutions))
	for _, resolution := range resolutions {
		views = append(views, PrintResolutionView{
			AttemptID: resolution.AttemptID, Decision: resolution.Decision,
			QueueFound:     resolution.QueueFound,
			QueueCheckedAt: resolution.QueueCheckedAt.UTC().Format(time.RFC3339Nano),
			RecordedAt:     resolution.RecordedAt.UTC().Format(time.RFC3339Nano),
			Note:           resolution.Note,
		})
	}
	return views
}

func recoveryItemView(item session.RecoveryItem) RecoveryItemView {
	view := RecoveryItemView{
		SessionID: item.SessionID, DirectoryPath: item.DirectoryPath,
		Status: string(item.Status), Reason: item.Reason,
		CompletedShots: item.CompletedShots, ExpectedShots: item.ExpectedShots,
		Phase: string(item.Phase), CapturePaths: append([]string(nil), item.CapturePaths...),
		RenderedPath:     item.RenderedPath,
		PrintAttempts:    printAttemptViews(item.PrintAttempts),
		PrintResolutions: printResolutionViews(item.PrintResolutions),
		PrintLockPresent: item.PrintLockPresent,
		PrintLockOwner:   item.PrintLockOwner,
		PrintLock:        printLockView(item.PrintLock),
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
				// A fully finalized session is normally hidden. Keep it visible
				// when it has a printer attempt or a stale lock so the operator
				// can inspect the spooler status without triggering a reprint.
				if len(item.PrintAttempts) == 0 && !item.PrintLockPresent {
					continue
				}
				break
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
		if item.Status != session.RecoveryFinalized || len(item.PrintAttempts) > 0 || item.PrintLockPresent {
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

func (a *App) inspectPersistedWindowsPrinterJob(queueName, documentName, jobID string) (boothoutput.PrinterJobInspection, error) {
	if a != nil && a.inspectPrinterJob != nil {
		return a.inspectPrinterJob(queueName, documentName, jobID)
	}
	return boothoutput.InspectWindowsPrinterJob(queueName, documentName, jobID)
}

// printerInspectionView converts the low-level spooler observation into the
// Wails-safe view type. An absent job is not interpreted as a successful print.
func printerInspectionView(inspection boothoutput.PrinterJobInspection) PrinterInspectionView {
	view := PrinterInspectionView{
		QueueName:   inspection.QueueName,
		Found:       inspection.Found,
		InspectedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Jobs:        make([]PrinterQueueJobView, 0, len(inspection.Jobs)),
		Note:        "The job is not currently listed in this queue. This does not prove that it printed or failed; do not resubmit automatically.",
	}
	if inspection.Found {
		view.Note = "The matching job is still listed by the Windows spooler. This is queue state, not proof of physical paper output."
	}
	for _, job := range inspection.Jobs {
		view.Jobs = append(view.Jobs, PrinterQueueJobView{
			JobID: job.JobID, DocumentName: job.DocumentName, DriverStatus: job.DriverStatus,
			StatusFlags: job.StatusFlags, Status: append([]string(nil), job.Status...),
			Position: job.Position, TotalPages: job.TotalPages, PagesPrinted: job.PagesPrinted,
		})
	}
	return view
}

// InspectPrinterAttempt performs a read-only lookup in the local Windows queue
// using the exact queue/document/job identity persisted in the session manifest.
// It does not mutate, resume, cancel, delete or resubmit a printer job.
func (a *App) InspectPrinterAttempt(sessionID, attemptID string) (PrinterInspectionView, error) {
	if runtime.GOOS != "windows" {
		return PrinterInspectionView{}, errors.New("Windows printer queue inspection is available only on the Windows Preview")
	}
	if !isPlayerSessionID(sessionID) || !isPlayerSessionID(attemptID) {
		return PrinterInspectionView{}, errors.New("invalid session or printer attempt ID")
	}
	items, err := a.listInterruptedPhotoSessions()
	if err != nil {
		return PrinterInspectionView{}, err
	}
	for _, item := range items {
		if item.SessionID != sessionID {
			continue
		}
		for _, attempt := range item.PrintAttempts {
			if attempt.AttemptID != attemptID {
				continue
			}
			switch attempt.Status {
			case session.PrintStatusQueued, session.PrintStatusSubmitting, session.PrintStatusUncertain:
			case session.PrintStatusFailed:
				if !item.PrintLockPresent {
					return PrinterInspectionView{}, errors.New("a definite pre-acceptance failure has no queue job to reconcile")
				}
			default:
				return PrinterInspectionView{}, fmt.Errorf("printer attempt state %q does not require queue reconciliation", attempt.Status)
			}
			inspection, inspectErr := a.inspectPersistedWindowsPrinterJob(
				attempt.QueueName, attempt.DocumentName, attempt.JobID,
			)
			if inspectErr != nil {
				return PrinterInspectionView{}, fmt.Errorf("inspect persisted printer attempt: %w", inspectErr)
			}
			return printerInspectionView(inspection), nil
		}
		if item.PrintLock != nil && item.PrintLock.AttemptID == attemptID {
			inspection, inspectErr := a.inspectPersistedWindowsPrinterJob(
				item.PrintLock.QueueName, item.PrintLock.DocumentName, "",
			)
			if inspectErr != nil {
				return PrinterInspectionView{}, fmt.Errorf("inspect interrupted printer lock: %w", inspectErr)
			}
			return printerInspectionView(inspection), nil
		}
	}
	return PrinterInspectionView{}, errors.New("persisted printer attempt or interrupted print lock was not found in the current recovery scan")
}

// ResolvePrinterAttempt records an operator-controlled outcome for a persisted
// queued/submitting/uncertain attempt. It re-inspects the exact Windows queue
// immediately before writing the decision. It never mutates the spooler.
// Any surviving .print.lock must first be reconciled by the separate
// kernel-verified lease recovery path; this method never removes a lock.
func (a *App) ResolvePrinterAttempt(sessionID, attemptID, decision, note string) (PrintResolutionView, error) {
	if runtime.GOOS != "windows" {
		return PrintResolutionView{}, errors.New("printer attempt resolution is available only on the Windows Preview")
	}
	if !isPlayerSessionID(sessionID) || !isPlayerSessionID(attemptID) {
		return PrintResolutionView{}, errors.New("invalid session or printer attempt ID")
	}
	decision = strings.TrimSpace(decision)
	note = strings.TrimSpace(note)
	if decision != session.PrintResolutionPrinted &&
		decision != session.PrintResolutionNotPrinted &&
		decision != session.PrintResolutionUnknown {
		return PrintResolutionView{}, fmt.Errorf("unsupported printer resolution decision %q", decision)
	}
	if note == "" || len(note) > 512 || strings.IndexFunc(note, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return PrintResolutionView{}, errors.New("enter a short printable operator note (1 to 512 bytes)")
	}

	// Serialize resolution with capture/output lifecycle operations. This does
	// not stop the idle preview or mutate the printer queue; it only prevents a
	// new session/recovery action from racing the manifest decision.
	a.cameraLifecycleMu.Lock()
	defer a.cameraLifecycleMu.Unlock()

	a.mu.Lock()
	busy := a.playerRunning || a.outputRetrying || a.settingsSaving || a.retentionBusy
	a.mu.Unlock()
	if busy {
		return PrintResolutionView{}, errors.New("finish the current station operation before resolving a printer attempt")
	}

	items, err := a.listInterruptedPhotoSessions()
	if err != nil {
		return PrintResolutionView{}, err
	}
	var selected *session.RecoveryItem
	for index := range items {
		if items[index].SessionID == sessionID {
			selected = &items[index]
			break
		}
	}
	if selected == nil || selected.Manifest == nil {
		return PrintResolutionView{}, errors.New("session manifest was not found in the current recovery scan")
	}
	if selected.PrintLockPresent || selected.PrintLock != nil {
		return PrintResolutionView{}, errors.New("a printer submission lock remains. Its owner may still be active; inspect only and do not resolve or remove this lock")
	}

	var selectedAttempt *session.PrintAttempt
	for index := range selected.PrintAttempts {
		if selected.PrintAttempts[index].AttemptID == attemptID {
			selectedAttempt = &selected.PrintAttempts[index]
			break
		}
	}
	if selectedAttempt == nil {
		return PrintResolutionView{}, errors.New("printer attempt was not found in the current recovery scan")
	}
	if selectedAttempt.Status != session.PrintStatusQueued &&
		selectedAttempt.Status != session.PrintStatusSubmitting &&
		selectedAttempt.Status != session.PrintStatusUncertain {
		return PrintResolutionView{}, fmt.Errorf("printer attempt state %q cannot be resolved", selectedAttempt.Status)
	}
	if _, resolved := session.PrintResolutionFor(*selected.Manifest, attemptID); resolved {
		return PrintResolutionView{}, errors.New("printer attempt already has an operator resolution")
	}

	lockPath := filepath.Join(selected.DirectoryPath, ".print.lock")
	if _, statErr := os.Lstat(lockPath); statErr == nil {
		return PrintResolutionView{}, errors.New("printer submission lock appeared during resolution; inspect only and retry after the active operation is gone")
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return PrintResolutionView{}, fmt.Errorf("inspect printer submission lock before resolution: %w", statErr)
	}

	inspection, err := a.inspectPersistedWindowsPrinterJob(
		selectedAttempt.QueueName, selectedAttempt.DocumentName, selectedAttempt.JobID,
	)
	if err != nil {
		return PrintResolutionView{}, fmt.Errorf("re-inspect printer queue before recording operator decision: %w", err)
	}
	checkedAt := time.Now().UTC()
	if decision != session.PrintResolutionUnknown && inspection.Found {
		return PrintResolutionView{}, errors.New("the matching job is still present in the Windows queue. Do not confirm printed/not_printed yet; wait until it is no longer listed and physically inspect the printer output")
	}

	// Re-read the latest manifest after the external queue query. This preserves
	// any unrelated manifest changes that occurred during the inspection.
	manifest, err := session.ReadManifest(selected.DirectoryPath)
	if err != nil {
		return PrintResolutionView{}, fmt.Errorf("reload session manifest before resolving printer attempt: %w", err)
	}
	if _, err := os.Lstat(lockPath); err == nil {
		return PrintResolutionView{}, errors.New("a printer submission lock appeared before the resolution was saved; no decision was recorded")
	} else if !errors.Is(err, os.ErrNotExist) {
		return PrintResolutionView{}, fmt.Errorf("recheck printer submission lock before resolution: %w", err)
	}

	recordedAt := time.Now().UTC()
	resolution := session.PrintResolution{
		AttemptID: attemptID, Decision: decision,
		QueueFound: inspection.Found, QueueCheckedAt: checkedAt,
		RecordedAt: recordedAt, Note: note,
	}
	if err := session.ResolvePrintAttempt(&manifest, resolution); err != nil {
		return PrintResolutionView{}, fmt.Errorf("validate operator printer resolution: %w", err)
	}
	if err := session.WriteManifest(selected.DirectoryPath, manifest); err != nil {
		return PrintResolutionView{}, fmt.Errorf("persist operator printer resolution: %w", err)
	}
	return PrintResolutionView{
		AttemptID: resolution.AttemptID, Decision: resolution.Decision,
		QueueFound:     resolution.QueueFound,
		QueueCheckedAt: resolution.QueueCheckedAt.UTC().Format(time.RFC3339Nano),
		RecordedAt:     resolution.RecordedAt.UTC().Format(time.RFC3339Nano),
		Note:           resolution.Note,
	}, nil
}

// PrintLockReconciliationView reports only the durable manifest state produced
// after a kernel-verified orphan lease was claimed and its lock marker removed.
type PrintLockReconciliationView struct {
	AttemptID string `json:"attempt_id"`
	Status    string `json:"status"`
	Created   bool   `json:"created"`
	Removed   bool   `json:"removed"`
	Message   string `json:"message"`
}

func hashRecoveredRenderedFile(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() ||
		info.Size() <= 0 || info.Size() > 128<<20 {
		return "", errors.New("rendered printer source is not a bounded regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// ReconcileOrphanedPrinterLock takes an exclusive Windows kernel lease on a
// versioned lock, proving that the old holder no longer owns the file. It
// converts a lock-only crash into a manifest-v3 uncertain attempt (or
// preserves an already-persisted attempt), then deletes the marker while the
// exclusive handle is still held. It does not inspect, cancel or resubmit a
// spooler job; the operator must inspect the queue and resolve the resulting
// attempt separately.
func (a *App) ReconcileOrphanedPrinterLock(sessionID string) (PrintLockReconciliationView, error) {
	if runtime.GOOS != "windows" {
		return PrintLockReconciliationView{}, errors.New("kernel-verified printer lock reconciliation is available only on Windows")
	}
	if !isPlayerSessionID(sessionID) {
		return PrintLockReconciliationView{}, errors.New("invalid session ID")
	}

	a.cameraLifecycleMu.Lock()
	defer a.cameraLifecycleMu.Unlock()
	a.mu.Lock()
	busy := a.playerRunning || a.outputRetrying || a.settingsSaving || a.retentionBusy
	a.mu.Unlock()
	if busy {
		return PrintLockReconciliationView{}, errors.New("finish the current station operation before reconciling a printer lock")
	}

	if err := a.preparePlayerPaths(); err != nil {
		return PrintLockReconciliationView{}, err
	}
	a.mu.Lock()
	workDir := a.workDir
	a.mu.Unlock()
	sessionDir := filepath.Join(workDir, sessionID)
	if err := stationconfig.ValidateStorageChild(workDir, sessionDir); err != nil {
		return PrintLockReconciliationView{}, fmt.Errorf("validate printer session directory: %w", err)
	}
	if err := stationconfig.ValidateStorageChild(sessionDir, filepath.Join(sessionDir, ".print.lock")); err != nil {
		return PrintLockReconciliationView{}, fmt.Errorf("validate printer lock path: %w", err)
	}

	lease, err := session.ClaimPrintLockLease(session.PrintLockPath(sessionDir), sessionID)
	if err != nil {
		switch {
		case errors.Is(err, session.ErrPrintLockActive):
			return PrintLockReconciliationView{}, errors.New("the printer lock is still held by an active process; do not reconcile it")
		case errors.Is(err, session.ErrPrintLockLegacy):
			return PrintLockReconciliationView{}, errors.New("this legacy printer lock has no verifiable kernel lease and remains inspect-only")
		case errors.Is(err, session.ErrPrintLockUnsupported):
			return PrintLockReconciliationView{}, err
		default:
			return PrintLockReconciliationView{}, fmt.Errorf("claim orphaned printer lock: %w", err)
		}
	}
	defer lease.Close()
	lockInfo := lease.Info
	if lockInfo.LeaseVersion != session.PrintLockLeaseVersion || !isPlayerSessionID(lockInfo.AttemptID) {
		return PrintLockReconciliationView{}, errors.New("printer lock lease version or attempt identity is not supported")
	}

	manifest, err := session.ReadManifest(sessionDir)
	if err != nil {
		return PrintLockReconciliationView{}, fmt.Errorf("read session manifest for printer-lock reconciliation: %w", err)
	}
	if manifest.SessionID != sessionID ||
		(manifest.Phase != session.PhaseRendered && manifest.Phase != session.PhaseOutputFinalized) {
		return PrintLockReconciliationView{}, errors.New("orphaned printer lock is not attached to a verified rendered session")
	}
	renderedPath := filepath.Join(sessionDir, "rendered.jpg")
	if err := stationconfig.ValidateStorageChild(sessionDir, renderedPath); err != nil {
		return PrintLockReconciliationView{}, fmt.Errorf("validate rendered source path: %w", err)
	}
	if err := session.ValidateJPEGFile(renderedPath); err != nil {
		return PrintLockReconciliationView{}, fmt.Errorf("validate rendered source before lock reconciliation: %w", err)
	}
	sourceHash, err := hashRecoveredRenderedFile(renderedPath)
	if err != nil {
		return PrintLockReconciliationView{}, fmt.Errorf("hash rendered source before lock reconciliation: %w", err)
	}
	if sourceHash != lockInfo.SourceSHA256 {
		return PrintLockReconciliationView{}, errors.New("rendered source SHA-256 does not match the orphaned lock; preserve the lock for manual inspection")
	}

	// A crash may occur after creating the new lock but before upgrading a
	// snapshot-backed v2 manifest. Upgrade only after re-verifying the snapshot.
	if manifest.Version == session.PolicyManifestVersion {
		if _, _, err := session.ReadFrameSnapshot(sessionDir, manifest); err != nil {
			return PrintLockReconciliationView{}, fmt.Errorf("verify immutable frame snapshot before manifest upgrade: %w", err)
		}
		manifest.Version = session.ManifestVersion
		if err := session.WriteManifest(sessionDir, manifest); err != nil {
			return PrintLockReconciliationView{}, fmt.Errorf("upgrade manifest before printer-lock reconciliation: %w", err)
		}
	}
	if manifest.Version != session.ManifestVersion {
		return PrintLockReconciliationView{}, errors.New("orphaned print lock can only be reconciled with a verified manifest-v3 session")
	}

	var attempt *session.PrintAttempt
	for index := range manifest.PrintAttempts {
		if manifest.PrintAttempts[index].AttemptID == lockInfo.AttemptID {
			attempt = &manifest.PrintAttempts[index]
			break
		}
	}

	created := false
	if attempt != nil {
		if attempt.QueueName != lockInfo.QueueName || attempt.DocumentName != lockInfo.DocumentName ||
			attempt.SourceSHA256 != lockInfo.SourceSHA256 {
			return PrintLockReconciliationView{}, errors.New("orphaned lock identity conflicts with its persisted manifest attempt")
		}
		switch attempt.Status {
		case session.PrintStatusSubmitting:
			at := time.Now().UTC()
			if at.Before(attempt.UpdatedAt) {
				at = attempt.UpdatedAt
			}
			message := "The printer process no longer holds its kernel lease, but its submission result was not finalized. Inspect the exact Windows queue and physical output before any manual decision."
			if err := session.FinishPrintAttempt(&manifest, lockInfo.AttemptID, session.PrintStatusUncertain, "", message, at); err != nil {
				return PrintLockReconciliationView{}, fmt.Errorf("mark interrupted printer attempt uncertain: %w", err)
			}
			if err := session.WriteManifest(sessionDir, manifest); err != nil {
				return PrintLockReconciliationView{}, fmt.Errorf("persist uncertain printer attempt after lock recovery: %w", err)
			}
		case session.PrintStatusQueued, session.PrintStatusUncertain, session.PrintStatusFailed:
			// The outcome was already persisted. The surviving lease means the
			// process exited before lock cleanup; preserve that durable result.
		default:
			return PrintLockReconciliationView{}, fmt.Errorf("persisted printer attempt state %q cannot be reconciled from a lock", attempt.Status)
		}
	} else {
		requestedAt := lockInfo.CreatedAt
		if requestedAt.Before(manifest.RenderedAt) {
			requestedAt = manifest.RenderedAt
		}
		if requestedAt.Before(manifest.UpdatedAt) {
			requestedAt = manifest.UpdatedAt
		}
		newAttempt := session.PrintAttempt{
			AttemptID: lockInfo.AttemptID, QueueName: lockInfo.QueueName,
			DocumentName: lockInfo.DocumentName, SourceSHA256: lockInfo.SourceSHA256,
			Status: session.PrintStatusSubmitting, RequestedAt: requestedAt, UpdatedAt: requestedAt,
		}
		if err := session.BeginPrintAttempt(&manifest, newAttempt); err != nil {
			return PrintLockReconciliationView{}, fmt.Errorf("append recovered printer intent: %w", err)
		}
		uncertainAt := time.Now().UTC()
		if uncertainAt.Before(requestedAt) {
			uncertainAt = requestedAt
		}
		message := "The process exited after creating the printer lease but before the submission intent was finalized. Inspect the exact Windows queue and physical output before any manual decision."
		if err := session.FinishPrintAttempt(&manifest, lockInfo.AttemptID, session.PrintStatusUncertain, "", message, uncertainAt); err != nil {
			return PrintLockReconciliationView{}, fmt.Errorf("complete recovered lock-only printer intent as uncertain: %w", err)
		}
		if err := session.WriteManifest(sessionDir, manifest); err != nil {
			return PrintLockReconciliationView{}, fmt.Errorf("persist recovered lock-only printer attempt: %w", err)
		}
		created = true
	}

	if err := lease.Remove(); err != nil {
		return PrintLockReconciliationView{}, fmt.Errorf("persisted the uncertain attempt but could not remove the exclusively claimed orphaned lock; refresh and retry reconciliation: %w", err)
	}
	status := session.PrintStatusUncertain
	for _, item := range manifest.PrintAttempts {
		if item.AttemptID == lockInfo.AttemptID {
			status = item.Status
			break
		}
	}
	return PrintLockReconciliationView{
		AttemptID: lockInfo.AttemptID, Status: status, Created: created, Removed: true,
		Message: "The orphaned kernel lease was claimed and the lock marker was removed. The print outcome remains unverified; inspect the saved queue/document/job and physical printer output before resolving it.",
	}, nil
}
