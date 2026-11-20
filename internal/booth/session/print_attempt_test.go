package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func validPrintTestManifest() Manifest {
	manifest := recoveryTestManifest()
	manifest.Version = ManifestVersion
	manifest.FrameSnapshotFile = FrameSnapshotFileName
	manifest.FrameSnapshotSHA256 = manifest.FrameConfigSHA256
	manifest.Phase = PhaseRendered
	manifest.CompletedShots = manifest.ExpectedShots
	manifest.RenderedAt = manifest.UpdatedAt
	return manifest
}

func validPrintTestAttempt(manifest Manifest, attemptID, status string, requestedAt time.Time) PrintAttempt {
	return PrintAttempt{
		AttemptID:    attemptID,
		QueueName:    "DS-RX1",
		DocumentName: PrintDocumentName(manifest.SessionID, attemptID),
		SourceSHA256: manifest.FrameConfigSHA256,
		Status:       status,
		RequestedAt:  requestedAt,
		UpdatedAt:    requestedAt,
	}
}

func TestPrintAttemptTransitionsPersistAndBlockAmbiguousRetry(t *testing.T) {
	manifest := validPrintTestManifest()
	now := time.Now().UTC()
	firstID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if err := BeginPrintAttempt(&manifest, validPrintTestAttempt(manifest, firstID, PrintStatusSubmitting, now)); err != nil {
		t.Fatalf("BeginPrintAttempt: %v", err)
	}
	if err := FinishPrintAttempt(&manifest, firstID, PrintStatusQueued, "101", "", now.Add(time.Second)); err != nil {
		t.Fatalf("FinishPrintAttempt queued: %v", err)
	}
	if got := manifest.PrintAttempts[0]; got.Status != PrintStatusQueued || got.JobID != "101" {
		t.Fatalf("queued attempt=%+v", got)
	}
	secondID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	second := validPrintTestAttempt(manifest, secondID, PrintStatusSubmitting, now.Add(2*time.Second))
	if err := BeginPrintAttempt(&manifest, second); err == nil {
		t.Fatal("queued attempt must block another print submission")
	}

	uncertain := validPrintTestManifest()
	uncertainAttempt := validPrintTestAttempt(uncertain, firstID, PrintStatusSubmitting, now)
	if err := BeginPrintAttempt(&uncertain, uncertainAttempt); err != nil {
		t.Fatal(err)
	}
	if err := FinishPrintAttempt(&uncertain, firstID, PrintStatusUncertain, "102", "EndDoc returned an error", now.Add(time.Second)); err != nil {
		t.Fatalf("FinishPrintAttempt uncertain: %v", err)
	}
	if err := BeginPrintAttempt(&uncertain, second); err == nil {
		t.Fatal("uncertain attempt must block another print submission")
	}
}

