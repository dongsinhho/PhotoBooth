package output

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dongsinhho/PhotoBooth/internal/booth/session"
)

type durablePrinterTarget struct {
	mu    sync.Mutex
	calls int
	run   func(context.Context, Job) (TargetOutput, error)
}

func (t *durablePrinterTarget) Name() string { return TargetPrinter }
func (t *durablePrinterTarget) Execute(ctx context.Context, job Job) (TargetOutput, error) {
	t.mu.Lock()
	t.calls++
	t.mu.Unlock()
	if t.run == nil {
		return TargetOutput{}, errors.New("unexpected printer target call")
	}
	return t.run(ctx, job)
}

func (t *durablePrinterTarget) callCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.calls
}

func newDurablePrinterFixture(t *testing.T, version int) (string, string, string, session.Manifest) {
	t.Helper()
	const sessionID = "8123456789abcdef0123456789abcdef"
	root := t.TempDir()
	sessionDir := filepath.Join(root, sessionID)
	captureDir := filepath.Join(sessionDir, "captures")
	if err := os.MkdirAll(captureDir, 0o700); err != nil {
		t.Fatal(err)
	}
	frameBytes := []byte(`{"schema_version":1,"frame":"durable-printer-test"}`)
	frameDigest := sha256.Sum256(frameBytes)
	configDigest := sha256.Sum256([]byte("durable-printer-config"))
	manifest := session.NewManifest(
		sessionID, 3, filepath.Join(sessionDir, session.FrameSnapshotFileName),
		hex.EncodeToString(frameDigest[:]), hex.EncodeToString(configDigest[:]),
	)
	if version != session.ManifestVersion {
		manifest.Version = version
	}
	framePath, snapshotHash, err := session.WriteFrameSnapshot(sessionDir, frameBytes)
	if err != nil {
		t.Fatalf("WriteFrameSnapshot: %v", err)
	}
	manifest.FrameConfigPath = framePath
	manifest.FrameSnapshotFile = session.FrameSnapshotFileName
	manifest.FrameSnapshotSHA256 = snapshotHash

	for shot := 1; shot <= manifest.ExpectedShots; shot++ {
		writeDurablePrinterJPEG(t, filepath.Join(captureDir, fmt.Sprintf("capture-%02d.jpg", shot)))
	}
	renderedPath := filepath.Join(sessionDir, "rendered.jpg")
	writeDurablePrinterJPEG(t, renderedPath)
	manifest.Phase = session.PhaseRendered
	manifest.CompletedShots = manifest.ExpectedShots
	manifest.RenderedAt = time.Now().UTC()
	manifest.UpdatedAt = manifest.RenderedAt
	if version == session.LegacyManifestVersion {
		manifest.FrameSnapshotFile = ""
		manifest.FrameSnapshotSHA256 = ""
		_ = os.Remove(framePath)
	}
	if err := session.WriteManifest(sessionDir, manifest); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	return root, sessionID, renderedPath, manifest
}

