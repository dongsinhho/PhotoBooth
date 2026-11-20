package output

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
	"github.com/dongsinhho/PhotoBooth/internal/booth/session"
)

const printLockFileName = ".print.lock"

var durablePrinterLocks sync.Map

// DurablePrinter persists a print intent before delegating to a platform
// printer target. A crash after the intent is written but before the outcome
// is persisted leaves the attempt as submitting; recovery treats that as
// uncertain and never resubmits it automatically.
type DurablePrinter struct {
	SessionsRoot string
	QueueName    string
	Target       Target

	now           func() time.Time
	newAttemptID  func() (string, error)
	writeManifest func(string, session.Manifest) error
}

// NewDurablePrinter wraps a platform printer target with per-session durable
// intent, result persistence and an exclusive crash-detectable lock.
func NewDurablePrinter(sessionsRoot, queueName string, target Target) (*DurablePrinter, error) {
	sessionsRoot = strings.TrimSpace(sessionsRoot)
	queueName = strings.TrimSpace(queueName)
	if sessionsRoot == "" {
		return nil, errors.New("printer sessions root is required")
	}
	if queueName == "" || len(queueName) > 256 {
		return nil, errors.New("printer queue name must contain between 1 and 256 bytes")
	}
	if target == nil || target.Name() != TargetPrinter {
		return nil, errors.New("durable printer requires a target named printer")
	}
	if windowsTarget, ok := target.(*WindowsPrinter); ok && !strings.EqualFold(strings.TrimSpace(windowsTarget.PrinterName), queueName) {
		return nil, fmt.Errorf("durable printer queue %q does not match the Windows target queue %q", queueName, windowsTarget.PrinterName)
	}
	absoluteRoot, err := filepath.Abs(sessionsRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve printer sessions root: %w", err)
	}
	return &DurablePrinter{
		SessionsRoot:  absoluteRoot,
		QueueName:     queueName,
		Target:        target,
		now:           func() time.Time { return time.Now().UTC() },
		newAttemptID:  newPrintAttemptID,
		writeManifest: session.WriteManifest,
	}, nil
}

func (p *DurablePrinter) Name() string { return TargetPrinter }

