package session

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode"

	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
)

const (
	ManifestFileName            = "manifest.json"
	LegacyManifestVersion       = 1
	PolicyManifestVersion       = 2
	ManifestVersion             = 3
	CaptureStagingDirectoryName = ".staging"
	maxPrintAttempts            = 8
	maxManifestBytes            = 64 << 10
	maxRecoveryRootEntries      = 4096
	maxSessionDirectoryEntries  = 5  // manifest, captures, optional frame snapshot, rendered.jpg and print lock
	maxCaptureDirectoryEntries  = 13 // up to 12 committed captures plus .staging
	maxCaptureStagingEntries    = 12
	maxRecoveryJPEGBytes        = 40 << 20
	maxRecoveryJPEGDimension    = 8000
	maxRecoveryJPEGPixels       = 40_000_000
)

type Phase string

const (
	PhasePreflight       Phase = "preflight"
	PhaseCapturing       Phase = "capturing"
	PhaseRendered        Phase = "rendered"
	PhaseOutputFinalized Phase = "output_finalized"
)

type Manifest struct {
	Version               int               `json:"version"`
	SessionID             string            `json:"session_id"`
	Phase                 Phase             `json:"phase"`
	ExpectedShots         int               `json:"expected_shots"`
	CompletedShots        int               `json:"completed_shots"`
	FrameConfigPath       string            `json:"frame_config_path"`
	FrameConfigSHA256     string            `json:"frame_config_sha256"`
	FrameSnapshotFile     string            `json:"frame_snapshot_file,omitempty"`
	FrameSnapshotSHA256   string            `json:"frame_snapshot_sha256,omitempty"`
	RunConfigFingerprint  string            `json:"run_config_fingerprint"`
	ConfigSchemaVersion   int               `json:"config_schema_version"`
	CameraProvider        string            `json:"camera_provider,omitempty"`
	CameraDevice          string            `json:"camera_device,omitempty"`
	CaptureWidth          int               `json:"capture_width,omitempty"`
	CaptureHeight         int               `json:"capture_height,omitempty"`
	CountdownMilliseconds int64             `json:"countdown_ms,omitempty"`
	IntervalMilliseconds  int64             `json:"interval_ms,omitempty"`
	DemoMode              bool              `json:"demo_mode,omitempty"`
	MediaDeliveryMode     string            `json:"media_delivery_mode,omitempty"`
	PaymentMode           string            `json:"payment_mode,omitempty"`
	PrintAttempts         []PrintAttempt    `json:"print_attempts,omitempty"`
	PrintResolutions      []PrintResolution `json:"print_resolutions,omitempty"`
	CreatedAt             time.Time         `json:"created_at"`
	UpdatedAt             time.Time         `json:"updated_at"`
	RenderedAt            time.Time         `json:"rendered_at,omitempty"`
	OutputFinalizedAt     time.Time         `json:"output_finalized_at,omitempty"`
	OutputPath            string            `json:"output_path,omitempty"`
}