func writeDurablePrinterJPEG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 24, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 24; x++ {
			img.Set(x, y, color.RGBA{R: uint8(20 + x*4), G: uint8(30 + y*3), B: 150, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: 90}); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func newTestDurablePrinter(t *testing.T, root string, target Target) *DurablePrinter {
	t.Helper()
	printer, err := NewDurablePrinter(root, "DS-RX1", target)
	if err != nil {
		t.Fatalf("NewDurablePrinter: %v", err)
	}
	printer.now = func() time.Time { return time.Now().UTC() }
	id := 0
	printer.newAttemptID = func() (string, error) {
		id++
		return fmt.Sprintf("a%031x", id), nil
	}
	return printer
}

func TestDurablePrinterPersistsIntentBeforeQueueAndStoresJobID(t *testing.T) {
	root, sessionID, renderedPath, _ := newDurablePrinterFixture(t, session.ManifestVersion)
	var observed Job
	target := &durablePrinterTarget{run: func(_ context.Context, job Job) (TargetOutput, error) {
		observed = job
		manifest, err := session.ReadManifest(filepath.Join(root, sessionID))
		if err != nil {
			return TargetOutput{}, err
		}
		if len(manifest.PrintAttempts) != 1 || manifest.PrintAttempts[0].Status != session.PrintStatusSubmitting {
			return TargetOutput{}, errors.New("print intent was not persisted before target execution")
		}
		return TargetOutput{Status: TargetQueued, JobID: "42"}, nil
	}}
	printer := newTestDurablePrinter(t, root, target)

	result, err := printer.Execute(context.Background(), Job{SessionID: sessionID, SourcePath: renderedPath})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Status != TargetQueued || result.JobID != "42" {
		t.Fatalf("result=%+v, want queued job 42", result)
	}
	if observed.DocumentName != session.PrintDocumentName(sessionID, "a0000000000000000000000000000001") {
		t.Fatalf("document name=%q", observed.DocumentName)
	}
	manifest, err := session.ReadManifest(filepath.Join(root, sessionID))
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.PrintAttempts) != 1 {
		t.Fatalf("attempt count=%d, want 1", len(manifest.PrintAttempts))
	}
	attempt := manifest.PrintAttempts[0]
	if attempt.Status != session.PrintStatusQueued || attempt.JobID != "42" || attempt.QueueName != "DS-RX1" {
		t.Fatalf("persisted attempt=%+v", attempt)
	}
	if attempt.SourceSHA256 == "" || attempt.DocumentName != observed.DocumentName {
		t.Fatalf("source hash/document name were not recorded: %+v", attempt)
	}
	if _, err := os.Lstat(filepath.Join(root, sessionID, printLockFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("print lock remains after successful execution: %v", err)
	}
}

func TestDurablePrinterKnownPreAcceptanceFailureAllowsExplicitNextAttempt(t *testing.T) {
	root, sessionID, renderedPath, _ := newDurablePrinterFixture(t, session.ManifestVersion)
	target := &durablePrinterTarget{run: func(_ context.Context, job Job) (TargetOutput, error) {
		if job.DocumentName == "" {
			t.Fatal("durable document name was not supplied")
		}
		if job.DocumentName == session.PrintDocumentName(sessionID, "a0000000000000000000000000000001") {
			return TargetOutput{}, &PrinterSubmissionError{Err: errors.New("OpenPrinterW denied")}
		}
		return TargetOutput{Status: TargetQueued, JobID: "43"}, nil
	}}
	printer := newTestDurablePrinter(t, root, target)

	if _, err := printer.Execute(context.Background(), Job{SessionID: sessionID, SourcePath: renderedPath}); err == nil {
		t.Fatal("expected known pre-acceptance printer failure")
	}
	manifest, err := session.ReadManifest(filepath.Join(root, sessionID))
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.PrintAttempts) != 1 || manifest.PrintAttempts[0].Status != session.PrintStatusFailed {
		t.Fatalf("first attempt=%+v; want persisted failed state", manifest.PrintAttempts)
	}
	result, err := printer.Execute(context.Background(), Job{SessionID: sessionID, SourcePath: renderedPath})
	if err != nil {
		t.Fatalf("explicit second attempt: %v", err)
	}
	if result.Status != TargetQueued || result.JobID != "43" {
		t.Fatalf("second result=%+v", result)
	}
	manifest, err = session.ReadManifest(filepath.Join(root, sessionID))
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.PrintAttempts) != 2 || manifest.PrintAttempts[1].Status != session.PrintStatusQueued {
		t.Fatalf("attempt history=%+v", manifest.PrintAttempts)
	}
}

