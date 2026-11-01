// Package framepackage validates, stores and activates declarative PhotoBooth
// frame packages. Schema v1 deliberately supports no scripts, themes, arbitrary
// assets or remote resources.
package framepackage

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dongsinhho/PhotoBooth/internal/booth/render"
)

const (
	ArchiveExtension           = ".pbframe"
	PackageSchemaVersion       = 1
	maxArchiveBytes      int64 = 16 << 20
	maxManifestBytes     int64 = 64 << 10
	maxDefinitionBytes   int64 = 1 << 20
	maxPreviewBytes      int64 = 4 << 20
	maxLicenseBytes      int64 = 64 << 10
	maxPayloadBytes      int64 = maxManifestBytes + maxDefinitionBytes + maxPreviewBytes + maxLicenseBytes
	maxArchiveEntries          = 4
)

var ErrNoRollbackAvailable = errors.New("no rollback is available for the currently active frame")

// Manifest is the fixed schema-v1 frame package contract. Every payload must be
// declared; the package has no execution entry point and never loads remote data.
type Manifest struct {
	SchemaVersion  int          `json:"schema_version"`
	PackageType    string       `json:"package_type"`
	PackageID      string       `json:"package_id"`
	PackageVersion string       `json:"package_version"`
	Name           string       `json:"name"`
	Description    string       `json:"description,omitempty"`
	Author         string       `json:"author"`
	License        string       `json:"license"`
	EntryPoint     string       `json:"entrypoint"`
	Files          []FileRecord `json:"files"`
}

type FileRecord struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	MediaType string `json:"media_type"`
}

// PackageInfo contains verified metadata suitable for display by the operator UI.
type PackageInfo struct {
	PackageID        string `json:"package_id"`
	PackageVersion   string `json:"package_version"`
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	Author           string `json:"author"`
	License          string `json:"license"`
	FramePath        string `json:"frame_path"`
	PreviewPath      string `json:"preview_path,omitempty"`
	SlotCount        int    `json:"slot_count"`
	HasPreview       bool   `json:"has_preview"`
	AlreadyInstalled bool   `json:"already_installed,omitempty"`
}

type Manager struct{ framesRoot string }

type parsedPackage struct {
	manifest      Manifest
	manifestBytes []byte
	payloads      map[string][]byte
	slots         int
}

type rollbackState struct {
	SchemaVersion      int       `json:"schema_version"`
	TargetFramePath    string    `json:"target_frame_path"`
	PreviousSourcePath string    `json:"previous_source_path"`
	BackupFileName     string    `json:"backup_file_name"`
	PreviousSHA256     string    `json:"previous_sha256"`
	PackageID          string    `json:"package_id"`
	PackageVersion     string    `json:"package_version"`
	CreatedAt          time.Time `json:"created_at"`
}

func NewManager(framesRoot string) *Manager {
	if strings.TrimSpace(framesRoot) == "" {
		return &Manager{}
	}
	absolute, err := filepath.Abs(framesRoot)
	if err != nil {
		return &Manager{}
	}
	return &Manager{framesRoot: filepath.Clean(absolute)}
}

// Validate fully inspects a package but writes nothing.
func (m *Manager) Validate(ctx context.Context, archivePath string) (PackageInfo, error) {
	parsed, err := readArchive(ctx, archivePath)
	if err != nil {
		return PackageInfo{}, err
	}
	return infoFromManifest(parsed.manifest, parsed.payloads, parsed.slots, ""), nil
}