// PrintAttempt records a single durable attempt to hand a rendered session
// image to a Windows printer queue. A queued status means the OS spooler
// accepted the job; it never means that physical paper was confirmed.
type PrintAttempt struct {
	AttemptID    string    `json:"attempt_id"`
	QueueName    string    `json:"queue_name"`
	DocumentName string    `json:"document_name"`
	SourceSHA256 string    `json:"source_sha256"`
	Status       string    `json:"status"`
	JobID        string    `json:"job_id,omitempty"`
	Error        string    `json:"error,omitempty"`
	RequestedAt  time.Time `json:"requested_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PrintLockInfo is written to .print.lock before the manifest intent is
// published. It lets recovery inspect the exact Windows queue/document even
// when a process crashes in the small window before the manifest write.
// PrintResolution is an append-only operator decision for one queued or
// ambiguous printer attempt. Queue absence is only a recorded observation;
// printed/not_printed decisions additionally reflect the operator's physical
// inspection of the printer output.
type PrintResolution struct {
	AttemptID      string    `json:"attempt_id"`
	Decision       string    `json:"decision"`
	QueueFound     bool      `json:"queue_found"`
	QueueCheckedAt time.Time `json:"queue_checked_at"`
	RecordedAt     time.Time `json:"recorded_at"`
	Note           string    `json:"note"`
}

type PrintLockInfo struct {
	LeaseVersion int       `json:"lease_version,omitempty"`
	OwnerPID     int       `json:"owner_pid,omitempty"`
	AttemptID    string    `json:"attempt_id"`
	QueueName    string    `json:"queue_name"`
	DocumentName string    `json:"document_name"`
	SourceSHA256 string    `json:"source_sha256,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

const (
	PrintStatusSubmitting = "submitting"
	PrintStatusQueued     = "queued"
	PrintStatusFailed     = "failed"
	PrintStatusUncertain  = "uncertain"

	PrintResolutionPrinted    = "printed"
	PrintResolutionNotPrinted = "not_printed"
	PrintResolutionUnknown    = "unknown"
)

func PrintDocumentName(sessionID, attemptID string) string {
	return "PhotoBooth-" + sessionID + "-" + attemptID
}

type RecoveryStatus string

const (
	RecoveryResumeAvailable RecoveryStatus = "resume_available"
	RecoveryOutputPending   RecoveryStatus = "output_pending"
	RecoveryFinalized       RecoveryStatus = "finalized"
	RecoveryInspectOnly     RecoveryStatus = "inspect_only"
)

type RecoveryItem struct {
	SessionID        string            `json:"session_id"`
	DirectoryPath    string            `json:"directory_path"`
	Status           RecoveryStatus    `json:"status"`
	Reason           string            `json:"reason,omitempty"`
	CompletedShots   int               `json:"completed_shots"`
	ExpectedShots    int               `json:"expected_shots"`
	Phase            Phase             `json:"phase,omitempty"`
	Manifest         *Manifest         `json:"manifest,omitempty"`
	FrameConfigPath  string            `json:"frame_config_path,omitempty"`
	CapturePaths     []string          `json:"capture_paths,omitempty"`
	RenderedPath     string            `json:"rendered_path,omitempty"`
	PrintAttempts    []PrintAttempt    `json:"print_attempts,omitempty"`
	PrintResolutions []PrintResolution `json:"print_resolutions,omitempty"`
	PrintLockPresent bool              `json:"print_lock_present,omitempty"`
	PrintLockOwner   string            `json:"print_lock_owner,omitempty"`
	PrintLock        *PrintLockInfo    `json:"print_lock,omitempty"`
}

type RecoveryScan struct {
	Items          []RecoveryItem `json:"items"`
	SkippedEntries int            `json:"skipped_entries"`
}

func NewManifest(sessionID string, expectedShots int, framePath, frameSHA256, fingerprint string) Manifest {
	now := time.Now().UTC()
	return Manifest{
		Version: ManifestVersion, SessionID: sessionID, Phase: PhasePreflight,
		ExpectedShots: expectedShots, FrameConfigPath: framePath,
		FrameConfigSHA256: frameSHA256, RunConfigFingerprint: fingerprint,
		MediaDeliveryMode: stationconfig.MediaDeliveryModeLocal,
		PaymentMode:       stationconfig.PaymentModeDisabled,
		CreatedAt:         now, UpdatedAt: now,
	}
}

func ValidateManifest(m Manifest) error {
	if m.Version < LegacyManifestVersion || m.Version > ManifestVersion {
		return fmt.Errorf("unsupported session manifest version %d", m.Version)
	}
	if !validSessionID(m.SessionID) {
		return errors.New("manifest session ID is invalid")
	}
	if m.Phase != PhasePreflight && m.Phase != PhaseCapturing &&
		m.Phase != PhaseRendered && m.Phase != PhaseOutputFinalized {
		return fmt.Errorf("manifest phase %q is unknown", m.Phase)
	}
	if m.ExpectedShots < 1 || m.ExpectedShots > 12 {
		return errors.New("manifest expected_shots must be between 1 and 12")
	}
	if m.CompletedShots < 0 || m.CompletedShots > m.ExpectedShots {
		return errors.New("manifest completed_shots is outside the expected range")
	}
	if strings.TrimSpace(m.FrameConfigPath) == "" {
		return errors.New("manifest frame_config_path is required")
	}
	if !validSHA256(m.FrameConfigSHA256) {
		return errors.New("manifest frame_config_sha256 must be a lowercase SHA-256 digest")
	}
	if m.Version == LegacyManifestVersion && (m.FrameSnapshotFile != "" || m.FrameSnapshotSHA256 != "") {
		return errors.New("legacy session manifest cannot declare a frame snapshot")
	}
	if m.Version >= PolicyManifestVersion && m.FrameSnapshotFile == "" {
		return fmt.Errorf("version-%d session manifest requires an immutable frame snapshot", m.Version)
	}
	if m.Version < ManifestVersion && len(m.PrintAttempts) != 0 {
		return errors.New("printer attempts require a version-3 session manifest")
	}
	if len(m.PrintAttempts) > maxPrintAttempts {
		return fmt.Errorf("manifest contains more than %d printer attempts", maxPrintAttempts)
	}
	if m.Version < ManifestVersion && len(m.PrintResolutions) != 0 {
		return errors.New("printer resolutions require a version-3 session manifest")
	}
	if len(m.PrintResolutions) > maxPrintAttempts {
		return fmt.Errorf("manifest contains more than %d printer resolutions", maxPrintAttempts)
	}
	if m.FrameSnapshotFile == "" {
		if m.FrameSnapshotSHA256 != "" {
			return errors.New("manifest frame_snapshot_sha256 requires frame_snapshot_file")
		}
	} else if m.FrameSnapshotFile != FrameSnapshotFileName ||
		!validSHA256(m.FrameSnapshotSHA256) || m.FrameSnapshotSHA256 != m.FrameConfigSHA256 {
		return errors.New("manifest frame snapshot metadata is invalid")
	}
	if !validSHA256(m.RunConfigFingerprint) {
		return errors.New("manifest run_config_fingerprint must be a lowercase SHA-256 digest")
	}
	if m.ConfigSchemaVersion < 0 {
		return errors.New("manifest config_schema_version cannot be negative")
	}
	if m.Version >= PolicyManifestVersion {
		if m.MediaDeliveryMode != stationconfig.MediaDeliveryModeLocal {
			return fmt.Errorf("manifest media_delivery_mode %q is not supported", m.MediaDeliveryMode)
		}
		if m.PaymentMode != stationconfig.PaymentModeDisabled {
			return fmt.Errorf("manifest payment_mode %q is not supported", m.PaymentMode)
		}
	}
	seenPrintAttempts := make(map[string]struct{}, len(m.PrintAttempts))
	for index, attempt := range m.PrintAttempts {
		if err := validatePrintAttempt(m, attempt); err != nil {
			return fmt.Errorf("manifest print_attempts[%d]: %w", index, err)
		}
		if _, exists := seenPrintAttempts[attempt.AttemptID]; exists {
			return fmt.Errorf("manifest print_attempts contains duplicate attempt ID %q", attempt.AttemptID)
		}
		seenPrintAttempts[attempt.AttemptID] = struct{}{}
		if index > 0 && attempt.RequestedAt.Before(m.PrintAttempts[index-1].RequestedAt) {
			return errors.New("manifest print attempts are not ordered by request time")
		}
	}
	if m.CreatedAt.IsZero() || m.UpdatedAt.IsZero() || m.UpdatedAt.Before(m.CreatedAt) {
		return errors.New("manifest timestamps are invalid")
	}
	if m.Phase == PhasePreflight && m.CompletedShots != 0 {
		return errors.New("preflight manifest cannot contain completed captures")
	}
	if m.Phase == PhaseRendered && m.CompletedShots != m.ExpectedShots {
		return errors.New("rendered manifest must contain every expected capture")
	}
	if m.Phase == PhasePreflight || m.Phase == PhaseCapturing {
		if !m.RenderedAt.IsZero() || !m.OutputFinalizedAt.IsZero() || m.OutputPath != "" {
			return errors.New("capture-phase manifest cannot contain rendered or output-finalized metadata")
		}
	}
	if m.Phase == PhaseRendered || m.Phase == PhaseOutputFinalized {
		if m.CompletedShots != m.ExpectedShots || m.RenderedAt.IsZero() ||
			m.RenderedAt.Before(m.CreatedAt) || m.RenderedAt.After(m.UpdatedAt) {
			return errors.New("rendered manifest requires all captures and a valid rendered timestamp")
		}
	}
	if m.Phase == PhaseOutputFinalized {
		if strings.TrimSpace(m.OutputPath) == "" || m.OutputFinalizedAt.IsZero() ||
			m.OutputFinalizedAt.Before(m.RenderedAt) || m.OutputFinalizedAt.After(m.UpdatedAt) {
			return errors.New("output-finalized manifest requires a path and valid finalization timestamp")
		}
	} else if !m.OutputFinalizedAt.IsZero() || m.OutputPath != "" {
		return errors.New("output-finalized metadata is only valid after output publication")
	}
	if len(m.PrintAttempts) > 0 && m.Phase != PhaseRendered && m.Phase != PhaseOutputFinalized {
		return errors.New("printer attempts are only valid after the rendered phase")
	}
	for _, attempt := range m.PrintAttempts {
		if attempt.RequestedAt.Before(m.RenderedAt) || attempt.UpdatedAt.After(m.UpdatedAt) {
			return errors.New("printer attempt timestamps are inconsistent with the rendered manifest")
		}
	}
	seenResolutions := make(map[string]struct{}, len(m.PrintResolutions))
	for index, resolution := range m.PrintResolutions {
		if err := validatePrintResolution(m, resolution); err != nil {
			return fmt.Errorf("manifest print_resolutions[%d]: %w", index, err)
		}
		if _, exists := seenResolutions[resolution.AttemptID]; exists {
			return fmt.Errorf("manifest print_resolutions contains duplicate attempt ID %q", resolution.AttemptID)
		}
		seenResolutions[resolution.AttemptID] = struct{}{}
		if index > 0 && resolution.RecordedAt.Before(m.PrintResolutions[index-1].RecordedAt) {
			return errors.New("manifest print resolutions are not ordered by record time")
		}
	}
	return nil
}

func validatePrintAttempt(m Manifest, attempt PrintAttempt) error {
	if !validSessionID(attempt.AttemptID) {
		return errors.New("attempt_id must be a 32-character lowercase hexadecimal identifier")
	}
	if strings.TrimSpace(attempt.QueueName) == "" || len(attempt.QueueName) > 256 {
		return errors.New("queue_name must contain between 1 and 256 bytes")
	}
	if attempt.QueueName != strings.TrimSpace(attempt.QueueName) {
		return errors.New("queue_name must not contain leading or trailing whitespace")
	}
	if attempt.DocumentName != PrintDocumentName(m.SessionID, attempt.AttemptID) {
		return errors.New("document_name does not match the session and attempt IDs")
	}
	if !validSHA256(attempt.SourceSHA256) {
		return errors.New("source_sha256 must be a lowercase SHA-256 digest")
	}
	switch attempt.Status {
	case PrintStatusSubmitting:
		if attempt.JobID != "" || attempt.Error != "" {
			return errors.New("submitting attempt cannot contain a job ID or error")
		}
	case PrintStatusQueued:
		if strings.TrimSpace(attempt.JobID) == "" || len(attempt.JobID) > 128 || attempt.Error != "" {
			return errors.New("queued attempt requires a bounded job ID and no error")
		}
	case PrintStatusFailed:
		if attempt.JobID != "" || strings.TrimSpace(attempt.Error) == "" || len(attempt.Error) > 1024 {
			return errors.New("failed attempt requires a bounded error and no job ID")
		}
	case PrintStatusUncertain:
		if len(attempt.JobID) > 128 || strings.TrimSpace(attempt.Error) == "" || len(attempt.Error) > 1024 {
			return errors.New("uncertain attempt requires a bounded error and an optional bounded job ID")
		}
	default:
		return fmt.Errorf("unknown printer attempt status %q", attempt.Status)
	}
	if attempt.RequestedAt.IsZero() || attempt.UpdatedAt.IsZero() ||
		attempt.UpdatedAt.Before(attempt.RequestedAt) || attempt.UpdatedAt.After(m.UpdatedAt) {
		return errors.New("printer attempt timestamps are invalid")
	}
	return nil
}

func validatePrintResolution(m Manifest, resolution PrintResolution) error {
	if !validSessionID(resolution.AttemptID) {
		return errors.New("attempt_id must be a 32-character lowercase hexadecimal identifier")
	}
	var attempt *PrintAttempt
	for index := range m.PrintAttempts {
		if m.PrintAttempts[index].AttemptID == resolution.AttemptID {
			attempt = &m.PrintAttempts[index]
			break
		}
	}
	if attempt == nil {
		return errors.New("resolution refers to an unknown printer attempt")
	}
	if attempt.Status != PrintStatusQueued && attempt.Status != PrintStatusSubmitting && attempt.Status != PrintStatusUncertain {
		return fmt.Errorf("attempt state %q does not require an operator resolution", attempt.Status)
	}
	switch resolution.Decision {
	case PrintResolutionPrinted, PrintResolutionNotPrinted:
		if resolution.QueueFound {
			return errors.New("printed/not_printed decisions require the matching job to be absent from the queue")
		}
	case PrintResolutionUnknown:
	default:
		return fmt.Errorf("unknown printer resolution decision %q", resolution.Decision)
	}
	if resolution.QueueCheckedAt.IsZero() || resolution.QueueCheckedAt.Before(attempt.RequestedAt) ||
		resolution.RecordedAt.IsZero() || resolution.RecordedAt.Before(resolution.QueueCheckedAt) ||
		resolution.RecordedAt.Before(attempt.UpdatedAt) || resolution.RecordedAt.After(m.UpdatedAt) {
		return errors.New("printer resolution timestamps are invalid")
	}
	if strings.TrimSpace(resolution.Note) == "" || len(resolution.Note) > 512 ||
		strings.IndexFunc(resolution.Note, unicode.IsControl) >= 0 {
		return errors.New("printer resolution note must contain 1 to 512 printable bytes")
	}
	return nil
}

// PrintResolutionFor returns the immutable operator decision for an attempt.
func PrintResolutionFor(m Manifest, attemptID string) (PrintResolution, bool) {
	for _, resolution := range m.PrintResolutions {
		if resolution.AttemptID == attemptID {
			return resolution, true
		}
	}
	return PrintResolution{}, false
}

// ResolvePrintAttempt appends one operator decision. It never changes the
// original PrintAttempt record or claims that the spooler confirmed physical output.
func ResolvePrintAttempt(m *Manifest, resolution PrintResolution) error {
	if m == nil {
		return errors.New("session manifest is required")
	}
	if m.Version != ManifestVersion {
		return fmt.Errorf("printer resolutions require manifest version %d", ManifestVersion)
	}
	if len(m.PrintResolutions) >= maxPrintAttempts {
		return fmt.Errorf("session already has the maximum of %d printer resolutions", maxPrintAttempts)
	}
	if _, exists := PrintResolutionFor(*m, resolution.AttemptID); exists {
		return fmt.Errorf("printer attempt %s already has an operator resolution", resolution.AttemptID)
	}
	if resolution.RecordedAt.IsZero() {
		return errors.New("printer resolution record time is required")
	}
	m.PrintResolutions = append(m.PrintResolutions, resolution)
	m.UpdatedAt = resolution.RecordedAt
	return ValidateManifest(*m)
}

// BeginPrintAttempt durably appends a pre-submission intent. A queued,
// submitting or uncertain previous attempt blocks another submission. Only a
// known pre-acceptance failure or an operator-confirmed not_printed resolution
// allows a later explicitly invoked attempt.
func BeginPrintAttempt(m *Manifest, attempt PrintAttempt) error {
	if m == nil {
		return errors.New("session manifest is required")
	}
	if m.Version != ManifestVersion {
		return fmt.Errorf("durable printer attempts require manifest version %d", ManifestVersion)
	}
	if m.Phase != PhaseRendered && m.Phase != PhaseOutputFinalized {
		return errors.New("printer attempts require a rendered session")
	}
	if len(m.PrintAttempts) >= maxPrintAttempts {
		return fmt.Errorf("session already has the maximum of %d printer attempts", maxPrintAttempts)
	}
	if len(m.PrintAttempts) > 0 {
		last := m.PrintAttempts[len(m.PrintAttempts)-1]
		manualRetryAllowed := last.Status == PrintStatusFailed
		if resolution, ok := PrintResolutionFor(*m, last.AttemptID); ok {
			if resolution.Decision == PrintResolutionNotPrinted {
				manualRetryAllowed = true
			} else {
				return fmt.Errorf("previous printer attempt %s has resolution %q; a new attempt is not allowed", last.AttemptID, resolution.Decision)
			}
		}
		if !manualRetryAllowed {
			return fmt.Errorf("previous printer attempt %s is %q; operator resolution is required before another submission", last.AttemptID, last.Status)
		}
	}
	if attempt.Status != PrintStatusSubmitting || attempt.JobID != "" || attempt.Error != "" {
		return errors.New("new printer attempt must start in submitting state without a job ID or error")
	}
	if attempt.RequestedAt.IsZero() || attempt.UpdatedAt.IsZero() || !attempt.RequestedAt.Equal(attempt.UpdatedAt) {
		return errors.New("new printer attempt must have matching non-zero request/update timestamps")
	}
	m.PrintAttempts = append(m.PrintAttempts, attempt)
	m.UpdatedAt = attempt.UpdatedAt
	return ValidateManifest(*m)
}

// FinishPrintAttempt records the result of one previously persisted submitting
// intent. A failed manifest write leaves the on-disk state as submitting,
// which recovery deliberately classifies as uncertain.
func FinishPrintAttempt(m *Manifest, attemptID, status, jobID, message string, at time.Time) error {
	if m == nil {
		return errors.New("session manifest is required")
	}
	for index := range m.PrintAttempts {
		attempt := &m.PrintAttempts[index]
		if attempt.AttemptID != attemptID {
			continue
		}
		if attempt.Status != PrintStatusSubmitting {
			return fmt.Errorf("printer attempt %s is already %q", attemptID, attempt.Status)
		}
		if status != PrintStatusQueued && status != PrintStatusFailed && status != PrintStatusUncertain {
			return fmt.Errorf("invalid completed printer attempt status %q", status)
		}
		if at.IsZero() || at.Before(attempt.RequestedAt) {
			return errors.New("printer attempt completion timestamp is invalid")
		}
		attempt.Status, attempt.JobID, attempt.Error, attempt.UpdatedAt = status, jobID, message, at
		m.UpdatedAt = at
		return ValidateManifest(*m)
	}
	return fmt.Errorf("printer attempt %s was not found", attemptID)
}

func WriteManifest(sessionDir string, m Manifest) error {
	if err := ValidateManifest(m); err != nil {
		return fmt.Errorf("validate session manifest: %w", err)
	}
	if filepath.Base(filepath.Clean(sessionDir)) != m.SessionID {
		return errors.New("manifest session ID does not match its directory")
	}
	if err := stationconfig.ValidateStorageChild(filepath.Dir(sessionDir), sessionDir); err != nil {
		return fmt.Errorf("validate session manifest directory identity: %w", err)
	}
	dirInfo, err := os.Lstat(sessionDir)
	if err != nil {
		return fmt.Errorf("inspect session manifest directory: %w", err)
	}
	if !dirInfo.IsDir() || dirInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("session manifest parent must be a real directory")
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("encode session manifest: %w", err)
	}
	if len(data) > maxManifestBytes {
		return errors.New("session manifest exceeds the size limit")
	}
	temp, err := os.CreateTemp(sessionDir, ".manifest-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary session manifest: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("secure temporary session manifest: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write temporary session manifest: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync temporary session manifest: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary session manifest: %w", err)
	}
	if err := atomicReplaceFile(tempPath, filepath.Join(sessionDir, ManifestFileName)); err != nil {
		return fmt.Errorf("publish session manifest: %w", err)
	}
	return nil
}

