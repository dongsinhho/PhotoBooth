package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dongsinhho/PhotoBooth/internal/booth/camera/recovery"
)

func TestHandlePreviewRecoveryEventRecordsAttemptErrorDetails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preview-recovery.jsonl")
	t.Setenv(cameraPreviewDiagnosticsEnv, path)
	t.Cleanup(func() {
		if err := closeCameraPreviewDiagnostics(); err != nil {
			t.Errorf("close preview diagnostics: %v", err)
		}
	})

	app := NewApp()
	wantErr := errors.New("Windows Media Foundation found no camera devices")
	app.handlePreviewRecoveryEvent(recovery.Event{
		Type:    "recovery_open_attempt_failed",
		Attempt: 7,
		Total:   30,
		Err:     wantErr,
	})

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read preview recovery diagnostics: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(raw), []byte{byte(10)})
	if len(lines) != 2 {
		t.Fatalf("diagnostic records=%d, want recovery event plus preview status; content=%q", len(lines), raw)
	}

	var event cameraPreviewDiagnosticRecord
	if err := json.Unmarshal(lines[0], &event); err != nil {
		t.Fatalf("decode recovery event: %v", err)
	}
	if event.Event != "recovery_open_attempt_failed" || event.Attempt != 7 || event.Total != 30 || event.Message != wantErr.Error() {
		t.Fatalf("recovery diagnostic=%+v, want attempt 7/30 and full open error", event)
	}

	var status cameraPreviewDiagnosticRecord
	if err := json.Unmarshal(lines[1], &status); err != nil {
		t.Fatalf("decode preview status: %v", err)
	}
	if status.Event != "preview_status" || status.State != "reconnecting" {
		t.Fatalf("preview status=%+v, want reconnecting", status)
	}
}

func TestRecordCameraPreviewDiagnosticWritesAppendOnlyJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preview-events.jsonl")
	t.Setenv(cameraPreviewDiagnosticsEnv, path)
	t.Cleanup(func() {
		if err := closeCameraPreviewDiagnostics(); err != nil {
			t.Errorf("close preview diagnostics: %v", err)
		}
	})

	recordCameraPreviewDiagnostic(cameraPreviewDiagnosticRecord{
		TimestampUTC: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		Event:        "frame_published",
		Sequence:     42,
		FrameBytes:   1234,
	})
	recordCameraPreviewDiagnostic(cameraPreviewDiagnosticRecord{
		TimestampUTC: time.Date(2026, 11, 1, 0, 0, 0, 200000000, time.UTC),
		Event:        "frame_acknowledged",
		Sequence:     42,
		LatencyMS:    2.5,
	})

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read diagnostic log: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(raw), []byte{byte(10)})
	if len(lines) != 2 {
		t.Fatalf("diagnostic records=%d, want 2; content=%q", len(lines), raw)
	}
	var published, acknowledged cameraPreviewDiagnosticRecord
	if err := json.Unmarshal(lines[0], &published); err != nil {
		t.Fatalf("decode publish record: %v", err)
	}
	if err := json.Unmarshal(lines[1], &acknowledged); err != nil {
		t.Fatalf("decode ACK record: %v", err)
	}
	if published.Event != "frame_published" || published.Sequence != 42 || published.FrameBytes != 1234 {
		t.Fatalf("unexpected publish record: %+v", published)
	}
	if acknowledged.Event != "frame_acknowledged" || acknowledged.Sequence != 42 || acknowledged.LatencyMS != 2.5 {
		t.Fatalf("unexpected ACK record: %+v", acknowledged)
	}
}

func TestRecordCameraPreviewDiagnosticSerializesConcurrentAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preview-events.jsonl")
	t.Setenv(cameraPreviewDiagnosticsEnv, path)
	t.Cleanup(func() {
		if err := closeCameraPreviewDiagnostics(); err != nil {
			t.Errorf("close preview diagnostics: %v", err)
		}
	})

	const writers = 12
	const recordsPerWriter = 25
	var wg sync.WaitGroup
	for writer := 0; writer < writers; writer++ {
		writer := writer
		wg.Add(1)
		go func() {
			defer wg.Done()
			for record := 0; record < recordsPerWriter; record++ {
				sequence := uint64(writer*recordsPerWriter + record + 1)
				recordCameraPreviewDiagnostic(cameraPreviewDiagnosticRecord{
					TimestampUTC: time.Now().UTC(),
					Event:        "concurrent_test",
					Sequence:     sequence,
				})
			}
		}()
	}
	wg.Wait()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read diagnostic log: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(raw), []byte{byte(10)})
	if len(lines) != writers*recordsPerWriter {
		t.Fatalf("diagnostic records=%d, want %d", len(lines), writers*recordsPerWriter)
	}
	seen := make(map[uint64]bool, len(lines))
	for index, line := range lines {
		var record cameraPreviewDiagnosticRecord
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatalf("decode concurrent record %d: %v; line=%q", index, err, line)
		}
		if record.Event != "concurrent_test" || record.Sequence == 0 {
			t.Fatalf("unexpected concurrent record %d: %+v", index, record)
		}
		if seen[record.Sequence] {
			t.Fatalf("duplicate concurrent sequence %d", record.Sequence)
		}
		seen[record.Sequence] = true
	}
	if len(seen) != writers*recordsPerWriter {
		t.Fatalf("unique records=%d, want %d", len(seen), writers*recordsPerWriter)
	}
}