func TestDurablePrinterUnknownOutcomeIsPersistedUncertainAndBlocksRetry(t *testing.T) {
	root, sessionID, renderedPath, _ := newDurablePrinterFixture(t, session.ManifestVersion)
	target := &durablePrinterTarget{run: func(context.Context, Job) (TargetOutput, error) {
		return TargetOutput{}, errors.New("driver returned an unclassified failure")
	}}
	printer := newTestDurablePrinter(t, root, target)

	_, firstErr := printer.Execute(context.Background(), Job{SessionID: sessionID, SourcePath: renderedPath})
	var submissionErr *PrinterSubmissionError
	if !errors.As(firstErr, &submissionErr) || !submissionErr.Uncertain {
		t.Fatalf("error=%v, want uncertain PrinterSubmissionError", firstErr)
	}
	manifest, err := session.ReadManifest(filepath.Join(root, sessionID))
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.PrintAttempts) != 1 || manifest.PrintAttempts[0].Status != session.PrintStatusUncertain {
		t.Fatalf("attempt history=%+v, want uncertain", manifest.PrintAttempts)
	}

	if _, err := printer.Execute(context.Background(), Job{SessionID: sessionID, SourcePath: renderedPath}); err == nil {
		t.Fatal("uncertain previous job must block automatic resubmission")
	}
	if target.callCount() != 1 {
		t.Fatalf("printer target calls=%d, want exactly one", target.callCount())
	}
}

func TestDurablePrinterPostSubmissionManifestFailureLeavesSubmittingIntent(t *testing.T) {
	root, sessionID, renderedPath, _ := newDurablePrinterFixture(t, session.ManifestVersion)
	target := &durablePrinterTarget{run: func(context.Context, Job) (TargetOutput, error) {
		return TargetOutput{Status: TargetQueued, JobID: "44"}, nil
	}}
	printer := newTestDurablePrinter(t, root, target)
	var writes int
	printer.writeManifest = func(dir string, manifest session.Manifest) error {
		writes++
		if writes == 2 {
			return errors.New("simulated disk failure after spooler acceptance")
		}
		return session.WriteManifest(dir, manifest)
	}

	_, err := printer.Execute(context.Background(), Job{SessionID: sessionID, SourcePath: renderedPath})
	var submissionErr *PrinterSubmissionError
	if !errors.As(err, &submissionErr) || !submissionErr.Uncertain || submissionErr.JobID != "44" {
		t.Fatalf("error=%v, want uncertain job 44", err)
	}
	manifest, readErr := session.ReadManifest(filepath.Join(root, sessionID))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(manifest.PrintAttempts) != 1 || manifest.PrintAttempts[0].Status != session.PrintStatusSubmitting {
		t.Fatalf("on-disk attempt=%+v; it must remain submitting after result-write failure", manifest.PrintAttempts)
	}
	scan, scanErr := session.InspectRecoveryRoot(root)
	if scanErr != nil {
		t.Fatal(scanErr)
	}
	if len(scan.Items) != 1 || scan.Items[0].Status != session.RecoveryOutputPending ||
		!strings.Contains(scan.Items[0].Reason, "uncertain") {
		t.Fatalf("recovery scan=%+v, want output-pending with uncertain-print warning", scan)
	}
	if target.callCount() != 1 {
		t.Fatalf("printer target calls=%d, want 1", target.callCount())
	}
}

func TestDurablePrinterRejectsStaleCrashLockWithoutSubmitting(t *testing.T) {
	root, sessionID, renderedPath, _ := newDurablePrinterFixture(t, session.ManifestVersion)
	lock := filepath.Join(root, sessionID, printLockFileName)
	lockInfo := session.PrintLockInfo{
		AttemptID:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		QueueName:    "DS-RX1",
		DocumentName: session.PrintDocumentName(sessionID, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		CreatedAt:    time.Now().UTC(),
	}
	lockData, err := json.Marshal(lockInfo)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lock, lockData, 0o600); err != nil {
		t.Fatal(err)
	}
	target := &durablePrinterTarget{run: func(context.Context, Job) (TargetOutput, error) {
		return TargetOutput{Status: TargetQueued, JobID: "45"}, nil
	}}
	printer := newTestDurablePrinter(t, root, target)
	if _, err := printer.Execute(context.Background(), Job{SessionID: sessionID, SourcePath: renderedPath}); err == nil {
		t.Fatal("stale crash lock must prevent submission")
	}
	if target.callCount() != 0 {
		t.Fatalf("printer target calls=%d, want 0", target.callCount())
	}
	scan, err := session.InspectRecoveryRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Items) != 1 || !scan.Items[0].PrintLockPresent ||
		scan.Items[0].PrintLockOwner != session.PrintLockOwnerUnverifiable {
		t.Fatalf("scan=%+v, want an unverifiable legacy-lock warning", scan)
	}
}