func ReadManifest(sessionDir string) (Manifest, error) {
	var m Manifest
	path := filepath.Join(sessionDir, ManifestFileName)
	info, err := os.Lstat(path)
	if err != nil {
		return m, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxManifestBytes {
		return m, errors.New("session manifest is not a bounded regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return m, fmt.Errorf("read session manifest: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&m); err != nil {
		return m, fmt.Errorf("decode session manifest: %w", err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		if err == nil {
			return m, errors.New("session manifest contains multiple JSON documents")
		}
		return m, fmt.Errorf("validate session manifest ending: %w", err)
	}
	if err := ValidateManifest(m); err != nil {
		return m, err
	}
	if filepath.Base(filepath.Clean(sessionDir)) != m.SessionID {
		return m, errors.New("manifest session ID does not match its directory")
	}
	return m, nil
}

// InspectRecoveryRoot performs a read-only, bounded inspection. It never
// creates, renames, repairs, resumes or deletes session data.
func InspectRecoveryRoot(sessionsRoot string) (RecoveryScan, error) {
	var scan RecoveryScan
	rootInfo, err := os.Lstat(sessionsRoot)
	if errors.Is(err, os.ErrNotExist) {
		return scan, nil
	}
	if err != nil {
		return scan, fmt.Errorf("inspect sessions root: %w", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return scan, errors.New("sessions root must be a real local directory")
	}
	absoluteRoot, err := filepath.Abs(sessionsRoot)
	if err != nil {
		return scan, fmt.Errorf("resolve sessions root: %w", err)
	}
	canonicalRoot, err := stationconfig.CanonicalLocalStoragePath(sessionsRoot)
	if err != nil {
		return scan, fmt.Errorf("resolve canonical sessions root: %w", err)
	}
	if !sameRecoveryPath(absoluteRoot, canonicalRoot) {
		return scan, errors.New("sessions root resolves through a symlink or junction; recovery discovery is disabled")
	}
	if err := stationconfig.ValidateLocalStoragePath(sessionsRoot); err != nil {
		return scan, fmt.Errorf("validate sessions root: %w", err)
	}
	entries, tooMany, err := readDirectoryLimited(sessionsRoot, maxRecoveryRootEntries)
	if err != nil {
		return scan, fmt.Errorf("read sessions root: %w", err)
	}
	if tooMany {
		return scan, fmt.Errorf("sessions root contains more than %d entries; recovery discovery is bounded and was not run", maxRecoveryRootEntries)
	}
	for _, entry := range entries {
		if !validSessionID(entry.Name()) {
			scan.SkippedEntries++
			continue
		}
		item := RecoveryItem{
			SessionID:     entry.Name(),
			DirectoryPath: filepath.Join(sessionsRoot, entry.Name()),
			Status:        RecoveryInspectOnly,
		}
		if err := stationconfig.ValidateStorageChild(sessionsRoot, item.DirectoryPath); err != nil {
			item.Reason = "The session directory resolves through a redirected path. Its contents were not inspected."
			scan.Items = append(scan.Items, item)
			continue
		}
		info, statErr := os.Lstat(item.DirectoryPath)
		if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			item.Reason = "The session path is not a real directory."
			scan.Items = append(scan.Items, item)
			continue
		}
		inspectSessionDirectory(&item)
		scan.Items = append(scan.Items, item)
	}
	sort.Slice(scan.Items, func(i, j int) bool {
		return scan.Items[i].SessionID < scan.Items[j].SessionID
	})
	return scan, nil
}

func readPrintLock(path, sessionID string) (PrintLockInfo, error) {
	var lock PrintLockInfo
	file, err := openPrintLockForRead(path)
	if err != nil {
		return lock, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		info.Size() <= 0 || info.Size() > PrintLockMaxBytes {
		if err != nil {
			return lock, fmt.Errorf("inspect printer lock read handle: %w", err)
		}
		return lock, errors.New("printer lock is not a bounded regular file")
	}
	return readPrintLockFromFile(file, sessionID)
}

func inspectSessionDirectory(item *RecoveryItem) {
	entries, tooMany, err := readDirectoryLimited(item.DirectoryPath, maxSessionDirectoryEntries)
	if err != nil {
		item.Reason = "The session directory could not be read."
		return
	}
	if tooMany {
		item.Reason = "The session contains too many entries. It is available for manual inspection only."
		return
	}
	known := map[string]bool{
		ManifestFileName: true, "captures": true, "rendered.jpg": true, FrameSnapshotFileName: true, ".print.lock": true,
	}
	printLockPresent := false
	for _, entry := range entries {
		if !known[entry.Name()] {
			item.Reason = "The session contains unexpected files or directories. It is available for inspection only."
			break
		}
		if entry.Name() == ".print.lock" {
			printLockPresent = true
			info, statErr := os.Lstat(filepath.Join(item.DirectoryPath, entry.Name()))
			if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 512 {
				item.Reason = firstReason(item.Reason, "The printer submission lock is not a bounded regular file.")
			} else {
				lockInfo, lockErr := readPrintLock(filepath.Join(item.DirectoryPath, entry.Name()), item.SessionID)
				if lockErr != nil {
					item.Reason = firstReason(item.Reason, "The printer submission lock metadata is invalid; inspect the session manually.")
				} else {
					item.PrintLock = &lockInfo
					owner, ownerErr := InspectPrintLockOwner(filepath.Join(item.DirectoryPath, entry.Name()), item.SessionID)
					if ownerErr != nil {
						item.PrintLockOwner = PrintLockOwnerUnknown
						item.Reason = firstReason(item.Reason, "The printer lock owner could not be verified safely.")
					} else {
						item.PrintLockOwner = owner
					}
				}
			}
		}
		if entry.Name() == FrameSnapshotFileName {
			info, statErr := os.Lstat(filepath.Join(item.DirectoryPath, entry.Name()))
			if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxFrameSnapshotBytes {
				item.Reason = firstReason(item.Reason, "The frame snapshot is not a bounded regular file.")
			}
		}
	}

	item.PrintLockPresent = printLockPresent
	manifest, manifestErr := ReadManifest(item.DirectoryPath)
	if manifestErr != nil {
		if errors.Is(manifestErr, os.ErrNotExist) {
			item.Reason = "This session has no recovery manifest (legacy or interrupted before manifest publication). It is available for inspection only."
		} else {
			item.Reason = "The recovery manifest is missing, corrupt or unsupported. The session was not modified."
		}
	} else {
		item.Manifest = &manifest
		item.PrintAttempts = append([]PrintAttempt(nil), manifest.PrintAttempts...)
		item.PrintResolutions = append([]PrintResolution(nil), manifest.PrintResolutions...)
		item.Phase = manifest.Phase
		item.ExpectedShots = manifest.ExpectedShots
		item.CompletedShots = manifest.CompletedShots
		frameSnapshotPath := filepath.Join(item.DirectoryPath, FrameSnapshotFileName)
		if manifest.FrameSnapshotFile == "" {
			if _, snapshotErr := os.Lstat(frameSnapshotPath); snapshotErr == nil {
				item.Reason = firstReason(item.Reason, "A frame snapshot exists but is not declared by the recovery manifest.")
			} else if !errors.Is(snapshotErr, os.ErrNotExist) {
				item.Reason = firstReason(item.Reason, "The frame snapshot could not be inspected safely.")
			}
		} else {
			path, _, snapshotErr := ReadFrameSnapshot(item.DirectoryPath, manifest)
			if snapshotErr != nil {
				item.Reason = firstReason(item.Reason, "The immutable frame snapshot is missing, corrupt or does not match its manifest.")
			} else {
				item.FrameConfigPath = path
			}
		}
	}

	captureDir := filepath.Join(item.DirectoryPath, "captures")
	if err := stationconfig.ValidateStorageChild(item.DirectoryPath, captureDir); err != nil {
		item.Reason = firstReason(item.Reason, "The captures directory resolves through a redirected path.")
		return
	}
	captureInfo, err := os.Lstat(captureDir)
	if err != nil || !captureInfo.IsDir() || captureInfo.Mode()&os.ModeSymlink != 0 {
		item.Reason = firstReason(item.Reason, "The captures directory is missing or is not a real directory.")
		return
	}
	captureEntries, tooManyCaptures, err := readDirectoryLimited(captureDir, maxCaptureDirectoryEntries)
	if err != nil {
		item.Reason = firstReason(item.Reason, "The captures directory could not be read.")
		return
	}
	if tooManyCaptures {
		item.Reason = firstReason(item.Reason, "The captures directory contains more than 12 committed photos plus the optional staging directory.")
		return
	}
	actualCaptures := 0
	for _, capture := range captureEntries {
		if capture.Name() == CaptureStagingDirectoryName {
			continue
		}
		actualCaptures++
		expected := fmt.Sprintf("capture-%02d.jpg", actualCaptures)
		if capture.Name() != expected {
			item.Reason = firstReason(item.Reason, "Capture files are not a complete, sequential set.")
			return
		}
		path := filepath.Join(captureDir, capture.Name())
		if err := ValidateJPEGHeader(path); err != nil {
			item.Reason = firstReason(item.Reason, fmt.Sprintf("Capture %d is not a valid, bounded local JPEG.", actualCaptures))
			return
		}
		item.CapturePaths = append(item.CapturePaths, path)
	}
	_, stagingErr := ValidateCaptureStagingDirectory(captureDir)
	if stagingErr != nil {
		item.Reason = firstReason(item.Reason, "The interrupted capture staging area contains an unsafe or unknown layout.")
		return
	}
	if item.Manifest != nil && actualCaptures != item.Manifest.CompletedShots {
		// A crash can occur after the complete JPEG was atomically published but
		// before the manifest progress update. Allow exactly that one durable
		// capture to be reconciled only after the operator explicitly continues.
		if item.Manifest.Phase != PhaseCapturing || actualCaptures != item.Manifest.CompletedShots+1 {
			item.Reason = firstReason(item.Reason, "The capture count does not match the durable manifest. The session is available for inspection only.")
			return
		}
	}
	item.CompletedShots = actualCaptures

	renderedPath := filepath.Join(item.DirectoryPath, "rendered.jpg")
	_, renderedErr := os.Lstat(renderedPath)
	if renderedErr != nil && !errors.Is(renderedErr, os.ErrNotExist) {
		item.Reason = firstReason(item.Reason, "The rendered file could not be inspected safely.")
		return
	}
	hasRendered := renderedErr == nil
	if hasRendered {
		if jpegErr := ValidateJPEGHeader(renderedPath); jpegErr != nil {
			item.Reason = firstReason(item.Reason, "The rendered file is not a valid, bounded local JPEG.")
			return
		}
		item.RenderedPath = renderedPath
	}

	if item.Manifest == nil || item.Reason != "" {
		return
	}
	m := item.Manifest
	switch m.Phase {
	case PhasePreflight:
		if actualCaptures != 0 || hasRendered {
			item.Reason = "Preflight phase conflicts with files present in the session. The session is available for inspection only."
			return
		}
		item.Status = RecoveryResumeAvailable
	case PhaseCapturing:
		if hasRendered {
			item.Reason = "A rendered file exists before the manifest reached the rendered phase. The session is available for inspection only."
			return
		}
		item.Status = RecoveryResumeAvailable
	case PhaseRendered:
		if actualCaptures != m.ExpectedShots || !hasRendered {
			item.Reason = "Rendered phase does not match the preserved captures/rendered file. The session is available for inspection only."
			return
		}
		item.Status = RecoveryOutputPending
	case PhaseOutputFinalized:
		if actualCaptures != m.ExpectedShots || !hasRendered {
			item.Reason = "Output-finalized phase does not match the preserved session files. The session is available for inspection only."
			return
		}
		item.Status = RecoveryFinalized
	}
	if item.Status != RecoveryInspectOnly {
		if printLockPresent {
			item.Reason = firstReason(item.Reason, "A printer submission lock is present. Verify its kernel owner state before any resolution or retry.")
		}
		if len(item.PrintAttempts) > 0 {
			last := item.PrintAttempts[len(item.PrintAttempts)-1]
			if resolution, resolved := PrintResolutionFor(*m, last.AttemptID); resolved {
				switch resolution.Decision {
				case PrintResolutionPrinted:
					item.Reason = firstReason(item.Reason, "An operator recorded that the physical print was observed. This attempt is closed; do not resubmit it.")
				case PrintResolutionNotPrinted:
					item.Reason = firstReason(item.Reason, "An operator recorded that no physical print was observed and the job was absent from the queue. A new attempt requires a separate explicit operator action.")
				case PrintResolutionUnknown:
					item.Reason = firstReason(item.Reason, "An operator closed this attempt with an unknown outcome. It remains blocked from resubmission.")
				}
			} else {
				switch last.Status {
				case PrintStatusSubmitting:
					item.Reason = firstReason(item.Reason, "A printer submission was interrupted before its result was persisted. Treat it as uncertain; check the Windows queue by document name before taking any action.")
				case PrintStatusQueued:
					item.Reason = firstReason(item.Reason, fmt.Sprintf("Windows accepted printer job %s on queue %s. This is not proof of physical printing; do not resubmit automatically.", last.JobID, last.QueueName))
				case PrintStatusUncertain:
					item.Reason = firstReason(item.Reason, "The printer submission outcome is uncertain. Check the Windows queue by document name before any manual retry.")
				case PrintStatusFailed:
					item.Reason = firstReason(item.Reason, "The last printer attempt was rejected before spooler acceptance. A later retry must be an explicit operator action.")
				}
			}
		}
	}
}

// ValidateJPEGHeader performs a bounded header-only check for startup discovery.
// It deliberately avoids decoding every guest photo while the idle screen loads.
func ValidateJPEGHeader(path string) error {
	file, err := openBoundedJPEG(path)
	if err != nil {
		return err
	}
	return file.Close()
}

// ValidateJPEGFile verifies the complete compressed JPEG payload. Resume calls
// this before opening a camera so a truncated/corrupt preserved capture cannot
// cause the operator to take replacement photos unnecessarily.
func ValidateJPEGFile(path string) error {
	file, err := openBoundedJPEG(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind JPEG before full decode: %w", err)
	}
	if _, err := jpeg.Decode(file); err != nil {
		return fmt.Errorf("decode complete JPEG payload: %w", err)
	}
	return nil
}

func openBoundedJPEG(path string) (*os.File, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxRecoveryJPEGBytes {
		return nil, errors.New("JPEG path is not a bounded, non-empty regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	header, err := jpeg.DecodeConfig(file)
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("read JPEG header: %w", err)
	}
	if header.Width < 1 || header.Height < 1 || header.Width > maxRecoveryJPEGDimension ||
		header.Height > maxRecoveryJPEGDimension || int64(header.Width)*int64(header.Height) > maxRecoveryJPEGPixels {
		_ = file.Close()
		return nil, fmt.Errorf("JPEG dimensions %dx%d exceed recovery limits", header.Width, header.Height)
	}
	return file, nil
}

func sameRecoveryPath(left, right string) bool {
	left, right = filepath.Clean(left), filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

// readDirectoryLimited reads at most limit+1 entries so callers can fail closed
// on oversized directories without allocating an unbounded directory listing.
func readDirectoryLimited(path string, limit int) ([]os.DirEntry, bool, error) {
	if limit < 0 {
		return nil, false, errors.New("directory entry limit cannot be negative")
	}
	directory, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	entries, readErr := directory.ReadDir(limit + 1)
	closeErr := directory.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return nil, false, errors.Join(readErr, closeErr)
	}
	if closeErr != nil {
		return nil, false, closeErr
	}
	tooMany := len(entries) > limit
	if tooMany {
		entries = entries[:limit]
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, tooMany, nil
}

func firstReason(current, next string) string {
	if current != "" {
		return current
	}
	return next
}

func validSessionID(value string) bool {
	if len(value) != 32 {
		return false
	}
	for _, r := range value {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
