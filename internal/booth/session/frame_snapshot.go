package session

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
)

const (
	FrameSnapshotFileName = "frame.json"
	maxFrameSnapshotBytes = 1 << 20
)

// WriteFrameSnapshot stores the exact validated frame definition used by a
// session. It is published atomically without replacing an existing file.
func WriteFrameSnapshot(sessionDir string, data []byte) (string, string, error) {
	if len(data) == 0 || len(data) > maxFrameSnapshotBytes {
		return "", "", fmt.Errorf("frame snapshot must be between 1 byte and %d bytes", maxFrameSnapshotBytes)
	}
	if err := stationconfig.ValidateStorageChild(filepath.Dir(sessionDir), sessionDir); err != nil {
		return "", "", fmt.Errorf("validate frame snapshot session identity: %w", err)
	}
	dirInfo, err := os.Lstat(sessionDir)
	if err != nil {
		return "", "", err
	}
	if dirInfo.Mode()&os.ModeSymlink != 0 || !dirInfo.IsDir() {
		return "", "", errors.New("frame snapshot parent must be a real session directory")
	}

	finalPath := filepath.Join(sessionDir, FrameSnapshotFileName)
	if _, err := os.Lstat(finalPath); err == nil {
		return "", "", errors.New("frame snapshot already exists; refusing to replace it")
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", "", fmt.Errorf("inspect frame snapshot destination: %w", err)
	}

	temp, err := os.CreateTemp(sessionDir, ".frame-*.tmp")
	if err != nil {
		return "", "", fmt.Errorf("create temporary frame snapshot: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return "", "", fmt.Errorf("secure temporary frame snapshot: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return "", "", fmt.Errorf("write temporary frame snapshot: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return "", "", fmt.Errorf("sync temporary frame snapshot: %w", err)
	}
	if err := temp.Close(); err != nil {
		return "", "", fmt.Errorf("close temporary frame snapshot: %w", err)
	}
	if err := publishFrameSnapshotNoReplace(tempPath, finalPath); err != nil {
		return "", "", fmt.Errorf("publish frame snapshot without replacement: %w", err)
	}

	digest := sha256.Sum256(data)
	return finalPath, hex.EncodeToString(digest[:]), nil
}

// ReadFrameSnapshot validates the declared, bounded snapshot and returns its
// exact bytes. It never falls back to the mutable original frame path.
func ReadFrameSnapshot(sessionDir string, manifest Manifest) (string, []byte, error) {
	if manifest.Version < PolicyManifestVersion || manifest.Version > ManifestVersion || manifest.FrameSnapshotFile != FrameSnapshotFileName ||
		!validSHA256(manifest.FrameSnapshotSHA256) ||
		manifest.FrameSnapshotSHA256 != manifest.FrameConfigSHA256 {
		return "", nil, errors.New("manifest frame snapshot declaration is invalid")
	}
	path := filepath.Join(sessionDir, FrameSnapshotFileName)
	if err := stationconfig.ValidateStorageChild(sessionDir, path); err != nil {
		return "", nil, fmt.Errorf("validate frame snapshot path identity: %w", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() ||
		info.Size() <= 0 || info.Size() > maxFrameSnapshotBytes {
		return "", nil, errors.New("frame snapshot is not a bounded regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return "", nil, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return "", nil, errors.New("frame snapshot identity changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxFrameSnapshotBytes+1))
	if err != nil {
		return "", nil, fmt.Errorf("read frame snapshot: %w", err)
	}
	if len(data) == 0 || len(data) > maxFrameSnapshotBytes {
		return "", nil, errors.New("frame snapshot size is outside the supported limit")
	}
	digest := sha256.Sum256(data)
	actual := hex.EncodeToString(digest[:])
	if actual != manifest.FrameSnapshotSHA256 || actual != manifest.FrameConfigSHA256 {
		return "", nil, errors.New("frame snapshot SHA-256 does not match the durable manifest")
	}
	return path, data, nil
}