func TestRecordCameraPreviewDiagnosticUsesCurrentPath(t *testing.T) {
	root := t.TempDir()
	firstPath := filepath.Join(root, "first.jsonl")
	secondPath := filepath.Join(root, "second.jsonl")

	t.Setenv(cameraPreviewDiagnosticsEnv, firstPath)
	t.Cleanup(func() {
		if err := closeCameraPreviewDiagnostics(); err != nil {
			t.Errorf("close preview diagnostics: %v", err)
		}
	})
	recordCameraPreviewDiagnostic(cameraPreviewDiagnosticRecord{
		TimestampUTC: time.Now().UTC(),
		Event:        "first_path",
		Sequence:     1,
	})

	t.Setenv(cameraPreviewDiagnosticsEnv, secondPath)
	recordCameraPreviewDiagnostic(cameraPreviewDiagnosticRecord{
		TimestampUTC: time.Now().UTC(),
		Event:        "second_path",
		Sequence:     2,
	})
	for _, item := range []struct {
		path         string
		wantEvent    string
		wantSequence uint64
	}{
		{path: firstPath, wantEvent: "first_path", wantSequence: 1},
		{path: secondPath, wantEvent: "second_path", wantSequence: 2},
	} {
		raw, err := os.ReadFile(item.path)
		if err != nil {
			t.Fatalf("read %s: %v", item.path, err)
		}
		var record cameraPreviewDiagnosticRecord
		if err := json.Unmarshal(bytes.TrimSpace(raw), &record); err != nil {
			t.Fatalf("decode %s: %v", item.path, err)
		}
		if record.Event != item.wantEvent || record.Sequence != item.wantSequence {
			t.Fatalf("%s record=%+v, want event=%q sequence=%d", item.path, record, item.wantEvent, item.wantSequence)
		}
	}
}

func TestRecordCameraPreviewDiagnosticRetriesAfterWriteFailure(t *testing.T) {
	root := t.TempDir()
	validPath := filepath.Join(root, "valid.jsonl")
	invalidPath := filepath.Join(root, "missing", "preview-events.jsonl")
	beforeFailures := previewDiagnosticsWriteFailureCount()

	t.Setenv(cameraPreviewDiagnosticsEnv, invalidPath)
	t.Cleanup(func() {
		if err := closeCameraPreviewDiagnostics(); err != nil {
			t.Errorf("close preview diagnostics: %v", err)
		}
	})
	recordCameraPreviewDiagnostic(cameraPreviewDiagnosticRecord{
		TimestampUTC: time.Now().UTC(),
		Event:        "expected_open_failure",
		Sequence:     1,
	})
	if got := previewDiagnosticsWriteFailureCount(); got != beforeFailures+1 {
		t.Fatalf("diagnostic write failures=%d, want %d", got, beforeFailures+1)
	}

	t.Setenv(cameraPreviewDiagnosticsEnv, validPath)
	recordCameraPreviewDiagnostic(cameraPreviewDiagnosticRecord{
		TimestampUTC: time.Now().UTC(),
		Event:        "recovered_write",
		Sequence:     2,
	})
	if err := closeCameraPreviewDiagnostics(); err != nil {
		t.Fatalf("close recovered diagnostic writer: %v", err)
	}

	raw, err := os.ReadFile(validPath)
	if err != nil {
		t.Fatalf("read recovered diagnostic log: %v", err)
	}
	var record cameraPreviewDiagnosticRecord
	if err := json.Unmarshal(bytes.TrimSpace(raw), &record); err != nil {
		t.Fatalf("decode recovered diagnostic record: %v", err)
	}
	if record.Event != "recovered_write" || record.Sequence != 2 {
		t.Fatalf("unexpected recovered diagnostic record: %+v", record)
	}
}

