//go:build !windows

package session

import (
	"fmt"
	"os"
	"path/filepath"
)

func publishFrameSnapshotNoReplace(source, destination string) error {
	// A same-directory hard link gives atomic no-replace publication on Unix.
	if err := os.Link(source, destination); err != nil {
		return err
	}
	if err := os.Remove(source); err != nil {
		return fmt.Errorf("remove staged frame snapshot name: %w", err)
	}
	directory, err := os.Open(filepath.Dir(destination))
	if err != nil {
		return err
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}