func TestDurablePrinterUpgradesPolicyManifestV2BeforeFirstAttempt(t *testing.T) {
	root, sessionID, renderedPath, _ := newDurablePrinterFixture(t, session.PolicyManifestVersion)
	target := &durablePrinterTarget{run: func(context.Context, Job) (TargetOutput, error) {
		return TargetOutput{Status: TargetQueued, JobID: "46"}, nil
	}}
	printer := newTestDurablePrinter(t, root, target)
	if _, err := printer.Execute(context.Background(), Job{SessionID: sessionID, SourcePath: renderedPath}); err != nil {
		t.Fatalf("Execute on v2 session: %v", err)
	}
	manifest, err := session.ReadManifest(filepath.Join(root, sessionID))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Version != session.ManifestVersion || len(manifest.PrintAttempts) != 1 {
		t.Fatalf("manifest version=%d attempts=%+v; want v3 and one attempt", manifest.Version, manifest.PrintAttempts)
	}
}

func TestDurablePrinterAllowsOnlyExplicitExecuteAfterNotPrintedResolution(t *testing.T) {
	root, sessionID, renderedPath, _ := newDurablePrinterFixture(t, session.ManifestVersion)
	sessionDir := filepath.Join(root, sessionID)
	manifest, err := session.ReadManifest(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	sourceHash, err := hashRegularFile(renderedPath)
	if err != nil {
		t.Fatal(err)
	}
	requestedAt := manifest.RenderedAt.Add(time.Second)
	const previousID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	previous := session.PrintAttempt{
		AttemptID: previousID, QueueName: "DS-RX1",
		DocumentName: session.PrintDocumentName(sessionID, previousID),
		SourceSHA256: sourceHash, Status: session.PrintStatusSubmitting,
		RequestedAt: requestedAt, UpdatedAt: requestedAt,
	}
	if err := session.BeginPrintAttempt(&manifest, previous); err != nil {
		t.Fatal(err)
	}
	queuedAt := requestedAt.Add(time.Second)
	if err := session.FinishPrintAttempt(&manifest, previousID, session.PrintStatusQueued, "500", "", queuedAt); err != nil {
		t.Fatal(err)
	}
	checkedAt := queuedAt.Add(time.Second)
	resolution := session.PrintResolution{
		AttemptID: previousID, Decision: session.PrintResolutionNotPrinted,
		QueueFound: false, QueueCheckedAt: checkedAt,
		RecordedAt: checkedAt.Add(time.Second),
		Note:       "operator confirmed no paper was produced after checking the output tray",
	}
	if err := session.ResolvePrintAttempt(&manifest, resolution); err != nil {
		t.Fatalf("ResolvePrintAttempt: %v", err)
	}
	if err := session.WriteManifest(sessionDir, manifest); err != nil {
		t.Fatalf("persist resolution: %v", err)
	}

	target := &durablePrinterTarget{run: func(context.Context, Job) (TargetOutput, error) {
		return TargetOutput{Status: TargetQueued, JobID: "501"}, nil
	}}
	printer := newTestDurablePrinter(t, root, target)
	printer.now = func() time.Time { return checkedAt.Add(2 * time.Second) }
	result, err := printer.Execute(context.Background(), Job{SessionID: sessionID, SourcePath: renderedPath})
	if err != nil {
		t.Fatalf("explicit Execute after not_printed: %v", err)
	}
	if result.Status != TargetQueued || result.JobID != "501" || target.callCount() != 1 {
		t.Fatalf("result=%+v calls=%d", result, target.callCount())
	}
	got, err := session.ReadManifest(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.PrintAttempts) != 2 || got.PrintAttempts[0].JobID != "500" ||
		got.PrintResolutions[0].Decision != session.PrintResolutionNotPrinted ||
		got.PrintAttempts[1].Status != session.PrintStatusQueued || got.PrintAttempts[1].JobID != "501" {
		t.Fatalf("new explicit attempt did not preserve the earlier audit: attempts=%+v resolutions=%+v",
			got.PrintAttempts, got.PrintResolutions)
	}
}
