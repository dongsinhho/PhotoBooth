//go:build !windows

package workflow

import (
	"fmt"
	"os"
	"path/filepath"
)

func publishCaptureNoReplace(stagedPath, finalPath string) error {
	info, err := os.Lstat(stagedPath)
	if err != nil {
		return fmt.Errorf("inspect staged capture: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 {
		return fmt.Errorf("staged capture must be a non-empty regular file")
	}
	// Link is atomic and fails if finalPath already exists. Unlike Rename on
	// Unix, it never replaces an existing committed guest photo.
	if err := os.Link(stagedPath, finalPath); err != nil {
		return fmt.Errorf("publish capture without replacement: %w", err)
	}
	if err := syncCaptureDirectory(filepath.Dir(finalPath)); err != nil {
		return fmt.Errorf("sync published capture directory: %w", err)
	}
	if err := os.Remove(stagedPath); err != nil {
		return fmt.Errorf("remove staged capture after publication: %w", err)
	}
	if err := syncCaptureDirectory(filepath.Dir(stagedPath)); err != nil {
		return fmt.Errorf("sync capture staging directory: %w", err)
	}
	return nil
}

func syncCaptureDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	syncErr := dir.Sync()
	closeErr := dir.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}