func TestKnownFailedPrintAttemptAllowsOnlyExplicitNewAttempt(t *testing.T) {
	manifest := validPrintTestManifest()
	now := time.Now().UTC()
	firstID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if err := BeginPrintAttempt(&manifest, validPrintTestAttempt(manifest, firstID, PrintStatusSubmitting, now)); err != nil {
		t.Fatal(err)
	}
	if err := FinishPrintAttempt(&manifest, firstID, PrintStatusFailed, "", "OpenPrinterW denied", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	secondID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	second := validPrintTestAttempt(manifest, secondID, PrintStatusSubmitting, now.Add(2*time.Second))
	if err := BeginPrintAttempt(&manifest, second); err != nil {
		t.Fatalf("known failed attempt should allow explicit new attempt: %v", err)
	}
	if len(manifest.PrintAttempts) != 2 {
		t.Fatalf("attempt count=%d, want 2", len(manifest.PrintAttempts))
	}
}

func TestPrintAttemptValidationRejectsDuplicateIDsAndInvalidQueuedRecords(t *testing.T) {
	manifest := validPrintTestManifest()
	now := time.Now().UTC()
	id := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	attempt := validPrintTestAttempt(manifest, id, PrintStatusQueued, now)
	if err := BeginPrintAttempt(&manifest, validPrintTestAttempt(manifest, id, PrintStatusSubmitting, now)); err != nil {
		t.Fatal(err)
	}
	if err := FinishPrintAttempt(&manifest, id, PrintStatusQueued, "200", "", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	duplicate := validPrintTestAttempt(manifest, id, PrintStatusSubmitting, now.Add(2*time.Second))
	if err := BeginPrintAttempt(&manifest, duplicate); err == nil {
		t.Fatal("duplicate attempt ID was accepted")
	}

	invalid := validPrintTestManifest()
	invalid.PrintAttempts = []PrintAttempt{attempt}
	if err := ValidateManifest(invalid); err == nil {
		t.Fatal("queued attempt without a job ID was accepted")
	}
}

func TestPrintAttemptsRequireManifestV3(t *testing.T) {
	manifest := validPrintTestManifest()
	manifest.Version = PolicyManifestVersion
	manifest.PrintAttempts = []PrintAttempt{validPrintTestAttempt(manifest, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", PrintStatusSubmitting, time.Now().UTC())}
	if err := ValidateManifest(manifest); err == nil {
		t.Fatal("version-2 manifest with print attempts was accepted")
	}
}

func TestReadFrameSnapshotSupportsExistingPolicyManifestV2(t *testing.T) {
	const id = "9123456789abcdef0123456789abcdef"
	sessionDir := filepath.Join(t.TempDir(), id)
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	frameBytes := []byte(`{"schema_version":1,"frame":"v2"}`)
	digest := sha256.Sum256(frameBytes)
	manifest := NewManifest(
		id, 3, filepath.Join(sessionDir, FrameSnapshotFileName),
		hex.EncodeToString(digest[:]), recoveryTestManifest().RunConfigFingerprint,
	)
	manifest.Version = PolicyManifestVersion
	path, snapshotHash, err := WriteFrameSnapshot(sessionDir, frameBytes)
	if err != nil {
		t.Fatal(err)
	}
	manifest.FrameConfigPath = path
	manifest.FrameSnapshotFile = FrameSnapshotFileName
	manifest.FrameSnapshotSHA256 = snapshotHash
	manifest.Phase = PhaseRendered
	manifest.CompletedShots = manifest.ExpectedShots
	manifest.RenderedAt = manifest.UpdatedAt
	if err := WriteManifest(sessionDir, manifest); err != nil {
		t.Fatalf("WriteManifest v2: %v", err)
	}
	readPath, data, err := ReadFrameSnapshot(sessionDir, manifest)
	if err != nil {
		t.Fatalf("ReadFrameSnapshot v2: %v", err)
	}
	if readPath != path || string(data) != string(frameBytes) {
		t.Fatalf("snapshot path/data mismatch: path=%q data=%q", readPath, data)
	}
}

func TestInspectRecoveryRootExposesPrintIntentAndStaleLockReadOnly(t *testing.T) {
	root := t.TempDir()
	const id = "a123456789abcdef0123456789abcdef"
	sessionDir := filepath.Join(root, id)
	if err := os.MkdirAll(filepath.Join(sessionDir, "captures"), 0o700); err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= 3; index++ {
		writeRecoveryJPEG(t, filepath.Join(sessionDir, "captures", captureFileName(index)))
	}
	writeRecoveryJPEG(t, filepath.Join(sessionDir, "rendered.jpg"))
	manifest := validPrintTestManifest()
	manifest.SessionID = id
	if err := os.WriteFile(filepath.Join(sessionDir, FrameSnapshotFileName), []byte("frame"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest.FrameConfigPath = filepath.Join(sessionDir, FrameSnapshotFileName)
	manifest.Phase = PhaseOutputFinalized
	manifest.OutputPath = filepath.Join(root, "output", id, "photo.jpg")
	manifest.OutputFinalizedAt = manifest.RenderedAt.Add(time.Second)
	attempt := validPrintTestAttempt(manifest, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", PrintStatusSubmitting, manifest.RenderedAt.Add(time.Second*2))
	manifest.PrintAttempts = []PrintAttempt{attempt}
	manifest.UpdatedAt = attempt.UpdatedAt
	// The lock survives an interrupted process after durable intent publication.
	lockInfo := PrintLockInfo{
		AttemptID: attempt.AttemptID, QueueName: attempt.QueueName,
		DocumentName: attempt.DocumentName, CreatedAt: attempt.RequestedAt,
	}
	lockBytes, err := json.Marshal(lockInfo)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, ".print.lock"), lockBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteManifest(sessionDir, manifest); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	before, err := os.ReadFile(filepath.Join(sessionDir, ManifestFileName))
	if err != nil {
		t.Fatal(err)
	}
	scan, err := InspectRecoveryRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Items) != 1 {
		t.Fatalf("recovery items=%d, want 1", len(scan.Items))
	}
	item := scan.Items[0]
	if item.Status != RecoveryFinalized || len(item.PrintAttempts) != 1 || !item.PrintLockPresent {
		t.Fatalf("recovery item=%+v", item)
	}
	if item.PrintAttempts[0].Status != PrintStatusSubmitting || item.Reason == "" {
		t.Fatalf("printer uncertainty was not surfaced: %+v", item)
	}
	if item.PrintLock == nil || item.PrintLock.AttemptID != attempt.AttemptID ||
		item.PrintLock.QueueName != attempt.QueueName || item.PrintLock.DocumentName != attempt.DocumentName {
		t.Fatalf("stale lock lookup identity was not preserved: %+v", item.PrintLock)
	}
	after, err := os.ReadFile(filepath.Join(sessionDir, ManifestFileName))
	if err != nil || string(after) != string(before) {
		t.Fatalf("read-only recovery changed manifest: err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(sessionDir, ".print.lock")); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestInspectRecoveryRootRecoversLockIdentityBeforeManifestIntent(t *testing.T) {
	root := t.TempDir()
	const sessionID = "c123456789abcdef0123456789abcdef"
	sessionDir := filepath.Join(root, sessionID)
	captureDir := filepath.Join(sessionDir, "captures")
	if err := os.MkdirAll(captureDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= 3; index++ {
		writeRecoveryJPEG(t, filepath.Join(captureDir, captureFileName(index)))
	}
	writeRecoveryJPEG(t, filepath.Join(sessionDir, "rendered.jpg"))

	manifest := validPrintTestManifest()
	manifest.SessionID = sessionID
	frameBytes := []byte(`{"schema_version":1,"frame":"lock-only"}`)
	frameDigest := sha256.Sum256(frameBytes)
	framePath, snapshotHash, err := WriteFrameSnapshot(sessionDir, frameBytes)
	if err != nil {
		t.Fatal(err)
	}
	manifest.FrameConfigPath = framePath
	manifest.FrameConfigSHA256 = hex.EncodeToString(frameDigest[:])
	manifest.FrameSnapshotFile = FrameSnapshotFileName
	manifest.FrameSnapshotSHA256 = snapshotHash
	manifest.PrintAttempts = nil
	if err := WriteManifest(sessionDir, manifest); err != nil {
		t.Fatal(err)
	}

	lock := PrintLockInfo{
		AttemptID: "dddddddddddddddddddddddddddddddd",
		QueueName: "DS-RX1",
		CreatedAt: time.Now().UTC(),
	}
	lock.DocumentName = PrintDocumentName(sessionID, lock.AttemptID)
	lockBytes, err := json.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, ".print.lock"), lockBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	scan, err := InspectRecoveryRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Items) != 1 {
		t.Fatalf("recovery items=%d, want 1", len(scan.Items))
	}
	item := scan.Items[0]
	if item.PrintLock == nil || item.PrintLock.AttemptID != lock.AttemptID ||
		item.PrintLock.QueueName != lock.QueueName || item.PrintLock.DocumentName != lock.DocumentName {
		t.Fatalf("lock-only attempt identity was not recovered: %+v", item.PrintLock)
	}
	if len(item.PrintAttempts) != 0 || !item.PrintLockPresent || item.Reason == "" {
		t.Fatalf("lock-only crash was not classified as uncertain: %+v", item)
	}
}

func TestOperatorPrintResolutionRequiresFreshQueueAbsenceForPhysicalDecision(t *testing.T) {
	manifest := validPrintTestManifest()
	requested := manifest.RenderedAt.Add(time.Second)
	attemptID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if err := BeginPrintAttempt(&manifest, validPrintTestAttempt(manifest, attemptID, PrintStatusSubmitting, requested)); err != nil {
		t.Fatal(err)
	}
	if err := FinishPrintAttempt(&manifest, attemptID, PrintStatusQueued, "301", "", requested.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	checked := requested.Add(2 * time.Second)
	recorded := checked.Add(time.Second)

	for _, decision := range []string{PrintResolutionPrinted, PrintResolutionNotPrinted} {
		t.Run(decision+"-queue-still-found", func(t *testing.T) {
			candidate := manifest
			candidate.PrintResolutions = append([]PrintResolution(nil), manifest.PrintResolutions...)
			resolution := PrintResolution{
				AttemptID: attemptID, Decision: decision, QueueFound: true,
				QueueCheckedAt: checked, RecordedAt: recorded,
				Note: "operator inspected the physical output",
			}
			if err := ResolvePrintAttempt(&candidate, resolution); err == nil {
				t.Fatal("physical decision was accepted while the job is still listed")
			}
		})
	}

	t.Run("not-printed-allows-a-separate-explicit-attempt", func(t *testing.T) {
		candidate := manifest
		candidate.PrintResolutions = append([]PrintResolution(nil), manifest.PrintResolutions...)
		resolution := PrintResolution{
			AttemptID: attemptID, Decision: PrintResolutionNotPrinted,
			QueueFound: false, QueueCheckedAt: checked, RecordedAt: recorded,
			Note: "no paper emerged; output tray and printer were checked",
		}
		if err := ResolvePrintAttempt(&candidate, resolution); err != nil {
			t.Fatalf("ResolvePrintAttempt: %v", err)
		}
		if got, ok := PrintResolutionFor(candidate, attemptID); !ok || got.Decision != PrintResolutionNotPrinted {
			t.Fatalf("resolution not persisted: %+v, found=%v", got, ok)
		}
		nextID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		next := validPrintTestAttempt(candidate, nextID, PrintStatusSubmitting, recorded.Add(time.Second))
		if err := BeginPrintAttempt(&candidate, next); err != nil {
			t.Fatalf("explicit new attempt after operator-confirmed not_printed: %v", err)
		}
	})

	for _, decision := range []string{PrintResolutionPrinted, PrintResolutionUnknown} {
		t.Run(decision+"-blocks-further-attempts", func(t *testing.T) {
			candidate := manifest
			candidate.PrintResolutions = append([]PrintResolution(nil), manifest.PrintResolutions...)
			resolution := PrintResolution{
				AttemptID: attemptID, Decision: decision,
				QueueFound: false, QueueCheckedAt: checked, RecordedAt: recorded,
				Note: "operator closed the attempt after checking the printer",
			}
			if err := ResolvePrintAttempt(&candidate, resolution); err != nil {
				t.Fatalf("ResolvePrintAttempt: %v", err)
			}
			nextID := "cccccccccccccccccccccccccccccccc"
			next := validPrintTestAttempt(candidate, nextID, PrintStatusSubmitting, recorded.Add(time.Second))
			if err := BeginPrintAttempt(&candidate, next); err == nil {
				t.Fatal("printed/unknown resolution must block another attempt")
			}
		})
	}
}

func TestPrintResolutionRejectsDuplicateUnknownAndInvalidRecords(t *testing.T) {
	manifest := validPrintTestManifest()
	requested := manifest.RenderedAt.Add(time.Second)
	attemptID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if err := BeginPrintAttempt(&manifest, validPrintTestAttempt(manifest, attemptID, PrintStatusSubmitting, requested)); err != nil {
		t.Fatal(err)
	}
	if err := FinishPrintAttempt(&manifest, attemptID, PrintStatusUncertain, "", "crash", requested.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	checked := requested.Add(2 * time.Second)
	valid := PrintResolution{
		AttemptID: attemptID, Decision: PrintResolutionUnknown,
		QueueFound: true, QueueCheckedAt: checked, RecordedAt: checked.Add(time.Second),
		Note: "queue entry remains; outcome cannot be established",
	}
	if err := ResolvePrintAttempt(&manifest, valid); err != nil {
		t.Fatalf("unknown close decision should be allowed even if queue job remains: %v", err)
	}
	if err := ResolvePrintAttempt(&manifest, valid); err == nil {
		t.Fatal("duplicate resolution for one attempt was accepted")
	}
	if err := BeginPrintAttempt(&manifest, validPrintTestAttempt(manifest, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", PrintStatusSubmitting, checked.Add(2*time.Second))); err == nil {
		t.Fatal("unknown outcome must remain blocked from retry")
	}

	invalid := validPrintTestManifest()
	invalidAttempt := validPrintTestAttempt(invalid, attemptID, PrintStatusQueued, requested)
	invalidAttempt.JobID = "302"
	invalid.PrintAttempts = []PrintAttempt{invalidAttempt}
	invalid.UpdatedAt = requested
	bad := valid
	bad.RecordedAt = checked.Add(time.Second)
	bad.QueueCheckedAt = requested.Add(-time.Second)
	invalid.PrintResolutions = []PrintResolution{bad}
	if err := ValidateManifest(invalid); err == nil {
		t.Fatal("resolution predating the queue attempt was accepted")
	}
}

func TestPrintResolutionPersistsWithManifestAndKeepsOriginalAttempt(t *testing.T) {
	root := t.TempDir()
	const sessionID = "e123456789abcdef0123456789abcdef"
	sessionDir := filepath.Join(root, sessionID)
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := validPrintTestManifest()
	manifest.SessionID = sessionID
	attemptID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	requested := manifest.RenderedAt.Add(time.Second)
	if err := BeginPrintAttempt(&manifest, validPrintTestAttempt(manifest, attemptID, PrintStatusSubmitting, requested)); err != nil {
		t.Fatal(err)
	}
	if err := FinishPrintAttempt(&manifest, attemptID, PrintStatusQueued, "303", "", requested.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	checked := requested.Add(2 * time.Second)
	resolution := PrintResolution{
		AttemptID: attemptID, Decision: PrintResolutionPrinted,
		QueueFound: false, QueueCheckedAt: checked, RecordedAt: checked.Add(time.Second),
		Note: "operator visually observed the print on the output tray",
	}
	if err := ResolvePrintAttempt(&manifest, resolution); err != nil {
		t.Fatalf("ResolvePrintAttempt: %v", err)
	}
	if err := WriteManifest(sessionDir, manifest); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	got, err := ReadManifest(sessionDir)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if len(got.PrintAttempts) != 1 || got.PrintAttempts[0].Status != PrintStatusQueued ||
		got.PrintAttempts[0].JobID != "303" {
		t.Fatalf("original print attempt was changed by resolution: %+v", got.PrintAttempts)
	}
	if len(got.PrintResolutions) != 1 || got.PrintResolutions[0] != resolution {
		t.Fatalf("print resolution did not round trip: %+v", got.PrintResolutions)
	}
}
