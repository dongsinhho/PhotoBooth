//go:build windows

package session

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testPrintLockInfo(sessionID, attemptID string) PrintLockInfo {
	return PrintLockInfo{
		AttemptID: attemptID, QueueName: "DS-RX1",
		DocumentName: PrintDocumentName(sessionID, attemptID),
		SourceSHA256: strings.Repeat("a", 64),
		CreatedAt:    time.Now().UTC(),
	}
}

func TestPrintLockKernelLeaseDetectsActiveAndOrphanedOwner(t *testing.T) {
	const sessionID = "a123456789abcdef0123456789abcdef"
	const attemptID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	path := filepath.Join(t.TempDir(), ".print.lock")
	lease, err := CreatePrintLockLease(path, sessionID, testPrintLockInfo(sessionID, attemptID))
	if err != nil {
		t.Fatalf("CreatePrintLockLease: %v", err)
	}
	owner, err := InspectPrintLockOwner(path, sessionID)
	if err != nil || owner != PrintLockOwnerActive {
		t.Fatalf("active owner=%q err=%v", owner, err)
	}
	if _, err := ClaimPrintLockLease(path, sessionID); !errors.Is(err, ErrPrintLockActive) {
		t.Fatalf("claim while active err=%v, want ErrPrintLockActive", err)
	}
	if err := lease.Close(); err != nil {
		t.Fatalf("release kernel lease: %v", err)
	}

	owner, err = InspectPrintLockOwner(path, sessionID)
	if err != nil || owner != PrintLockOwnerOrphaned {
		t.Fatalf("released owner=%q err=%v, want orphaned", owner, err)
	}
	claimed, err := ClaimPrintLockLease(path, sessionID)
	if err != nil {
		t.Fatalf("claim released lease: %v", err)
	}
	if claimed.Info.AttemptID != attemptID || claimed.Info.SourceSHA256 != strings.Repeat("a", 64) {
		t.Fatalf("claimed lock identity=%+v", claimed.Info)
	}
	if err := claimed.Remove(); err != nil {
		t.Fatalf("remove claimed lock: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock file remains after exclusive removal: %v", err)
	}
}

func TestPrintLockLeaseIsReleasedAutomaticallyWhenOwnerProcessExits(t *testing.T) {
	if os.Getenv("PHOTOBOOTH_PRINT_LOCK_CHILD") == "1" {
		path := os.Getenv("PHOTOBOOTH_PRINT_LOCK_PATH")
		sessionID := os.Getenv("PHOTOBOOTH_PRINT_LOCK_SESSION")
		const attemptID = "cccccccccccccccccccccccccccccccc"
		_, err := CreatePrintLockLease(path, sessionID, testPrintLockInfo(sessionID, attemptID))
		if err != nil {
			t.Fatalf("child CreatePrintLockLease: %v", err)
		}
		// Intentionally bypass all Go defers. Process termination must release
		// the kernel file handle while leaving the durable marker behind.
		os.Exit(0)
	}
	const sessionID = "d123456789abcdef0123456789abcdef"
	path := filepath.Join(t.TempDir(), ".print.lock")
	command := exec.Command(os.Args[0], "-test.run=^TestPrintLockLeaseIsReleasedAutomaticallyWhenOwnerProcessExits$")
	command.Env = append(os.Environ(),
		"PHOTOBOOTH_PRINT_LOCK_CHILD=1",
		"PHOTOBOOTH_PRINT_LOCK_PATH="+path,
		"PHOTOBOOTH_PRINT_LOCK_SESSION="+sessionID,
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("lock owner subprocess: %v output=%s", err, output)
	}
	owner, err := InspectPrintLockOwner(path, sessionID)
	if err != nil || owner != PrintLockOwnerOrphaned {
		t.Fatalf("owner after process exit=%q err=%v, want orphaned", owner, err)
	}
	claimed, err := ClaimPrintLockLease(path, sessionID)
	if err != nil {
		t.Fatalf("claim lock after owner process exit: %v", err)
	}
	if err := claimed.Remove(); err != nil {
		t.Fatalf("remove orphaned test lock: %v", err)
	}
}

func TestLegacyPrintLockRemainsUnverifiableAndUnmodified(t *testing.T) {
	const sessionID = "e123456789abcdef0123456789abcdef"
	const attemptID = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	path := filepath.Join(t.TempDir(), ".print.lock")
	data, err := json.Marshal(PrintLockInfo{
		AttemptID: attemptID, QueueName: "DS-RX1",
		DocumentName: PrintDocumentName(sessionID, attemptID),
		CreatedAt:    time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := InspectPrintLockOwner(path, sessionID)
	if err != nil || owner != PrintLockOwnerUnverifiable {
		t.Fatalf("legacy owner=%q err=%v, want unverifiable", owner, err)
	}
	if _, err := ClaimPrintLockLease(path, sessionID); !errors.Is(err, ErrPrintLockLegacy) {
		t.Fatalf("legacy claim err=%v, want ErrPrintLockLegacy", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("legacy lock changed during inspection: err=%v", err)
	}
}