func TestReportCameraPreviewDecodeFailureRequiresCurrentFrame(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preview-events.jsonl")
	t.Setenv(cameraPreviewDiagnosticsEnv, path)
	t.Cleanup(func() {
		if err := closeCameraPreviewDiagnostics(); err != nil {
			t.Errorf("close preview diagnostics: %v", err)
		}
	})
	app := NewApp()
	app.previewDone = make(chan struct{})
	app.previewInFlight = 9

	if err := app.ReportCameraPreviewDecodeFailure(0); err == nil {
		t.Fatal("zero sequence should fail")
	}
	if err := app.ReportCameraPreviewDecodeFailure(8); err == nil {
		t.Fatal("stale sequence should fail")
	}
	if err := app.ReportCameraPreviewDecodeFailure(9); err != nil {
		t.Fatalf("current frame decode failure: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read diagnostic log: %v", err)
	}
	var record cameraPreviewDiagnosticRecord
	if err := json.Unmarshal(bytes.TrimSpace(raw), &record); err != nil {
		t.Fatalf("decode diagnostic record: %v", err)
	}
	if record.Event != "frame_decode_failed" || record.Sequence != 9 {
		t.Fatalf("unexpected decode-failure record: %+v", record)
	}
}

func TestAcknowledgeCameraPreviewRequiresCurrentInFlightFrame(t *testing.T) {
	app := NewApp()
	if err := app.AcknowledgeCameraPreview(1); err == nil {
		t.Fatal("acknowledgement without a preview run should fail")
	}
	app.previewDone = make(chan struct{})
	app.previewAck = make(chan uint64, 1)
	app.previewInFlight = 7
	if err := app.AcknowledgeCameraPreview(0); err == nil {
		t.Fatal("zero sequence should fail")
	}
	if err := app.AcknowledgeCameraPreview(6); err == nil {
		t.Fatal("stale sequence should fail")
	}
	if err := app.AcknowledgeCameraPreview(7); err != nil {
		t.Fatalf("current sequence acknowledgement: %v", err)
	}
	if !app.previewAckPending {
		t.Fatal("accepted acknowledgement must be marked pending")
	}
	if err := app.AcknowledgeCameraPreview(7); err == nil {
		t.Fatal("a duplicate acknowledgement must not queue twice")
	}
	if got := <-app.previewAck; got != 7 {
		t.Fatalf("acknowledged sequence=%d, want 7", got)
	}
	app.previewAckPending = false
	app.previewInFlight = 0
	if err := app.AcknowledgeCameraPreview(7); err == nil {
		t.Fatal("an already-consumed frame must no longer be acknowledgeable")
	}
}

func TestClearCameraPreviewRunOnlyClearsMatchingRun(t *testing.T) {
	app := NewApp()
	done := make(chan struct{})
	close(done)
	app.previewDone = make(chan struct{})
	app.previewAck = make(chan uint64, 1)
	app.previewInFlight = 3
	if err := app.clearCameraPreviewRun(done); err != nil {
		t.Fatalf("clear mismatched run: %v", err)
	}
	if app.previewDone == nil || app.previewInFlight != 3 {
		t.Fatal("mismatched run must not clear current preview state")
	}
	current := app.previewDone
	app.previewCloseErr = errors.New("native camera worker did not close")
	if err := app.clearCameraPreviewRun(current); !errors.Is(err, app.previewCloseErr) {
		t.Fatalf("clear current run error=%v, want sticky native close error", err)
	}
	if app.previewDone != nil || app.previewAck != nil || app.previewInFlight != 0 {
		t.Fatal("completed preview run was not cleared")
	}
	if app.previewCloseErr == nil {
		t.Fatal("native close error must remain latched to prevent a second camera open")
	}
}

func TestPreviewCloseFailureBlocksFutureCameraSessions(t *testing.T) {
	app := NewApp()
	closeFailure := errors.New("native camera worker did not release device")
	app.previewCloseErr = closeFailure
	if err := app.StopCameraPreview(); !errors.Is(err, closeFailure) {
		t.Fatalf("StopCameraPreview error=%v, want preserved close error", err)
	}
	if err := app.StopCameraPreview(); !errors.Is(err, closeFailure) {
		t.Fatalf("second StopCameraPreview error=%v, close error should remain latched", err)
	}
	if _, err := app.StartDemoPhotoSession(); !errors.Is(err, closeFailure) {
		t.Fatalf("StartDemoPhotoSession error=%v, expected fail-closed camera lifecycle", err)
	}
	if app.player.ID != "" || app.playerRunning {
		t.Fatalf("session must not start while previous camera cleanup is uncertain: %+v", app.player)
	}
}
