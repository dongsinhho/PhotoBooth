package output

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Provider interface {
	Save(ctx context.Context, sessionID, sourcePath string) (string, error)
}

type LocalProvider struct{ RootDir string }

func NewLocal(root string) *LocalProvider { return &LocalProvider{RootDir: root} }

// Save writes only the specified rendered image into a session-scoped directory.
func (p *LocalProvider) Save(ctx context.Context, sessionID, sourcePath string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if p == nil || strings.TrimSpace(p.RootDir) == "" {
		return "", errors.New("local output root directory is required")
	}
	if !validSessionID(sessionID) {
		return "", errors.New("session ID must be a 32-character lowercase hexadecimal identifier")
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		return "", fmt.Errorf("stat rendered output: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() < 1 {
		return "", errors.New("rendered output must be a non-empty regular file")
	}

	outDir := filepath.Join(p.RootDir, sessionID)
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", fmt.Errorf("create session output directory: %w", err)
	}
	finalPath := filepath.Join(outDir, "photo.jpg")
	temp, err := os.CreateTemp(outDir, ".photo-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temporary output: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	src, err := os.Open(sourcePath)
	if err != nil {
		_ = temp.Close()
		return "", fmt.Errorf("open rendered output: %w", err)
	}
	defer src.Close()
	if _, err := io.Copy(temp, &contextReader{ctx: ctx, reader: src}); err != nil {
		_ = temp.Close()
		return "", fmt.Errorf("copy rendered output: %w", err)
	}
	if err := ctx.Err(); err != nil {
		_ = temp.Close()
		return "", err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return "", fmt.Errorf("sync output: %w", err)
	}
	if err := temp.Close(); err != nil {
		return "", fmt.Errorf("close output: %w", err)
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return "", fmt.Errorf("finalize output: %w", err)
	}
	return finalPath, nil
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
