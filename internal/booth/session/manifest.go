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

	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
)

const (
	ManifestFileName            = "manifest.json"
	LegacyManifestVersion       = 1
	ManifestVersion             = 2
	CaptureStagingDirectoryName = ".staging"
	maxManifestBytes            = 64 << 10
	maxRecoveryRootEntries      = 4096
	maxSessionDirectoryEntries  = 4  // manifest, captures, optional frame snapshot and rendered.jpg
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
	Version               int       `json:"version"`
	SessionID             string    `json:"session_id"`
	Phase                 Phase     `json:"phase"`
	ExpectedShots         int       `json:"expected_shots"`
	CompletedShots        int       `json:"completed_shots"`
	FrameConfigPath       string    `json:"frame_config_path"`
	FrameConfigSHA256     string    `json:"frame_config_sha256"`
	FrameSnapshotFile     string    `json:"frame_snapshot_file,omitempty"`
	FrameSnapshotSHA256   string    `json:"frame_snapshot_sha256,omitempty"`
	RunConfigFingerprint  string    `json:"run_config_fingerprint"`
	ConfigSchemaVersion   int       `json:"config_schema_version"`
	CameraProvider        string    `json:"camera_provider,omitempty"`
	CameraDevice          string    `json:"camera_device,omitempty"`
	CaptureWidth          int       `json:"capture_width,omitempty"`
	CaptureHeight         int       `json:"capture_height,omitempty"`
	CountdownMilliseconds int64     `json:"countdown_ms,omitempty"`
	IntervalMilliseconds  int64     `json:"interval_ms,omitempty"`
	DemoMode              bool      `json:"demo_mode,omitempty"`
	MediaDeliveryMode     string    `json:"media_delivery_mode,omitempty"`
	PaymentMode           string    `json:"payment_mode,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
	RenderedAt            time.Time `json:"rendered_at,omitempty"`
	OutputFinalizedAt     time.Time `json:"output_finalized_at,omitempty"`
	OutputPath            string    `json:"output_path,omitempty"`
}

type RecoveryStatus string

const (
	RecoveryResumeAvailable RecoveryStatus = "resume_available"
	RecoveryOutputPending   RecoveryStatus = "output_pending"
	RecoveryFinalized       RecoveryStatus = "finalized"
	RecoveryInspectOnly     RecoveryStatus = "inspect_only"
)

type RecoveryItem struct {
	SessionID       string         `json:"session_id"`
	DirectoryPath   string         `json:"directory_path"`
	Status          RecoveryStatus `json:"status"`
	Reason          string         `json:"reason,omitempty"`
	CompletedShots  int            `json:"completed_shots"`
	ExpectedShots   int            `json:"expected_shots"`
	Phase           Phase          `json:"phase,omitempty"`
	Manifest        *Manifest      `json:"manifest,omitempty"`
	FrameConfigPath string         `json:"frame_config_path,omitempty"`
	CapturePaths    []string       `json:"capture_paths,omitempty"`
	RenderedPath    string         `json:"rendered_path,omitempty"`
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
	if m.Version != LegacyManifestVersion && m.Version != ManifestVersion {
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
	if m.Version == ManifestVersion && m.FrameSnapshotFile == "" {
		return errors.New("version-2 session manifest requires an immutable frame snapshot")
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
	if m.Version == ManifestVersion {
		if m.MediaDeliveryMode != stationconfig.MediaDeliveryModeLocal {
			return fmt.Errorf("manifest media_delivery_mode %q is not supported", m.MediaDeliveryMode)
		}
		if m.PaymentMode != stationconfig.PaymentModeDisabled {
			return fmt.Errorf("manifest payment_mode %q is not supported", m.PaymentMode)
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
	return nil
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
		ManifestFileName: true, "captures": true, "rendered.jpg": true, FrameSnapshotFileName: true,
	}
	for _, entry := range entries {
		if !known[entry.Name()] {
			item.Reason = "The session contains unexpected files or directories. It is available for inspection only."
			break
		}
		if entry.Name() == FrameSnapshotFileName {
			info, statErr := os.Lstat(filepath.Join(item.DirectoryPath, entry.Name()))
			if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxFrameSnapshotBytes {
				item.Reason = firstReason(item.Reason, "The frame snapshot is not a bounded regular file.")
			}
		}
	}

	manifest, manifestErr := ReadManifest(item.DirectoryPath)
	if manifestErr != nil {
		if errors.Is(manifestErr, os.ErrNotExist) {
			item.Reason = "This session has no recovery manifest (legacy or interrupted before manifest publication). It is available for inspection only."
		} else {
			item.Reason = "The recovery manifest is missing, corrupt or unsupported. The session was not modified."
		}
	} else {
		item.Manifest = &manifest
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
