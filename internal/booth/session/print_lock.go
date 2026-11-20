package session

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	PrintLockLeaseVersion = 1
	PrintLockMaxBytes     = 512

	PrintLockOwnerActive       = "active"
	PrintLockOwnerOrphaned     = "orphaned"
	PrintLockOwnerUnverifiable = "unverifiable"
	PrintLockOwnerUnknown      = "unknown"
)

var (
	ErrPrintLockActive      = errors.New("printer lock is held by an active process")
	ErrPrintLockLegacy      = errors.New("printer lock does not contain a verifiable kernel lease")
	ErrPrintLockUnsupported = errors.New("this platform cannot safely claim an orphaned printer lock")
)

type PrintLockLease struct {
	file *os.File
	path string
	Info PrintLockInfo
}

// CreatePrintLockLease creates the durable identity marker and holds an OS-level
// lease until Close or Remove. On Windows, the handle permits read-only
// inspection but denies another process the write/delete access needed to claim
// an orphaned lock. A process exit releases that kernel handle automatically.
func CreatePrintLockLease(path, sessionID string, info PrintLockInfo) (*PrintLockLease, error) {
	if !validSessionID(sessionID) {
		return nil, errors.New("printer lock session ID is invalid")
	}
	info.LeaseVersion = currentPrintLockLeaseVersion()
	if info.OwnerPID == 0 {
		info.OwnerPID = os.Getpid()
	}
	if err := validatePrintLockInfo(info, sessionID); err != nil {
		return nil, err
	}
	data, err := json.Marshal(info)
	if err != nil {
		return nil, fmt.Errorf("encode printer lock metadata: %w", err)
	}
	if len(data) == 0 || len(data) > PrintLockMaxBytes {
		return nil, errors.New("printer lock metadata exceeds its size limit")
	}
	file, err := createPrintLockFile(path)
	if err != nil {
		return nil, err
	}
	cleanup := func(cause error) (*PrintLockLease, error) {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, cause
	}
	if _, err := file.Write(data); err != nil {
		return cleanup(fmt.Errorf("write printer lock metadata: %w", err))
	}
	if err := file.Sync(); err != nil {
		return cleanup(fmt.Errorf("sync printer lock metadata: %w", err))
	}
	return &PrintLockLease{file: file, path: path, Info: info}, nil
}

// ClaimPrintLockLease obtains exclusive write/delete access to an existing
// lease-versioned lock. Success is kernel evidence that the original holder no
// longer has its lease. Legacy locks are deliberately not claimable.
func ClaimPrintLockLease(path, sessionID string) (*PrintLockLease, error) {
	if !validSessionID(sessionID) {
		return nil, errors.New("printer lock session ID is invalid")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() ||
		info.Size() <= 0 || info.Size() > PrintLockMaxBytes {
		return nil, errors.New("printer lock is not a bounded regular file")
	}
	file, err := claimPrintLockFile(path)
	if err != nil {
		return nil, err
	}
	openedInfo, statErr := file.Stat()
	if statErr != nil || !openedInfo.Mode().IsRegular() || openedInfo.Mode()&os.ModeSymlink != 0 {
		_ = file.Close()
		if statErr != nil {
			return nil, fmt.Errorf("inspect claimed printer lock handle: %w", statErr)
		}
		return nil, errors.New("claimed printer lock handle is not a regular file")
	}
	lockInfo, readErr := readPrintLockFromFile(file, sessionID)
	if readErr != nil {
		_ = file.Close()
		return nil, readErr
	}
	if lockInfo.LeaseVersion != PrintLockLeaseVersion {
		_ = file.Close()
		return nil, ErrPrintLockLegacy
	}
	return &PrintLockLease{file: file, path: path, Info: lockInfo}, nil
}

// InspectPrintLockOwner probes the kernel lease without changing the lock file.
// An orphaned result is only a point-in-time observation; reconciliation must
// claim the lease again and keep that claim for the entire operation.
func InspectPrintLockOwner(path, sessionID string) (string, error) {
	lease, err := ClaimPrintLockLease(path, sessionID)
	switch {
	case errors.Is(err, ErrPrintLockActive):
		return PrintLockOwnerActive, nil
	case errors.Is(err, ErrPrintLockLegacy), errors.Is(err, ErrPrintLockUnsupported):
		return PrintLockOwnerUnverifiable, nil
	case err != nil:
		return PrintLockOwnerUnknown, err
	}
	if closeErr := lease.Close(); closeErr != nil {
		return PrintLockOwnerUnknown, closeErr
	}
	return PrintLockOwnerOrphaned, nil
}

// Close releases the kernel lease while preserving the marker file.
func (l *PrintLockLease) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	file := l.file
	l.file = nil
	return file.Close()
}

// Remove marks the held lock for deletion while exclusive access is still
// held, then closes it. On Windows this avoids a close/remove race in which a
// second resolver could claim the same orphan between those two operations.
func (l *PrintLockLease) Remove() error {
	if l == nil || l.file == nil {
		return errors.New("printer lock lease is not held")
	}
	file := l.file
	// Every platform helper closes the handle, even when marking the file for
	// deletion fails. Clear our reference regardless so callers cannot leak an
	// OS lease after a reported cleanup error.
	err := removePrintLockFile(file, l.path)
	l.file = nil
	return err
}

func validatePrintLockInfo(info PrintLockInfo, sessionID string) error {
	if !validSessionID(info.AttemptID) ||
		info.QueueName == "" || len(info.QueueName) > 256 ||
		info.QueueName != trimSpace(info.QueueName) || containsUnsafeLockText(info.QueueName) ||
		info.DocumentName != PrintDocumentName(sessionID, info.AttemptID) ||
		info.CreatedAt.IsZero() || info.OwnerPID < 0 ||
		info.LeaseVersion < 0 || info.LeaseVersion > PrintLockLeaseVersion {
		return errors.New("printer lock identity, lease version or timestamp is invalid")
	}
	if info.LeaseVersion == PrintLockLeaseVersion && !validSHA256(info.SourceSHA256) {
		return errors.New("versioned printer lock requires a valid rendered-source SHA-256")
	}
	if info.LeaseVersion == 0 && info.SourceSHA256 != "" && !validSHA256(info.SourceSHA256) {
		return errors.New("legacy printer lock source SHA-256 is invalid")
	}
	return nil
}

func readPrintLockFromFile(file *os.File, sessionID string) (PrintLockInfo, error) {
	var lock PrintLockInfo
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return lock, fmt.Errorf("rewind printer lock metadata: %w", err)
	}
	data, err := io.ReadAll(io.LimitReader(file, PrintLockMaxBytes+1))
	if err != nil {
		return lock, fmt.Errorf("read printer lock metadata: %w", err)
	}
	if len(data) == 0 || len(data) > PrintLockMaxBytes {
		return lock, errors.New("printer lock metadata exceeds its size limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&lock); err != nil {
		return lock, fmt.Errorf("decode printer lock metadata: %w", err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return lock, errors.New("printer lock metadata contains trailing data")
	}
	if err := validatePrintLockInfo(lock, sessionID); err != nil {
		return lock, err
	}
	return lock, nil
}

func trimSpace(value string) string {
	return strings.TrimSpace(value)
}

func containsUnsafeLockText(value string) bool {
	return strings.ContainsAny(value, "\r\n\x00")
}

// PrintLockPath returns the canonical lock marker name for a session folder.
func PrintLockPath(sessionDir string) string {
	return filepath.Join(sessionDir, ".print.lock")
}