// Install validates the package, stages every file, then atomically renames the
// staged version into packages/<package-id>/<version>. Installation never activates it.
func (m *Manager) Install(ctx context.Context, archivePath string) (PackageInfo, error) {
	parsed, err := readArchive(ctx, archivePath)
	if err != nil {
		return PackageInfo{}, err
	}
	if err := m.ensureFramesRoot(); err != nil {
		return PackageInfo{}, err
	}
	packagesRoot := filepath.Join(m.framesRoot, "packages")
	if err := ensureDirectory(packagesRoot); err != nil {
		return PackageInfo{}, fmt.Errorf("prepare package directory: %w", err)
	}
	idRoot := filepath.Join(packagesRoot, parsed.manifest.PackageID)
	if err := ensureDirectory(idRoot); err != nil {
		return PackageInfo{}, fmt.Errorf("prepare package id directory: %w", err)
	}
	finalPath := filepath.Join(idRoot, parsed.manifest.PackageVersion)
	if existing, statErr := os.Lstat(finalPath); statErr == nil {
		if existing.Mode()&os.ModeSymlink != 0 || !existing.IsDir() {
			return PackageInfo{}, errors.New("package destination exists and is not a regular directory")
		}
		info, manifestBytes, err := inspectInstalledDir(ctx, finalPath)
		if err != nil {
			return PackageInfo{}, fmt.Errorf("installed version exists but is invalid; refusing overwrite: %w", err)
		}
		if !bytes.Equal(manifestBytes, parsed.manifestBytes) {
			return PackageInfo{}, fmt.Errorf("package %s version %s already exists with different manifest data; versions are immutable", parsed.manifest.PackageID, parsed.manifest.PackageVersion)
		}
		info.AlreadyInstalled = true
		return info, nil
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return PackageInfo{}, fmt.Errorf("inspect package destination: %w", statErr)
	}

	stage, err := os.MkdirTemp(idRoot, ".staging-*")
	if err != nil {
		return PackageInfo{}, fmt.Errorf("create staging directory: %w", err)
	}
	defer os.RemoveAll(stage)
	if err := writeNewFile(filepath.Join(stage, "manifest.json"), parsed.manifestBytes, 0o600); err != nil {
		return PackageInfo{}, fmt.Errorf("stage manifest: %w", err)
	}
	for name, data := range parsed.payloads {
		if err := ctx.Err(); err != nil {
			return PackageInfo{}, err
		}
		if err := writeNewFile(filepath.Join(stage, name), data, 0o600); err != nil {
			return PackageInfo{}, fmt.Errorf("stage package file %q: %w", name, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return PackageInfo{}, err
	}
	if err := os.Rename(stage, finalPath); err != nil {
		return PackageInfo{}, fmt.Errorf("activate staged package version: %w", err)
	}
	info := infoFromManifest(parsed.manifest, parsed.payloads, parsed.slots, filepath.Join(finalPath, "definition.json"))
	if name := previewName(parsed.payloads); name != "" {
		info.PreviewPath = filepath.Join(finalPath, name)
	}
	return info, nil
}

// List re-verifies installed versions and returns metadata in stable order.
// A corrupt installed package is reported as an error rather than silently hidden.
func (m *Manager) List(ctx context.Context) ([]PackageInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if m == nil || m.framesRoot == "" {
		return nil, errors.New("frame package storage root is invalid")
	}
	rootInfo, err := os.Lstat(m.framesRoot)
	if errors.Is(err, os.ErrNotExist) {
		return []PackageInfo{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect managed frame root: %w", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return nil, errors.New("managed frame root must be a regular directory, not a symlink")
	}
	root := filepath.Join(m.framesRoot, "packages")
	rootInfo, err = os.Lstat(root)
	if errors.Is(err, os.ErrNotExist) {
		return []PackageInfo{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect installed package root: %w", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return nil, errors.New("installed package root must be a regular directory, not a symlink")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("list installed packages: %w", err)
	}
	var result []PackageInfo
	for _, idEntry := range entries {
		if strings.HasPrefix(idEntry.Name(), ".") || !validPackageID(idEntry.Name()) {
			continue
		}
		idPath := filepath.Join(root, idEntry.Name())
		if err := rejectSymlinkEntry(idEntry, idPath); err != nil {
			return nil, err
		}
		if !idEntry.IsDir() {
			continue
		}
		versions, err := os.ReadDir(idPath)
		if err != nil {
			return nil, fmt.Errorf("list versions for %q: %w", idEntry.Name(), err)
		}
		for _, versionEntry := range versions {
			if strings.HasPrefix(versionEntry.Name(), ".") || !validVersion(versionEntry.Name()) {
				continue
			}
			versionPath := filepath.Join(idPath, versionEntry.Name())
			if err := rejectSymlinkEntry(versionEntry, versionPath); err != nil {
				return nil, err
			}
			if !versionEntry.IsDir() {
				continue
			}
			info, _, err := inspectInstalledDir(ctx, versionPath)
			if err != nil {
				return nil, fmt.Errorf("installed package %s/%s failed verification: %w", idEntry.Name(), versionEntry.Name(), err)
			}
			if info.PackageID != idEntry.Name() || info.PackageVersion != versionEntry.Name() {
				return nil, fmt.Errorf("directory %s/%s does not match manifest identity", idEntry.Name(), versionEntry.Name())
			}
			result = append(result, info)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		if result[i].PackageID != result[j].PackageID {
			return result[i].PackageID < result[j].PackageID
		}
		return result[i].PackageVersion < result[j].PackageVersion
	})
	return result, nil
}

// Uninstall removes an immutable package version only when it is not active,
// pending activation, or referenced by the currently committed rollback record.
// The caller must pass the station profile's current active frame path.
func (m *Manager) Uninstall(ctx context.Context, id, version, currentFramePath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m == nil || m.framesRoot == "" {
		return errors.New("frame package storage root is invalid")
	}
	if !validPackageID(id) || !validVersion(version) {
		return errors.New("invalid package ID or version")
	}
	current, err := absoluteRequiredPath(currentFramePath)
	if err != nil {
		return err
	}

	// Verify every managed directory component before inspecting/deleting a version.
	// This prevents following a package-root or ID-directory symlink.
	for _, dir := range []string{
		m.framesRoot,
		filepath.Join(m.framesRoot, "packages"),
		filepath.Join(m.framesRoot, "packages", id),
	} {
		info, statErr := os.Lstat(dir)
		if statErr != nil {
			return fmt.Errorf("inspect package directory %q: %w", dir, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("package directory %q must be a regular directory, not a symlink", dir)
		}
	}

	info, err := m.findInstalled(ctx, id, version)
	if err != nil {
		return err
	}
	target, err := absoluteRequiredPath(info.FramePath)
	if err != nil {
		return err
	}
	if pathsEqual(current, target) {
		return errors.New("cannot uninstall the currently active frame package")
	}

	// If an activation is between PrepareActivation and Commit/Abort, do not
	// remove either endpoint. The caller can retry after the operation resolves.
	pending, pendingErr := m.readPendingState()
	if pendingErr == nil {
		if (pending.PackageID == id && pending.PackageVersion == version) ||
			pathsEqual(target, pending.TargetFramePath) ||
			pathsEqual(target, pending.PreviousSourcePath) {
			return errors.New("cannot uninstall a package referenced by an activation that is awaiting commit or abort")
		}
	} else if !errors.Is(pendingErr, os.ErrNotExist) {
		return fmt.Errorf("cannot safely uninstall while pending activation state is invalid: %w", pendingErr)
	}

	// Keep both ends of the last-known-good transition available until rollback
	// is cleared or a later activation replaces this record.
	rollback, rollbackErr := m.loadRollbackState()
	if rollbackErr == nil {
		if (rollback.PackageID == id && rollback.PackageVersion == version) ||
			pathsEqual(target, rollback.TargetFramePath) ||
			pathsEqual(target, rollback.PreviousSourcePath) {
			return errors.New("cannot uninstall a package referenced by rollback state; activate another frame or clear rollback first")
		}
	} else if !errors.Is(rollbackErr, os.ErrNotExist) {
		return fmt.Errorf("cannot safely uninstall while rollback state is invalid: %w", rollbackErr)
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	versionDir := filepath.Join(m.framesRoot, "packages", id, version)
	// Re-verify immediately before removal to avoid deleting a version that was
	// replaced or corrupted after the first validation.
	if _, _, err := inspectInstalledDir(ctx, versionDir); err != nil {
		return fmt.Errorf("re-verify package before uninstall: %w", err)
	}
	if err := os.RemoveAll(versionDir); err != nil {
		return fmt.Errorf("remove installed package %s/%s: %w", id, version, err)
	}
	return nil
}

// PrepareActivation saves the current frame as a durable rollback point before
// the caller atomically persists a settings profile pointing to the returned frame.
// Rollback is offered only while the current profile still points at that target.
func (m *Manager) PrepareActivation(ctx context.Context, id, version, currentFramePath string) (PackageInfo, error) {
	info, err := m.findInstalled(ctx, id, version)
	if err != nil {
		return PackageInfo{}, err
	}
	current, err := absoluteRequiredPath(currentFramePath)
	if err != nil {
		return PackageInfo{}, err
	}
	target, err := filepath.Abs(info.FramePath)
	if err != nil {
		return PackageInfo{}, err
	}
	if err := m.finalizePendingActivation(current); err != nil {
		return PackageInfo{}, err
	}
	if pathsEqual(current, target) {
		return PackageInfo{}, errors.New("this package version is already active")
	}
	if err := validateFrameFile(ctx, current); err != nil {
		return PackageInfo{}, fmt.Errorf("cannot save rollback point from current frame: %w", err)
	}
	currentBytes, err := readRegularFile(current, maxDefinitionBytes)
	if err != nil {
		return PackageInfo{}, fmt.Errorf("read current frame for rollback: %w", err)
	}
	if err := m.ensureFramesRoot(); err != nil {
		return PackageInfo{}, err
	}
	rollbackDir := filepath.Join(m.framesRoot, "rollback")
	if err := ensureDirectory(rollbackDir); err != nil {
		return PackageInfo{}, fmt.Errorf("prepare rollback storage: %w", err)
	}
	if _, err := os.Lstat(filepath.Join(rollbackDir, "pending-v1.json")); err == nil {
		return PackageInfo{}, errors.New("another frame activation is awaiting commit or abort")
	} else if !errors.Is(err, os.ErrNotExist) {
		return PackageInfo{}, err
	}
	backup, err := uniqueFilePath(rollbackDir)
	if err != nil {
		return PackageInfo{}, err
	}
	if err := writeNewFile(backup, currentBytes, 0o600); err != nil {
		return PackageInfo{}, fmt.Errorf("write previous frame copy: %w", err)
	}
	sum := sha256.Sum256(currentBytes)
	state := rollbackState{
		SchemaVersion: 1, TargetFramePath: target, PreviousSourcePath: current,
		BackupFileName: filepath.Base(backup), PreviousSHA256: hex.EncodeToString(sum[:]),
		PackageID: info.PackageID, PackageVersion: info.PackageVersion, CreatedAt: time.Now().UTC(),
	}
	stateBytes, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		_ = os.Remove(backup)
		return PackageInfo{}, err
	}
	if err := writeAtomic(filepath.Join(rollbackDir, "pending-v1.json"), stateBytes, 0o600); err != nil {
		_ = os.Remove(backup)
		return PackageInfo{}, fmt.Errorf("persist pending rollback state: %w", err)
	}
	return info, nil
}

// CommitActivation publishes the pending rollback state only after the caller
// has saved the station profile pointing at the new frame.
func (m *Manager) CommitActivation(targetFramePath string) error {
	target, err := absoluteRequiredPath(targetFramePath)
	if err != nil {
		return err
	}
	state, err := m.readPendingState()
	if err != nil {
		return err
	}
	if !pathsEqual(target, state.TargetFramePath) {
		return errors.New("pending activation target does not match the saved station profile")
	}
	if err := m.publishPendingState(state); err != nil {
		return err
	}
	return nil
}

// AbortActivation discards a pending activation if station-profile persistence
// fails. The prior committed rollback state remains intact.
func (m *Manager) AbortActivation() error {
	state, err := m.readPendingState()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	dir := filepath.Join(m.framesRoot, "rollback")
	if err := os.Remove(filepath.Join(dir, "pending-v1.json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if filepath.Base(state.BackupFileName) == state.BackupFileName {
		_ = os.Remove(filepath.Join(dir, state.BackupFileName))
	}
	return nil
}

// CanRollback validates rollback metadata and its backup without creating any files.
// It also completes/discards a pending activation left by an interrupted process.
func (m *Manager) CanRollback(ctx context.Context, currentFramePath string) (bool, error) {
	current, err := absoluteRequiredPath(currentFramePath)
	if err != nil {
		return false, err
	}
	if err := m.finalizePendingActivation(current); err != nil {
		return false, err
	}
	state, err := m.loadRollbackState()
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !pathsEqual(current, state.TargetFramePath) {
		return false, nil
	}
	backup := filepath.Join(m.framesRoot, "rollback", state.BackupFileName)
	if err := rejectSymlinkPath(backup); err != nil {
		return false, err
	}
	data, err := readRegularFile(backup, maxDefinitionBytes)
	if err != nil {
		return false, err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != state.PreviousSHA256 {
		return false, errors.New("rollback frame SHA-256 verification failed")
	}
	if _, err := validateFrameBytes(ctx, data); err != nil {
		return false, fmt.Errorf("rollback frame is invalid: %w", err)
	}
	return true, nil
}

func checkedRollbackDirectory(framesRoot string) (string, error) {
	if strings.TrimSpace(framesRoot) == "" {
		return "", errors.New("frame package storage root is invalid")
	}
	rootInfo, err := os.Lstat(framesRoot)
	if err != nil {
		return "", err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return "", errors.New("managed frame root must be a regular directory, not a symlink")
	}
	dir := filepath.Join(framesRoot, "rollback")
	dirInfo, err := os.Lstat(dir)
	if err != nil {
		return dir, err
	}
	if dirInfo.Mode()&os.ModeSymlink != 0 || !dirInfo.IsDir() {
		return "", errors.New("rollback state directory must be a regular directory, not a symlink")
	}
	return dir, nil
}

func (m *Manager) readPendingState() (rollbackState, error) {
	var state rollbackState
	if m == nil || m.framesRoot == "" {
		return state, errors.New("frame package storage root is invalid")
	}
	dir, err := checkedRollbackDirectory(m.framesRoot)
	if err != nil {
		return state, err
	}
	data, err := readRegularFile(filepath.Join(dir, "pending-v1.json"), 16<<10)
	if err != nil {
		return state, err
	}
	if err := decodeStrictJSON(data, &state); err != nil {
		return state, err
	}
	if err := validateRollbackState(state); err != nil {
		return state, err
	}
	return state, nil
}

func (m *Manager) publishPendingState(state rollbackState) error {
	dir := filepath.Join(m.framesRoot, "rollback")
	oldState, _ := m.loadRollbackState()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(dir, "state-v1.json"), data, 0o600); err != nil {
		return fmt.Errorf("publish rollback state: %w", err)
	}
	if err := os.Remove(filepath.Join(dir, "pending-v1.json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if oldState.BackupFileName != "" && oldState.BackupFileName != state.BackupFileName && filepath.Base(oldState.BackupFileName) == oldState.BackupFileName {
		_ = os.Remove(filepath.Join(dir, oldState.BackupFileName))
	}
	return nil
}

func (m *Manager) finalizePendingActivation(currentFramePath string) error {
	state, err := m.readPendingState()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect pending frame activation: %w", err)
	}
	dir := filepath.Join(m.framesRoot, "rollback")
	if pathsEqual(currentFramePath, state.TargetFramePath) {
		return m.publishPendingState(state)
	}
	if err := os.Remove(filepath.Join(dir, "pending-v1.json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if filepath.Base(state.BackupFileName) == state.BackupFileName {
		_ = os.Remove(filepath.Join(dir, state.BackupFileName))
	}
	return nil
}

// RollbackFramePath verifies the previous frame and copies it to a stable restored
// path. The rollback backup itself is not used as the active frame, so cleanup
// cannot delete the active frame file.
func (m *Manager) RollbackFramePath(ctx context.Context, currentFramePath string) (string, error) {
	current, pathErr := absoluteRequiredPath(currentFramePath)
	if pathErr != nil {
		return "", pathErr
	}
	if err := m.finalizePendingActivation(current); err != nil {
		return "", err
	}
	state, err := m.loadRollbackState()
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNoRollbackAvailable
	}
	if err != nil {
		return "", fmt.Errorf("read rollback state: %w", err)
	}
	if !pathsEqual(current, state.TargetFramePath) {
		return "", ErrNoRollbackAvailable
	}
	rollbackDir := filepath.Join(m.framesRoot, "rollback")
	backup := filepath.Join(rollbackDir, state.BackupFileName)
	if err := rejectSymlinkPath(backup); err != nil {
		return "", err
	}
	data, err := readRegularFile(backup, maxDefinitionBytes)
	if err != nil {
		return "", fmt.Errorf("read rollback frame: %w", err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != state.PreviousSHA256 {
		return "", errors.New("rollback frame SHA-256 verification failed; refusing rollback")
	}
	if _, err := validateFrameBytes(ctx, data); err != nil {
		return "", fmt.Errorf("rollback frame is invalid: %w", err)
	}
	restoredDir := filepath.Join(m.framesRoot, "restored")
	if err := ensureDirectory(restoredDir); err != nil {
		return "", err
	}
	path, err := uniqueFilePath(restoredDir)
	if err != nil {
		return "", err
	}
	if err := writeNewFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write restored frame: %w", err)
	}
	return path, nil
}

// ClearRollback removes only the rollback record/backup, not any active frame.
func (m *Manager) ClearRollback() error {
	state, err := m.loadRollbackState()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	dir := filepath.Join(m.framesRoot, "rollback")
	if err := os.Remove(filepath.Join(dir, "state-v1.json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove rollback state: %w", err)
	}
	if state.BackupFileName != "" && filepath.Base(state.BackupFileName) == state.BackupFileName {
		_ = os.Remove(filepath.Join(dir, state.BackupFileName))
	}
	return nil
}

func readArchive(ctx context.Context, path string) (parsedPackage, error) {
	var empty parsedPackage
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return empty, errors.New("frame package path is required")
	}
	if !strings.EqualFold(filepath.Ext(path), ArchiveExtension) {
		return empty, fmt.Errorf("frame package must use the %s extension", ArchiveExtension)
	}
	archiveBytes, err := readRegularFile(path, maxArchiveBytes)
	if err != nil {
		return empty, fmt.Errorf("read frame package: %w", err)
	}
	// Parse the exact bytes whose file identity and size were validated; reopening
	// the path here would allow a same-name replacement between check and use.
	zr, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		return empty, fmt.Errorf("open frame package ZIP: %w", err)
	}
	if len(zr.File) < 2 || len(zr.File) > maxArchiveEntries {
		return empty, fmt.Errorf("package must contain 2 to %d files", maxArchiveEntries)
	}
	files := make(map[string][]byte, len(zr.File))
	var total int64
	for _, item := range zr.File {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		name := item.Name
		if !validArchivePath(name) {
			return empty, fmt.Errorf("unsafe or unsupported archive path %q", name)
		}
		if _, exists := files[name]; exists {
			return empty, fmt.Errorf("duplicate archive entry %q", name)
		}
		mode := item.Mode()
		if mode&os.ModeSymlink != 0 || !mode.IsRegular() {
			return empty, fmt.Errorf("archive entry %q is not a regular file", name)
		}
		if item.Method != zip.Store && item.Method != zip.Deflate {
			return empty, fmt.Errorf("archive entry %q uses unsupported compression", name)
		}
		limit, _ := maxBytesForPath(name)
		if item.UncompressedSize64 == 0 || item.UncompressedSize64 > uint64(limit) {
			return empty, fmt.Errorf("archive entry %q exceeds size limit", name)
		}
		total += int64(item.UncompressedSize64)
		if total > maxPayloadBytes {
			return empty, errors.New("package uncompressed size exceeds limits")
		}
		handle, err := item.Open()
		if err != nil {
			return empty, err
		}
		data, readErr := io.ReadAll(io.LimitReader(handle, limit+1))
		closeErr := handle.Close()
		if readErr != nil {
			return empty, fmt.Errorf("read package entry %q: %w", name, readErr)
		}
		if closeErr != nil {
			return empty, fmt.Errorf("close package entry %q: %w", name, closeErr)
		}
		if uint64(len(data)) != item.UncompressedSize64 || int64(len(data)) > limit {
			return empty, fmt.Errorf("archive entry %q has inconsistent size", name)
		}
		files[name] = data
	}
	rawManifest, ok := files["manifest.json"]
	if !ok {
		return empty, errors.New("package is missing manifest.json")
	}
	var manifest Manifest
	if err := decodeStrictJSON(rawManifest, &manifest); err != nil {
		return empty, fmt.Errorf("decode package manifest: %w", err)
	}
	if err := validateManifest(manifest); err != nil {
		return empty, err
	}
	payloads := make(map[string][]byte, len(files)-1)
	for name, data := range files {
		if name != "manifest.json" {
			payloads[name] = data
		}
	}
	if err := verifyPayloads(manifest, payloads); err != nil {
		return empty, err
	}
	if err := validateOptionalPayloads(payloads); err != nil {
		return empty, err
	}
	slots, err := validateFrameBytes(ctx, payloads["definition.json"])
	if err != nil {
		return empty, fmt.Errorf("validate frame definition: %w", err)
	}
	return parsedPackage{manifest: manifest, manifestBytes: rawManifest, payloads: payloads, slots: slots}, nil
}

func validateManifest(m Manifest) error {
	if m.SchemaVersion != PackageSchemaVersion {
		return fmt.Errorf("package schema_version must be %d; got %d", PackageSchemaVersion, m.SchemaVersion)
	}
	if m.PackageType != "frame" {
		return fmt.Errorf("unsupported package_type %q; schema v1 supports frame packages only", m.PackageType)
	}
	if !validPackageID(m.PackageID) {
		return errors.New("package_id must contain 1–63 lowercase letters, digits or hyphens and start with a letter/digit")
	}
	if !validVersion(m.PackageVersion) {
		return errors.New("package_version must use numeric major.minor.patch form, such as 1.0.0")
	}
	if strings.TrimSpace(m.Name) == "" || !utf8.ValidString(m.Name) || len(m.Name) > 120 {
		return errors.New("package name must be valid UTF-8 and 1–120 bytes")
	}
	if !utf8.ValidString(m.Description) || len(m.Description) > 1000 {
		return errors.New("package description must be valid UTF-8 and at most 1000 bytes")
	}
	if strings.TrimSpace(m.Author) == "" || !utf8.ValidString(m.Author) || len(m.Author) > 100 {
		return errors.New("package author must be valid UTF-8 and 1–100 bytes")
	}
	if strings.TrimSpace(m.License) == "" || !utf8.ValidString(m.License) || len(m.License) > 100 {
		return errors.New("package license is required and must be at most 100 bytes")
	}
	if m.EntryPoint != "definition.json" {
		return errors.New("entrypoint must be definition.json")
	}
	if len(m.Files) < 1 || len(m.Files) > 3 {
		return errors.New("manifest must declare definition.json and at most one preview plus one LICENSE.txt")
	}
	seen := make(map[string]bool, len(m.Files))
	hasDefinition := false
	hasPNGPreview, hasJPEGPreview := false, false
	for _, f := range m.Files {
		if !validPayloadPath(f.Path) {
			return fmt.Errorf("unsupported package file path %q", f.Path)
		}
		if seen[f.Path] {
			return fmt.Errorf("duplicate file record %q", f.Path)
		}
		seen[f.Path] = true
		limit, _ := maxBytesForPath(f.Path)
		if f.Size <= 0 || f.Size > limit {
			return fmt.Errorf("manifest size for %q is outside limits", f.Path)
		}
		if len(f.SHA256) != 64 || !isLowerHex(f.SHA256) {
			return fmt.Errorf("SHA-256 for %q must be 64 lowercase hexadecimal characters", f.Path)
		}
		if f.MediaType != mediaTypeForPath(f.Path) {
			return fmt.Errorf("media_type for %q must be %q", f.Path, mediaTypeForPath(f.Path))
		}
		if f.Path == "definition.json" {
			hasDefinition = true
		}
		if f.Path == "preview.png" {
			hasPNGPreview = true
		}
		if f.Path == "preview.jpg" {
			hasJPEGPreview = true
		}
	}
	if hasPNGPreview && hasJPEGPreview {
		return errors.New("package may contain only one preview image")
	}
	if !hasDefinition {
		return errors.New("manifest must declare definition.json")
	}
	return nil
}

func verifyPayloads(m Manifest, payloads map[string][]byte) error {
	if len(payloads) != len(m.Files) {
		return errors.New("package contains undeclared or missing payload files")
	}
	for _, f := range m.Files {
		data, ok := payloads[f.Path]
		if !ok {
			return fmt.Errorf("package is missing declared file %q", f.Path)
		}
		if int64(len(data)) != f.Size {
			return fmt.Errorf("size mismatch for %q", f.Path)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != f.SHA256 {
			return fmt.Errorf("SHA-256 mismatch for %q", f.Path)
		}
	}
	return nil
}

func validateOptionalPayloads(payloads map[string][]byte) error {
	for name, data := range payloads {
		switch name {
		case "definition.json":
		case "preview.png", "preview.jpg":
			cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
			if err != nil {
				return fmt.Errorf("decode preview %q: %w", name, err)
			}
			expected := strings.TrimPrefix(filepath.Ext(name), ".")
			if expected == "jpg" {
				expected = "jpeg"
			}
			if format != expected {
				return fmt.Errorf("preview %q content is %q, not %q", name, format, expected)
			}
			if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 4000 || cfg.Height > 4000 || int64(cfg.Width)*int64(cfg.Height) > 16_000_000 {
				return fmt.Errorf("preview %q dimensions exceed limits", name)
			}
			// Decode the whole image, not just its header, so truncated or
			// malformed payloads do not get installed as valid previews.
			decoded, decodedFormat, err := image.Decode(bytes.NewReader(data))
			if err != nil {
				return fmt.Errorf("decode preview %q: %w", name, err)
			}
			if decodedFormat != expected || decoded.Bounds().Dx() != cfg.Width || decoded.Bounds().Dy() != cfg.Height {
				return fmt.Errorf("preview %q data does not match its validated header", name)
			}
		case "LICENSE.txt":
			if !utf8.Valid(data) {
				return errors.New("LICENSE.txt must be UTF-8 text")
			}
		default:
			return fmt.Errorf("unsupported package file %q", name)
		}
	}
	return nil
}

func validateFrameBytes(ctx context.Context, data []byte) (int, error) {
	if len(data) == 0 || int64(len(data)) > maxDefinitionBytes {
		return 0, fmt.Errorf("frame definition must be 1 byte to %d bytes", maxDefinitionBytes)
	}
	dir, err := os.MkdirTemp("", "photobooth-frame-check-*")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "definition.json")
	if err := writeNewFile(path, data, 0o600); err != nil {
		return 0, err
	}
	r := render.New()
	if err := r.Validate(ctx, path); err != nil {
		return 0, err
	}
	return r.SlotCount(ctx, path)
}
func validateFrameFile(ctx context.Context, path string) error {
	data, err := readRegularFile(path, maxDefinitionBytes)
	if err != nil {
		return err
	}
	_, err = validateFrameBytes(ctx, data)
	return err
}

func inspectInstalledDir(ctx context.Context, dir string) (PackageInfo, []byte, error) {
	if err := rejectSymlinkPath(dir); err != nil {
		return PackageInfo{}, nil, err
	}
	raw, err := readRegularFile(filepath.Join(dir, "manifest.json"), maxManifestBytes)
	if err != nil {
		return PackageInfo{}, nil, fmt.Errorf("read installed manifest: %w", err)
	}
	var m Manifest
	if err := decodeStrictJSON(raw, &m); err != nil {
		return PackageInfo{}, nil, err
	}
	if err := validateManifest(m); err != nil {
		return PackageInfo{}, nil, err
	}
	expected := map[string]bool{"manifest.json": true}
	payloads := make(map[string][]byte, len(m.Files))
	for _, f := range m.Files {
		expected[f.Path] = true
		data, err := readRegularFile(filepath.Join(dir, f.Path), mustFileLimit(f.Path))
		if err != nil {
			return PackageInfo{}, nil, fmt.Errorf("read installed file %q: %w", f.Path, err)
		}
		payloads[f.Path] = data
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return PackageInfo{}, nil, err
	}
	for _, e := range entries {
		if !expected[e.Name()] {
			return PackageInfo{}, nil, fmt.Errorf("installed version contains unexpected entry %q", e.Name())
		}
		if err := rejectSymlinkEntry(e, filepath.Join(dir, e.Name())); err != nil {
			return PackageInfo{}, nil, err
		}
	}
	if err := verifyPayloads(m, payloads); err != nil {
		return PackageInfo{}, nil, err
	}
	if err := validateOptionalPayloads(payloads); err != nil {
		return PackageInfo{}, nil, err
	}
	slots, err := validateFrameBytes(ctx, payloads["definition.json"])
	if err != nil {
		return PackageInfo{}, nil, err
	}
	info := infoFromManifest(m, payloads, slots, filepath.Join(dir, "definition.json"))
	if preview := previewName(payloads); preview != "" {
		info.PreviewPath = filepath.Join(dir, preview)
	}
	return info, raw, nil
}

func (m *Manager) findInstalled(ctx context.Context, id, version string) (PackageInfo, error) {
	if err := ctx.Err(); err != nil {
		return PackageInfo{}, err
	}
	if m == nil || m.framesRoot == "" {
		return PackageInfo{}, errors.New("frame package storage root is invalid")
	}
	if !validPackageID(id) || !validVersion(version) {
		return PackageInfo{}, errors.New("invalid package ID or version")
	}
	packageRoot := filepath.Join(m.framesRoot, "packages")
	idRoot := filepath.Join(packageRoot, id)
	versionRoot := filepath.Join(idRoot, version)
	for _, dir := range []string{m.framesRoot, packageRoot, idRoot, versionRoot} {
		info, statErr := os.Lstat(dir)
		if statErr != nil {
			return PackageInfo{}, fmt.Errorf("inspect installed package directory %q: %w", dir, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return PackageInfo{}, fmt.Errorf("installed package directory %q must be a regular directory, not a symlink", dir)
		}
	}
	info, _, err := inspectInstalledDir(ctx, versionRoot)
	if err != nil {
		return PackageInfo{}, fmt.Errorf("load installed package %s/%s: %w", id, version, err)
	}
	if info.PackageID != id || info.PackageVersion != version {
		return PackageInfo{}, errors.New("installed package identity mismatch")
	}
	return info, nil
}

func (m *Manager) loadRollbackState() (rollbackState, error) {
	var state rollbackState
	if m == nil || m.framesRoot == "" {
		return state, errors.New("frame package storage root is invalid")
	}
	dir, err := checkedRollbackDirectory(m.framesRoot)
	if err != nil {
		return state, err
	}
	raw, err := readRegularFile(filepath.Join(dir, "state-v1.json"), 16<<10)
	if err != nil {
		return state, err
	}
	if err := decodeStrictJSON(raw, &state); err != nil {
		return state, err
	}
	if err := validateRollbackState(state); err != nil {
		return state, err
	}
	return state, nil
}

func validateRollbackState(state rollbackState) error {
	if state.SchemaVersion != 1 || !validPackageID(state.PackageID) || !validVersion(state.PackageVersion) ||
		len(state.PreviousSHA256) != 64 || !isLowerHex(state.PreviousSHA256) ||
		strings.TrimSpace(state.TargetFramePath) == "" || strings.TrimSpace(state.PreviousSourcePath) == "" ||
		state.BackupFileName == "" || filepath.Base(state.BackupFileName) != state.BackupFileName {
		return errors.New("frame rollback state is invalid")
	}
	return nil
}

func (m *Manager) ensureFramesRoot() error {
	if m == nil || m.framesRoot == "" {
		return errors.New("frame package storage root is invalid")
	}
	return ensureDirectory(m.framesRoot)
}
func ensureDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("managed directory %q cannot be a symlink or non-directory", path)
	}
	return nil
}
func rejectSymlinkPath(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("managed path %q cannot be a symlink", path)
	}
	return nil
}
func rejectSymlinkEntry(entry os.DirEntry, path string) error {
	info, err := entry.Info()
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("managed path %q cannot be a symlink", path)
	}
	return nil
}
func writeNewFile(path string, data []byte, mode fs.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}
func writeAtomic(path string, data []byte, mode fs.FileMode) error {
	if err := ensureDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".frame-state-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(mode); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
func uniqueFilePath(dir string) (string, error) {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate file ID: %w", err)
	}
	return filepath.Join(dir, fmt.Sprintf("frame-%d-%s.json", time.Now().UTC().UnixNano(), hex.EncodeToString(random))), nil
}
func absoluteRequiredPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("frame path is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(absolute), nil
}
func pathsEqual(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
func readRegularFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, errors.New("path must be a regular file, not a symlink")
	}
	if info.Size() <= 0 || info.Size() > limit {
		return nil, fmt.Errorf("file size must be 1 byte to %d bytes", limit)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return nil, errors.New("file identity changed while being opened")
	}
	if openedInfo.Size() <= 0 || openedInfo.Size() > limit {
		return nil, fmt.Errorf("file size must be 1 byte to %d bytes", limit)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) != openedInfo.Size() || int64(len(data)) > limit {
		return nil, errors.New("file changed size while being read")
	}
	return data, nil
}
func decodeStrictJSON(data []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("JSON must contain one document")
		}
		return err
	}
	return nil
}
func infoFromManifest(m Manifest, payloads map[string][]byte, slots int, path string) PackageInfo {
	return PackageInfo{PackageID: m.PackageID, PackageVersion: m.PackageVersion, Name: m.Name, Description: m.Description, Author: m.Author, License: m.License, FramePath: path, SlotCount: slots, HasPreview: previewName(payloads) != ""}
}
func validPackageID(id string) bool {
	if len(id) < 1 || len(id) > 63 || !isLowerAlphaNum(id[0]) || id[0] == '-' {
		return false
	}
	for _, ch := range id {
		if !(ch >= 'a' && ch <= 'z') && !(ch >= '0' && ch <= '9') && ch != '-' {
			return false
		}
	}
	return true
}
func isLowerAlphaNum(ch byte) bool { return (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') }
func validVersion(v string) bool {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" || (len(p) > 1 && p[0] == '0') {
			return false
		}
		for i := 0; i < len(p); i++ {
			if p[i] < '0' || p[i] > '9' {
				return false
			}
		}
		if _, err := strconv.ParseUint(p, 10, 32); err != nil {
			return false
		}
	}
	return true
}
func isLowerHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
func validArchivePath(path string) bool { return path == "manifest.json" || validPayloadPath(path) }
func validPayloadPath(path string) bool {
	switch path {
	case "definition.json", "preview.png", "preview.jpg", "LICENSE.txt":
		return true
	default:
		return false
	}
}
func maxBytesForPath(path string) (int64, bool) {
	switch path {
	case "manifest.json":
		return maxManifestBytes, true
	case "definition.json":
		return maxDefinitionBytes, true
	case "preview.png", "preview.jpg":
		return maxPreviewBytes, true
	case "LICENSE.txt":
		return maxLicenseBytes, true
	default:
		return 0, false
	}
}
func mustFileLimit(path string) int64 { n, _ := maxBytesForPath(path); return n }
func mediaTypeForPath(path string) string {
	switch path {
	case "definition.json":
		return "application/json"
	case "preview.png":
		return "image/png"
	case "preview.jpg":
		return "image/jpeg"
	case "LICENSE.txt":
		return "text/plain"
	default:
		return ""
	}
}
func previewName(files map[string][]byte) string {
	if _, ok := files["preview.png"]; ok {
		return "preview.png"
	}
	if _, ok := files["preview.jpg"]; ok {
		return "preview.jpg"
	}
	return ""
}