func (p *DurablePrinter) Execute(ctx context.Context, job Job) (result TargetOutput, retErr error) {
	if p == nil || p.Target == nil {
		return TargetOutput{}, errors.New("durable printer is not configured")
	}
	if ctx == nil {
		return TargetOutput{}, errors.New("durable printer context is required")
	}
	if err := ctx.Err(); err != nil {
		return TargetOutput{}, err
	}
	if !validSessionID(job.SessionID) {
		return TargetOutput{}, errors.New("printer job session ID must be a 32-character lowercase hexadecimal identifier")
	}
	if strings.TrimSpace(job.SourcePath) == "" {
		return TargetOutput{}, errors.New("printer source path is required")
	}
	if err := stationconfig.ValidateLocalStoragePath(p.SessionsRoot); err != nil {
		return TargetOutput{}, fmt.Errorf("validate printer sessions root: %w", err)
	}

	sessionDir := filepath.Join(p.SessionsRoot, job.SessionID)
	if err := stationconfig.ValidateStorageChild(p.SessionsRoot, sessionDir); err != nil {
		return TargetOutput{}, fmt.Errorf("validate printer session directory: %w", err)
	}
	if err := requireRetryDirectory(sessionDir, "printer session directory"); err != nil {
		return TargetOutput{}, err
	}
	renderedPath := filepath.Join(sessionDir, "rendered.jpg")
	if filepath.Clean(job.SourcePath) != filepath.Clean(renderedPath) {
		return TargetOutput{}, errors.New("durable printer source must be the rendered.jpg owned by the session")
	}
	if err := stationconfig.ValidateStorageChild(sessionDir, renderedPath); err != nil {
		return TargetOutput{}, fmt.Errorf("validate rendered printer source identity: %w", err)
	}
	sourceHash, err := hashRegularFile(renderedPath)
	if err != nil {
		return TargetOutput{}, fmt.Errorf("hash rendered printer source: %w", err)
	}

	lock := session.PrintLockPath(sessionDir)
	if err := stationconfig.ValidateStorageChild(sessionDir, lock); err != nil {
		return TargetOutput{}, fmt.Errorf("validate printer lock identity: %w", err)
	}
	if _, loaded := durablePrinterLocks.LoadOrStore(lock, struct{}{}); loaded {
		return TargetOutput{}, errors.New("another printer submission is already active for this session")
	}
	defer durablePrinterLocks.Delete(lock)

	attemptID, err := p.newAttemptID()
	if err != nil {
		return TargetOutput{}, fmt.Errorf("generate printer attempt ID: %w", err)
	}
	if !validSessionID(attemptID) {
		return TargetOutput{}, errors.New("printer attempt ID generator returned an invalid identifier")
	}
	lockCreatedAt := p.now()
	documentName := session.PrintDocumentName(job.SessionID, attemptID)
	lockInfo := session.PrintLockInfo{
		AttemptID: attemptID, QueueName: p.QueueName,
		DocumentName: documentName, SourceSHA256: sourceHash,
		CreatedAt: lockCreatedAt,
	}
	lockLease, err := session.CreatePrintLockLease(lock, job.SessionID, lockInfo)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return TargetOutput{}, errors.New("a previous printer submission lock remains; inspect its owner state before any retry")
		}
		return TargetOutput{}, fmt.Errorf("create exclusive printer lock lease: %w", err)
	}
	defer func() {
		if removeErr := lockLease.Remove(); removeErr != nil && retErr == nil {
			retErr = &PrinterSubmissionError{Uncertain: true, JobID: result.JobID, Err: fmt.Errorf("remove printer lock lease: %w", removeErr)}
		}
	}()

	manifest, err := session.ReadManifest(sessionDir)
	if err != nil {
		return TargetOutput{}, fmt.Errorf("read durable printer session manifest: %w", err)
	}
	if manifest.SessionID != job.SessionID {
		return TargetOutput{}, errors.New("printer session manifest identity does not match the job")
	}
	if manifest.Phase != session.PhaseRendered && manifest.Phase != session.PhaseOutputFinalized {
		return TargetOutput{}, errors.New("printer output requires a fully rendered session")
	}
	if manifest.Version == session.PolicyManifestVersion {
		// Version 2 already contains the immutable frame snapshot and policy
		// fields. Upgrade only the manifest envelope; preserve all old data.
		manifest.Version = session.ManifestVersion
		if err := p.persistManifest(sessionDir, manifest); err != nil {
			return TargetOutput{}, fmt.Errorf("upgrade session manifest for durable printer state: %w", err)
		}
	}
	if manifest.Version != session.ManifestVersion {
		return TargetOutput{}, fmt.Errorf("durable printer state requires manifest version %d with a frame snapshot; legacy session cannot be upgraded safely", session.ManifestVersion)
	}
	if len(manifest.PrintAttempts) > 0 {
		last := manifest.PrintAttempts[len(manifest.PrintAttempts)-1]
		manualRetryAllowed := last.Status == session.PrintStatusFailed
		if resolution, ok := session.PrintResolutionFor(manifest, last.AttemptID); ok {
			if resolution.Decision == session.PrintResolutionNotPrinted {
				manualRetryAllowed = true
			} else {
				return TargetOutput{}, fmt.Errorf("previous printer attempt %s was resolved as %q; another submission is not allowed", last.AttemptID, resolution.Decision)
			}
		}
		if !manualRetryAllowed {
			return TargetOutput{}, fmt.Errorf("previous printer attempt %s is %q; inspect/reconcile it before another submission", last.AttemptID, last.Status)
		}
		if last.SourceSHA256 != sourceHash {
			return TargetOutput{}, errors.New("rendered source changed after the previous failed printer attempt; refusing to reuse the session's print history")
		}
	}
	if err := ctx.Err(); err != nil {
		return TargetOutput{}, err
	}

	now := lockCreatedAt
	attempt := session.PrintAttempt{
		AttemptID:    attemptID,
		QueueName:    p.QueueName,
		DocumentName: documentName,
		SourceSHA256: sourceHash,
		Status:       session.PrintStatusSubmitting,
		RequestedAt:  now,
		UpdatedAt:    now,
	}
	if err := session.BeginPrintAttempt(&manifest, attempt); err != nil {
		return TargetOutput{}, fmt.Errorf("prepare durable printer intent: %w", err)
	}
	if err := p.persistManifest(sessionDir, manifest); err != nil {
		return TargetOutput{}, fmt.Errorf("persist printer intent before submission: %w", err)
	}

	finish := func(status, jobID string, cause error) error {
		message := ""
		if cause != nil {
			message = cause.Error()
			if len(message) > 1024 {
				message = message[:1024]
			}
		}
		if err := session.FinishPrintAttempt(&manifest, attemptID, status, jobID, message, p.now()); err != nil {
			return fmt.Errorf("update printer attempt state: %w", err)
		}
		if err := p.persistManifest(sessionDir, manifest); err != nil {
			return fmt.Errorf("persist printer attempt result: %w", err)
		}
		return nil
	}

	if err := ctx.Err(); err != nil {
		if persistErr := finish(session.PrintStatusFailed, "", err); persistErr != nil {
			return TargetOutput{}, errors.Join(err, persistErr)
		}
		return TargetOutput{}, err
	}

	targetResult, targetErr := p.Target.Execute(ctx, Job{
		SessionID:    job.SessionID,
		SourcePath:   renderedPath,
		DocumentName: attempt.DocumentName,
	})
	if targetErr != nil {
		var submissionErr *PrinterSubmissionError
		status, jobID := session.PrintStatusUncertain, ""
		if errors.As(targetErr, &submissionErr) {
			jobID = submissionErr.JobID
			if !submissionErr.Uncertain {
				status = session.PrintStatusFailed
			}
		}
		if persistErr := finish(status, jobID, targetErr); persistErr != nil {
			return TargetOutput{}, &PrinterSubmissionError{
				Uncertain: true, JobID: jobID,
				Err: errors.Join(targetErr, fmt.Errorf("persist printer error status: %w", persistErr)),
			}
		}
		if status == session.PrintStatusUncertain {
			return TargetOutput{}, &PrinterSubmissionError{Uncertain: true, JobID: jobID, Err: targetErr}
		}
		return TargetOutput{}, targetErr
	}
	if targetResult.Status != TargetQueued || strings.TrimSpace(targetResult.JobID) == "" {
		unexpected := fmt.Errorf("printer target returned status %q and job ID %q; queued status and a job ID are required", targetResult.Status, targetResult.JobID)
		if persistErr := finish(session.PrintStatusUncertain, targetResult.JobID, unexpected); persistErr != nil {
			unexpected = errors.Join(unexpected, persistErr)
		}
		return TargetOutput{}, &PrinterSubmissionError{Uncertain: true, JobID: targetResult.JobID, Err: unexpected}
	}
	if err := finish(session.PrintStatusQueued, targetResult.JobID, nil); err != nil {
		return TargetOutput{}, &PrinterSubmissionError{
			Uncertain: true, JobID: targetResult.JobID,
			Err: fmt.Errorf("Windows accepted the print job but its result was not durably recorded: %w", err),
		}
	}
	result = TargetOutput{Status: TargetQueued, JobID: targetResult.JobID}
	return result, nil
}

func (p *DurablePrinter) persistManifest(sessionDir string, manifest session.Manifest) error {
	if p.writeManifest != nil {
		return p.writeManifest(sessionDir, manifest)
	}
	return session.WriteManifest(sessionDir, manifest)
}

func hashRegularFile(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxPrinterImageBytes {
		return "", fmt.Errorf("rendered printer source must be a non-empty regular file no larger than %d bytes, not a symlink", maxPrinterImageBytes)
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func newPrintAttemptID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}
