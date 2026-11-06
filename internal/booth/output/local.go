package output

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
)

var ErrInsufficientStorage = errors.New("insufficient storage space")

type Provider interface {
	Save(ctx context.Context, sessionID, sourcePath string) (string, error)
}

type LocalProvider struct {
	RootDir             string
	ops                 localFileOps
	validateStoragePath func(string) error // injectable for deterministic network-storage tests
}

func NewLocal(root string) *LocalProvider { return &LocalProvider{RootDir: root} }

func (p *LocalProvider) Name() string { return TargetLocalStorage }

func (p *LocalProvider) Execute(ctx context.Context, job Job) (TargetOutput, error) {
	// A dispatcher is publishing a finalized session artifact. Never replace
	// an existing photo for the same session identity.
	path, err := p.SaveExclusive(ctx, job.SessionID, job.SourcePath)
	return TargetOutput{OutputPath: path}, err
}

// Dispatch preserves the simple local-only mode while using the same target
// reporting contract as a multi-target coordinator.
func (p *LocalProvider) Dispatch(ctx context.Context, job Job) (DispatchResult, error) {
	return NewCoordinator(p).Dispatch(ctx, job)
}

// Save copies only the specified rendered image into a session-scoped directory.
// The final photo.jpg is published only after the temporary copy has been
// written, synced and closed successfully. A failed copy never replaces it.
func (p *LocalProvider) Save(ctx context.Context, sessionID, sourcePath string) (string, error) {
	return p.save(ctx, sessionID, sourcePath, false)
}

// SaveExclusive publishes the completed temporary file only when photo.jpg
// does not exist. A same-directory hard link gives an atomic no-replace
// operation on supported local filesystems, including Unix where Rename would
// otherwise replace an existing photo.
func (p *LocalProvider) SaveExclusive(ctx context.Context, sessionID, sourcePath string) (string, error) {
	return p.save(ctx, sessionID, sourcePath, true)
}

func (p *LocalProvider) save(ctx context.Context, sessionID, sourcePath string, exclusive bool) (savedPath string, saveErr error) {
	if ctx == nil {
		return "", errors.New("output save context is required")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if p == nil || strings.TrimSpace(p.RootDir) == "" {
		return "", errors.New("local output root directory is required")
	}
	if !validSessionID(sessionID) {
		return "", errors.New("session ID must be a 32-character lowercase hexadecimal identifier")
	}
	if err := p.validateRoot(p.RootDir); err != nil {
		return "", fmt.Errorf("validate local output root: %w", err)
	}
	ops := p.fileOps()
	info, err := ops.Stat(sourcePath)
	if err != nil {
		return "", fmt.Errorf("stat rendered output: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() < 1 {
		return "", errors.New("rendered output must be a non-empty regular file")
	}

	outDir := filepath.Join(p.RootDir, sessionID)
	if err := stationconfig.ValidateStorageChild(p.RootDir, outDir); err != nil {
		return "", fmt.Errorf("validate output session identity: %w", err)
	}
	if err := ops.MkdirAll(outDir, 0o700); err != nil {
		return "", storageOperationError("create session output directory", err)
	}
	// Re-resolve the now-existing session directory immediately before creating
	// any temporary photo bytes. This catches an existing junction/symlink or
	// mount target that was not present during configuration load.
	if err := stationconfig.ValidateStorageChild(p.RootDir, outDir); err != nil {
		return "", fmt.Errorf("revalidate output session identity: %w", err)
	}
	if err := p.validateRoot(outDir); err != nil {
		return "", fmt.Errorf("validate local session output directory: %w", err)
	}
	finalPath := filepath.Join(outDir, "photo.jpg")
	temp, err := ops.CreateTemp(outDir, ".photo-*.tmp")
	if err != nil {
		return "", storageOperationError("create temporary output", err)
	}
	tempPath := temp.Name()
	tempClosed := false
	publishTemp := true
	defer func() {
		if !tempClosed {
			if closeErr := temp.Close(); closeErr != nil {
				saveErr = errors.Join(saveErr, storageOperationError("close partial temporary output", closeErr))
			}
		}
		if publishTemp {
			if removeErr := ops.Remove(tempPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				saveErr = errors.Join(saveErr, fmt.Errorf("remove partial temporary output %q: %w", tempPath, removeErr))
			}
		}
		if saveErr != nil {
			savedPath = ""
		}
	}()

	src, err := ops.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("open rendered output: %w", err)
	}
	defer src.Close()

	if _, err := io.Copy(temp, &contextReader{ctx: ctx, reader: src}); err != nil {
		return "", storageOperationError("write temporary output", err)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := temp.Sync(); err != nil {
		return "", storageOperationError("sync temporary output", err)
	}
	if err := temp.Close(); err != nil {
		tempClosed = true
		return "", storageOperationError("close temporary output", err)
	}
	tempClosed = true

	if exclusive {
		if err := ops.Link(tempPath, finalPath); err != nil {
			return "", storageOperationError("publish local output without overwriting", err)
		}
		// The final name now references the fully synced file. Removing the
		// temporary name is best effort; failure here must not misreport a
		// successfully published photo as unsaved.
		publishTemp = false
		_ = ops.Remove(tempPath)
		return finalPath, nil
	}
	if err := ops.Rename(tempPath, finalPath); err != nil {
		return "", storageOperationError("finalize local output", err)
	}
	publishTemp = false
	return finalPath, nil
}

func (p *LocalProvider) validateRoot(path string) error {
	if p.validateStoragePath != nil {
		return p.validateStoragePath(path)
	}
	return stationconfig.ValidateLocalStoragePath(path)
}

func (p *LocalProvider) fileOps() localFileOps {
	if p.ops != nil {
		return p.ops
	}
	return osLocalFileOps{}
}

type localTempFile interface {
	io.Writer
	Name() string
	Sync() error
	Close() error
}

type localSourceFile interface {
	io.Reader
	Close() error
}

type localFileOps interface {
	Stat(name string) (os.FileInfo, error)
	MkdirAll(path string, perm fs.FileMode) error
	CreateTemp(dir, pattern string) (localTempFile, error)
	Open(name string) (localSourceFile, error)
	Rename(oldPath, newPath string) error
	Link(oldPath, newPath string) error
	Remove(path string) error
}

type osLocalFileOps struct{}

func (osLocalFileOps) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (osLocalFileOps) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (osLocalFileOps) CreateTemp(dir, pattern string) (localTempFile, error) {
	return os.CreateTemp(dir, pattern)
}

func (osLocalFileOps) Open(name string) (localSourceFile, error) {
	return os.Open(name)
}

func (osLocalFileOps) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

func (osLocalFileOps) Link(oldPath, newPath string) error {
	return os.Link(oldPath, newPath)
}

func (osLocalFileOps) Remove(path string) error {
	return os.Remove(path)
}

func storageOperationError(operation string, err error) error {
	if isInsufficientStorageError(err) {
		return fmt.Errorf("%w during %s: %w; free disk space or storage quota, verify the destination volume, then retry. Captures and the rendered session file are preserved.", ErrInsufficientStorage, operation, err)
	}
	return fmt.Errorf("%s: %w", operation, err)
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(b []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(b)
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
