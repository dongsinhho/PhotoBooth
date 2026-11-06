package output

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
)

// RetryLocalFromSession validates a completed session's preserved captures and
// rendered image, then retries only the local save. It never opens a camera or
// invokes the renderer. Existing output is never intentionally overwritten.
func RetryLocalFromSession(ctx context.Context, sessionsRoot, outputRoot, sessionID string, captureCount int) (string, error) {
	if ctx == nil {
		return "", errors.New("output retry context is required")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !validSessionID(sessionID) {
		return "", errors.New("session ID must be a 32-character lowercase hexadecimal identifier")
	}
	if captureCount < 1 || captureCount > 12 {
		return "", errors.New("capture count must be between 1 and 12")
	}
	sessionsRoot = strings.TrimSpace(sessionsRoot)
	outputRoot = strings.TrimSpace(outputRoot)
	if sessionsRoot == "" || outputRoot == "" {
		return "", errors.New("sessions and output directories are required")
	}

	sessionsRoot, err := filepath.Abs(sessionsRoot)
	if err != nil {
		return "", fmt.Errorf("resolve sessions root: %w", err)
	}
	outputRoot, err = filepath.Abs(outputRoot)
	if err != nil {
		return "", fmt.Errorf("resolve output root: %w", err)
	}
	if err := stationconfig.ValidateLocalStoragePath(sessionsRoot); err != nil {
		return "", fmt.Errorf("validate sessions root for output retry: %w", err)
	}
	if err := stationconfig.ValidateLocalStoragePath(outputRoot); err != nil {
		return "", fmt.Errorf("validate output root for retry: %w", err)
	}
	if err := stationconfig.ValidateDistinctStorageRoots(sessionsRoot, outputRoot); err != nil {
		return "", fmt.Errorf("refuse output retry: %w", err)
	}
	if err := requireRetryDirectory(sessionsRoot, "sessions root"); err != nil {
		return "", err
	}

	sessionDir := filepath.Join(sessionsRoot, sessionID)
	if err := stationconfig.ValidateStorageChild(sessionsRoot, sessionDir); err != nil {
		return "", fmt.Errorf("validate session identity for output retry: %w", err)
	}
	if err := requireRetryDirectory(sessionDir, "session directory"); err != nil {
		return "", err
	}
	captureDir := filepath.Join(sessionDir, "captures")
	if err := stationconfig.ValidateStorageChild(sessionDir, captureDir); err != nil {
		return "", fmt.Errorf("validate capture directory for output retry: %w", err)
	}
	if err := requireRetryDirectory(captureDir, "capture directory"); err != nil {
		return "", err
	}
	for shot := 1; shot <= captureCount; shot++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		path := filepath.Join(captureDir, fmt.Sprintf("capture-%02d.jpg", shot))
		if err := requireRetryFile(path, fmt.Sprintf("capture %d", shot)); err != nil {
			return "", err
		}
	}

	renderedPath := filepath.Join(sessionDir, "rendered.jpg")
	if err := requireRetryFile(renderedPath, "rendered session image"); err != nil {
		return "", err
	}
	if err := ensureRetryOutputAvailable(outputRoot, sessionID); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return NewLocal(outputRoot).SaveExclusive(ctx, sessionID, renderedPath)
}

func requireRetryDirectory(path, label string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect %s %q: %w", label, path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%s %q must be a real directory, not a symlink or file", label, path)
	}
	return nil
}

func requireRetryFile(path, label string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect %s %q: %w", label, path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("%s %q must be a non-empty regular file, not a symlink", label, path)
	}
	return nil
}

func ensureRetryOutputAvailable(outputRoot, sessionID string) error {
	rootInfo, err := os.Lstat(outputRoot)
	if err == nil {
		if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
			return fmt.Errorf("output root %q must be a real directory", outputRoot)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect output root %q: %w", outputRoot, err)
	}

	sessionOutput := filepath.Join(outputRoot, sessionID)
	info, err := os.Lstat(sessionOutput)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("session output path %q is not a real directory", sessionOutput)
		}
		finalPath := filepath.Join(sessionOutput, "photo.jpg")
		if _, err := os.Lstat(finalPath); err == nil {
			return fmt.Errorf("output %q already exists; refusing to overwrite a previously saved photo", finalPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect existing output %q: %w", finalPath, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect session output %q: %w", sessionOutput, err)
	}
	return nil
}
