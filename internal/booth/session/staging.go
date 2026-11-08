package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
)

// ValidateCaptureStagingDirectory checks the narrowly defined temporary layout
// left by an interrupted capture. Discovery may inspect it, but never removes it.
func ValidateCaptureStagingDirectory(capturesDir string) (bool, error) {
	stagingDir := filepath.Join(capturesDir, CaptureStagingDirectoryName)
	info, err := os.Lstat(stagingDir)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect capture staging directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return false, errors.New("capture staging path must be a real directory")
	}
	if err := stationconfig.ValidateStorageChild(capturesDir, stagingDir); err != nil {
		return false, fmt.Errorf("validate capture staging identity: %w", err)
	}
	attempts, tooMany, err := readDirectoryLimited(stagingDir, maxCaptureStagingEntries)
	if err != nil {
		return false, fmt.Errorf("read capture staging directory: %w", err)
	}
	if tooMany {
		return false, fmt.Errorf("capture staging directory contains more than %d attempts", maxCaptureStagingEntries)
	}
	for _, attempt := range attempts {
		if !validCaptureStagingAttemptName(attempt.Name()) {
			return false, fmt.Errorf("unexpected capture staging entry %q", attempt.Name())
		}
		attemptDir := filepath.Join(stagingDir, attempt.Name())
		if err := stationconfig.ValidateStorageChild(stagingDir, attemptDir); err != nil {
			return false, fmt.Errorf("validate capture attempt identity: %w", err)
		}
		attemptInfo, err := os.Lstat(attemptDir)
		if err != nil {
			return false, fmt.Errorf("inspect capture attempt %q: %w", attempt.Name(), err)
		}
		if attemptInfo.Mode()&os.ModeSymlink != 0 || !attemptInfo.IsDir() {
			return false, fmt.Errorf("capture attempt %q is not a real directory", attempt.Name())
		}
		files, tooManyFiles, err := readDirectoryLimited(attemptDir, 3)
		if err != nil {
			return false, fmt.Errorf("read capture attempt %q: %w", attempt.Name(), err)
		}
		if tooManyFiles {
			return false, fmt.Errorf("capture attempt %q contains too many temporary files", attempt.Name())
		}
		for _, file := range files {
			if !validCaptureStagingArtifactName(file.Name()) {
				return false, fmt.Errorf("unexpected capture staging artifact %q", file.Name())
			}
			filePath := filepath.Join(attemptDir, file.Name())
			fileInfo, err := os.Lstat(filePath)
			if err != nil {
				return false, fmt.Errorf("inspect capture staging artifact %q: %w", file.Name(), err)
			}
			if fileInfo.Mode()&os.ModeSymlink != 0 || !fileInfo.Mode().IsRegular() || fileInfo.Size() > maxRecoveryJPEGBytes {
				return false, fmt.Errorf("capture staging artifact %q is not a bounded regular file", file.Name())
			}
		}
	}
	return true, nil
}

// CleanupCaptureStagingDirectory is called only after an operator explicitly
// chooses Continue. Unknown layouts are preserved and cause a fail-closed error.
func CleanupCaptureStagingDirectory(capturesDir string) error {
	exists, err := ValidateCaptureStagingDirectory(capturesDir)
	if err != nil || !exists {
		return err
	}
	stagingDir := filepath.Join(capturesDir, CaptureStagingDirectoryName)
	if err := stationconfig.ValidateStorageChild(capturesDir, stagingDir); err != nil {
		return fmt.Errorf("revalidate capture staging identity before cleanup: %w", err)
	}
	info, err := os.Lstat(stagingDir)
	if err != nil {
		return fmt.Errorf("reinspect capture staging directory before cleanup: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("capture staging directory changed before cleanup")
	}
	if err := os.RemoveAll(stagingDir); err != nil {
		return fmt.Errorf("remove interrupted capture staging artifacts: %w", err)
	}
	return nil
}

func validCaptureStagingAttemptName(name string) bool {
	// shot-NN-<32 lowercase hexadecimal chars>
	if len(name) != len("shot-01-")+32 || !strings.HasPrefix(name, "shot-") || name[7] != '-' {
		return false
	}
	shot := name[5:7]
	if len(shot) != 2 || shot[0] < '0' || shot[0] > '9' || shot[1] < '0' || shot[1] > '9' {
		return false
	}
	shotNumber := int(shot[0]-'0')*10 + int(shot[1]-'0')
	return shotNumber >= 1 && shotNumber <= 12 && validStagingToken(name[8:])
}

func validStagingToken(token string) bool {
	if len(token) != 32 {
		return false
	}
	for _, c := range token {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func validCaptureStagingArtifactName(name string) bool {
	if name == "capture.jpg" {
		return true
	}
	for _, prefix := range []string{".photobooth-capture-", ".fake-capture-"} {
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".tmp") && len(name) > len(prefix)+len(".tmp") {
			return true
		}
	}
	return false
}
