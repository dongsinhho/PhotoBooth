package session

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const recoveryTestID = "0123456789abcdef0123456789abcdef"

func recoveryTestManifest() Manifest {
	frameDigest := sha256.Sum256([]byte("frame"))
	configDigest := sha256.Sum256([]byte("run-config"))
	manifest := NewManifest(
		recoveryTestID,
		3,
		"C:\\frames\\frame.json",
		hex.EncodeToString(frameDigest[:]),
		hex.EncodeToString(configDigest[:]),
	)
	manifest.Version = LegacyManifestVersion
	return manifest
}

func writeRecoveryJPEG(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: uint8(40 + x*20), G: uint8(60 + y*20), B: 120, A: 255})
		}
	}
	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: 80}); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func truncateJPEGScanPayload(t *testing.T, data []byte) []byte {
	t.Helper()
	marker := bytes.Index(data, []byte{0xff, 0xda})
	if marker < 0 || marker+4 > len(data) {
		t.Fatal("JPEG fixture has no complete start-of-scan marker")
	}
	segmentLength := int(data[marker+2])<<8 | int(data[marker+3])
	scanStart := marker + 2 + segmentLength
	if scanStart >= len(data)-2 {
		t.Fatalf("JPEG fixture has no scan payload to truncate: size=%d scan_start=%d", len(data), scanStart)
	}
	truncated := append([]byte(nil), data[:scanStart]...)
	return append(truncated, 0xff, 0xd9)
}

func writeRecoveryCaptures(t *testing.T, sessionDir string, count int) {
	t.Helper()
	for i := 1; i <= count; i++ {
		writeRecoveryJPEG(t, filepath.Join(sessionDir, "captures", captureFileName(i)))
	}
}

func captureFileName(index int) string {
	return "capture-0" + string(rune('0'+index)) + ".jpg"
}

func TestValidateJPEGFileRejectsTruncatedPayloadAfterValidHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "truncated.jpg")
	writeRecoveryJPEG(t, path)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(original) < 64 {
		t.Fatalf("fixture JPEG is unexpectedly small: %d bytes", len(original))
	}
	truncated := truncateJPEGScanPayload(t, original)
	if err := os.WriteFile(path, truncated, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateJPEGHeader(path); err != nil {
		t.Fatalf("header-only discovery should accept the intact JPEG header: %v", err)
	}
	if err := ValidateJPEGFile(path); err == nil {
		t.Fatal("full JPEG validation accepted a truncated image payload")
	}
}

func TestManifestWriteReadIsAtomicAndStrict(t *testing.T) {
	sessionDir := filepath.Join(t.TempDir(), recoveryTestID)
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := recoveryTestManifest()
	if err := WriteManifest(sessionDir, manifest); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	got, err := ReadManifest(sessionDir)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if got.SessionID != manifest.SessionID || got.Phase != PhasePreflight {
		t.Fatalf("manifest=%+v", got)
	}
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != ManifestFileName {
		t.Fatalf("unexpected manifest directory entries: %+v", entries)
	}

	// A second update replaces the prior manifest rather than leaving temp files.
	manifest.Phase = PhaseCapturing
	manifest.UpdatedAt = time.Now().UTC()
	if err := WriteManifest(sessionDir, manifest); err != nil {
		t.Fatalf("update manifest: %v", err)
	}
	got, err = ReadManifest(sessionDir)
	if err != nil || got.Phase != PhaseCapturing {
		t.Fatalf("updated manifest=%+v err=%v", got, err)
	}
}

func TestManifestRejectsUnknownFieldsAndIdentityMismatch(t *testing.T) {
	sessionDir := filepath.Join(t.TempDir(), recoveryTestID)
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := recoveryTestManifest()
	if err := WriteManifest(sessionDir, manifest); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sessionDir, ManifestFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data[:len(data)-1], []byte(",\"unexpected\":true}")...)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadManifest(sessionDir); err == nil {
		t.Fatal("expected unknown manifest field to be rejected")
	}

	otherDir := filepath.Join(filepath.Dir(sessionDir), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err := os.MkdirAll(otherDir, 0o700); err != nil {
		t.Fatal(err)
	}
	manifest = recoveryTestManifest()
	if err := WriteManifest(otherDir, manifest); err == nil {
		t.Fatal("expected manifest/session directory identity mismatch")
	}
}

func TestInspectRecoveryRootClassifiesSafePhasesReadOnly(t *testing.T) {
	root := t.TempDir()

	preflightDir := filepath.Join(root, recoveryTestID)
	if err := os.MkdirAll(filepath.Join(preflightDir, "captures"), 0o700); err != nil {
		t.Fatal(err)
	}
	preflight := recoveryTestManifest()
	if err := WriteManifest(preflightDir, preflight); err != nil {
		t.Fatal(err)
	}

	capturingID := "1123456789abcdef0123456789abcdef"
	capturingDir := filepath.Join(root, capturingID)
	if err := os.MkdirAll(capturingDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRecoveryCaptures(t, capturingDir, 1)
	capturing := recoveryTestManifest()
	capturing.SessionID = capturingID
	capturing.Phase = PhaseCapturing
	capturing.CompletedShots = 1
	capturing.UpdatedAt = time.Now().UTC()
	if err := WriteManifest(capturingDir, capturing); err != nil {
		t.Fatal(err)
	}

	renderedID := "2123456789abcdef0123456789abcdef"
	renderedDir := filepath.Join(root, renderedID)
	if err := os.MkdirAll(renderedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRecoveryCaptures(t, renderedDir, 3)
	writeRecoveryJPEG(t, filepath.Join(renderedDir, "rendered.jpg"))
	rendered := recoveryTestManifest()
	rendered.SessionID = renderedID
	rendered.Phase = PhaseRendered
	rendered.CompletedShots = 3
	rendered.RenderedAt = time.Now().UTC()
	rendered.UpdatedAt = rendered.RenderedAt
	if err := WriteManifest(renderedDir, rendered); err != nil {
		t.Fatal(err)
	}

	finalID := "3123456789abcdef0123456789abcdef"
	finalDir := filepath.Join(root, finalID)
	if err := os.MkdirAll(finalDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRecoveryCaptures(t, finalDir, 3)
	writeRecoveryJPEG(t, filepath.Join(finalDir, "rendered.jpg"))
	final := recoveryTestManifest()
	final.SessionID = finalID
	final.Phase = PhaseOutputFinalized
	final.CompletedShots = 3
	final.RenderedAt = time.Now().UTC()
	final.OutputPath = filepath.Join(root, "output", finalID, "photo.jpg")
	final.OutputFinalizedAt = final.RenderedAt.Add(time.Second)
	final.UpdatedAt = final.OutputFinalizedAt
	if err := WriteManifest(finalDir, final); err != nil {
		t.Fatal(err)
	}

	missingID := "4123456789abcdef0123456789abcdef"
	missingDir := filepath.Join(root, missingID)
	if err := os.MkdirAll(filepath.Join(missingDir, "captures"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeRecoveryCaptures(t, missingDir, 1)

	unknownID := "5123456789abcdef0123456789abcdef"
	unknownDir := filepath.Join(root, unknownID)
	if err := os.MkdirAll(filepath.Join(unknownDir, "captures"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unknownDir, "operator-notes.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	unknown := recoveryTestManifest()
	unknown.SessionID = unknownID
	if err := WriteManifest(unknownDir, unknown); err != nil {
		t.Fatal(err)
	}

	before := make(map[string][]byte)
	for _, dir := range []string{preflightDir, capturingDir, renderedDir, finalDir, missingDir, unknownDir} {
		filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				data, readErr := os.ReadFile(path)
				if readErr == nil {
					before[path] = data
				}
			}
			return nil
		})
	}

	scan, err := InspectRecoveryRoot(root)
	if err != nil {
		t.Fatalf("InspectRecoveryRoot: %v", err)
	}
	got := make(map[string]RecoveryItem)
	for _, item := range scan.Items {
		got[item.SessionID] = item
	}
	for id, want := range map[string]RecoveryStatus{
		recoveryTestID: RecoveryResumeAvailable,
		capturingID:    RecoveryResumeAvailable,
		renderedID:     RecoveryOutputPending,
		finalID:        RecoveryFinalized,
		missingID:      RecoveryInspectOnly,
		unknownID:      RecoveryInspectOnly,
	} {
		if item, ok := got[id]; !ok || item.Status != want {
			t.Fatalf("session %s item=%+v exists=%v, want status %s", id, item, ok, want)
		}
	}

	for path, prior := range before {
		after, readErr := os.ReadFile(path)
		if readErr != nil || string(after) != string(prior) {
			t.Fatalf("discovery modified or removed %s: err=%v", path, readErr)
		}
	}
	for _, dir := range []string{preflightDir, capturingDir, renderedDir, finalDir, missingDir, unknownDir} {
		entries, readErr := os.ReadDir(dir)
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, entry := range entries {
			if len(entry.Name()) >= len(".manifest-") && entry.Name()[:len(".manifest-")] == ".manifest-" {
				t.Fatalf("discovery left/created a temporary manifest: %s", entry.Name())
			}
		}
	}
}

func TestReadDirectoryLimitedStopsAfterLimitPlusOne(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a", "b", "c", "d"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	entries, tooMany, err := readDirectoryLimited(dir, 2)
	if err != nil {
		t.Fatalf("readDirectoryLimited: %v", err)
	}
	if !tooMany || len(entries) != 2 {
		t.Fatalf("entries=%d tooMany=%v, want 2 entries and tooMany=true", len(entries), tooMany)
	}
	if entries[0].Name() >= entries[1].Name() {
		t.Fatalf("limited entries are not sorted: %+v", entries)
	}
}

func TestValidateManifestRejectsInconsistentPhaseTimestamps(t *testing.T) {
	base := recoveryTestManifest()
	base.Phase = PhaseRendered
	base.CompletedShots = base.ExpectedShots
	base.RenderedAt = base.UpdatedAt
	if err := ValidateManifest(base); err != nil {
		t.Fatalf("valid rendered manifest rejected: %v", err)
	}

	preflightWithRender := recoveryTestManifest()
	preflightWithRender.RenderedAt = preflightWithRender.UpdatedAt
	if err := ValidateManifest(preflightWithRender); err == nil {
		t.Fatal("preflight manifest with render timestamp was accepted")
	}

	futureRender := base
	futureRender.RenderedAt = futureRender.UpdatedAt.Add(time.Second)
	if err := ValidateManifest(futureRender); err == nil {
		t.Fatal("render timestamp after updated_at was accepted")
	}

	finalized := base
	finalized.Phase = PhaseOutputFinalized
	finalized.OutputPath = filepath.Join(t.TempDir(), "photo.jpg")
	finalized.OutputFinalizedAt = finalized.RenderedAt.Add(-time.Second)
	if err := ValidateManifest(finalized); err == nil {
		t.Fatal("output finalization before render was accepted")
	}
}

func TestInspectRecoveryRootRejectsManifestCaptureCountMismatch(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, recoveryTestID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRecoveryCaptures(t, dir, 2)
	m := recoveryTestManifest()
	m.Phase = PhaseCapturing
	m.CompletedShots = 0
	m.UpdatedAt = time.Now().UTC()
	if err := WriteManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	scan, err := InspectRecoveryRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Items) != 1 || scan.Items[0].Status != RecoveryInspectOnly {
		t.Fatalf("scan=%+v, want inspect-only", scan)
	}
}

func TestInspectRecoveryRootRejectsRedirectedSessionDirectory(t *testing.T) {
	if os.PathSeparator != '\\' {
		t.Skip("Windows junction behavior is covered by Windows-specific storage path tests")
	}
	root := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, recoveryTestID)
	if err := os.MkdirAll(filepath.Join(target, "captures"), 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, recoveryTestID)
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink/junction creation unavailable: %v", err)
	}
	scan, err := InspectRecoveryRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Items) != 1 || scan.Items[0].Status != RecoveryInspectOnly {
		t.Fatalf("scan=%+v, want inspect-only redirected entry", scan)
	}
}
func TestInspectRecoveryRootTreatsPartialStagingArtifactsAsResumableAndReadOnly(t *testing.T) {
	root := t.TempDir()
	const id = "6123456789abcdef0123456789abcdef"
	sessionDir := filepath.Join(root, id)
	capturesDir := filepath.Join(sessionDir, "captures")
	stagingAttempt := filepath.Join(capturesDir, CaptureStagingDirectoryName, "shot-01-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err := os.MkdirAll(stagingAttempt, 0o700); err != nil {
		t.Fatal(err)
	}
	partial := []byte("interrupted camera write; not a complete JPEG")
	partialPath := filepath.Join(stagingAttempt, "capture.jpg")
	if err := os.WriteFile(partialPath, partial, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := recoveryTestManifest()
	manifest.SessionID = id
	manifest.Phase = PhaseCapturing
	manifest.UpdatedAt = time.Now().UTC()
	if err := WriteManifest(sessionDir, manifest); err != nil {
		t.Fatal(err)
	}

	scan, err := InspectRecoveryRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Items) != 1 || scan.Items[0].Status != RecoveryResumeAvailable || scan.Items[0].CompletedShots != 0 {
		t.Fatalf("scan=%+v, want resumable session with zero committed captures", scan)
	}
	after, err := os.ReadFile(partialPath)
	if err != nil || string(after) != string(partial) {
		t.Fatalf("read-only discovery changed staged bytes: got=%q err=%v", after, err)
	}
	if exists, err := ValidateCaptureStagingDirectory(capturesDir); err != nil || !exists {
		t.Fatalf("ValidateCaptureStagingDirectory exists=%v err=%v", exists, err)
	}
}

func TestInspectRecoveryRootRejectsUnknownCaptureStagingArtifacts(t *testing.T) {
	root := t.TempDir()
	const id = "7123456789abcdef0123456789abcdef"
	sessionDir := filepath.Join(root, id)
	attemptDir := filepath.Join(sessionDir, "captures", CaptureStagingDirectoryName, "shot-01-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err := os.MkdirAll(attemptDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attemptDir, "operator-data.txt"), []byte("do not delete"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := recoveryTestManifest()
	manifest.SessionID = id
	manifest.Phase = PhaseCapturing
	if err := WriteManifest(sessionDir, manifest); err != nil {
		t.Fatal(err)
	}
	scan, err := InspectRecoveryRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Items) != 1 || scan.Items[0].Status != RecoveryInspectOnly {
		t.Fatalf("scan=%+v, want inspect-only unknown staging artifact", scan)
	}
}

func TestFrameSnapshotIsImmutableAndManifestBound(t *testing.T) {
	sessionDir := filepath.Join(t.TempDir(), recoveryTestID)
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	frameBytes := []byte("{\"schema_version\":1,\"frame\":\"original\"}")
	manifest := recoveryTestManifest()
	manifest.Version = ManifestVersion
	frameDigest := sha256.Sum256(frameBytes)
	manifest.FrameConfigSHA256 = hex.EncodeToString(frameDigest[:])

	path, digest, err := WriteFrameSnapshot(sessionDir, frameBytes)
	if err != nil {
		t.Fatalf("WriteFrameSnapshot: %v", err)
	}
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != FrameSnapshotFileName {
		t.Fatalf("snapshot publication left temporary entries: %+v", entries)
	}
	manifest.FrameSnapshotFile = FrameSnapshotFileName
	manifest.FrameSnapshotSHA256 = digest
	if err := ValidateManifest(manifest); err != nil {
		t.Fatalf("snapshot manifest validation: %v", err)
	}
	gotPath, gotBytes, err := ReadFrameSnapshot(sessionDir, manifest)
	if err != nil {
		t.Fatalf("ReadFrameSnapshot: %v", err)
	}
	if gotPath != path || !bytes.Equal(gotBytes, frameBytes) {
		t.Fatalf("snapshot path/bytes mismatch: path=%q bytes=%q", gotPath, gotBytes)
	}

	replacement := []byte("{\"schema_version\":1,\"frame\":\"changed\"}")
	if _, _, err := WriteFrameSnapshot(sessionDir, replacement); err == nil {
		t.Fatal("WriteFrameSnapshot replaced an existing immutable snapshot")
	}
	gotBytes, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(gotBytes, frameBytes) {
		t.Fatalf("existing snapshot changed: bytes=%q err=%v", gotBytes, err)
	}

	if err := os.WriteFile(path, replacement, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadFrameSnapshot(sessionDir, manifest); err == nil {
		t.Fatal("ReadFrameSnapshot accepted bytes with a mismatched manifest digest")
	}
}

func TestInspectRecoveryRootUsesVerifiedFrameSnapshot(t *testing.T) {
	root := t.TempDir()
	sessionDir := filepath.Join(root, recoveryTestID)
	if err := os.MkdirAll(filepath.Join(sessionDir, "captures"), 0o700); err != nil {
		t.Fatal(err)
	}
	frameBytes := []byte("{\"schema_version\":1,\"frame\":\"snapshot\"}")
	manifest := recoveryTestManifest()
	manifest.Version = ManifestVersion
	frameDigest := sha256.Sum256(frameBytes)
	manifest.FrameConfigSHA256 = hex.EncodeToString(frameDigest[:])
	path, digest, err := WriteFrameSnapshot(sessionDir, frameBytes)
	if err != nil {
		t.Fatal(err)
	}
	manifest.FrameSnapshotFile = FrameSnapshotFileName
	manifest.FrameSnapshotSHA256 = digest
	if err := WriteManifest(sessionDir, manifest); err != nil {
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
	if item.Status != RecoveryResumeAvailable || item.FrameConfigPath != path {
		t.Fatalf("recovery item=%+v, want resume with snapshot path %q", item, path)
	}
}

func TestInspectRecoveryRootRejectsCorruptDeclaredFrameSnapshot(t *testing.T) {
	root := t.TempDir()
	sessionDir := filepath.Join(root, recoveryTestID)
	if err := os.MkdirAll(filepath.Join(sessionDir, "captures"), 0o700); err != nil {
		t.Fatal(err)
	}
	frameBytes := []byte("{\"schema_version\":1,\"frame\":\"snapshot\"}")
	manifest := recoveryTestManifest()
	manifest.Version = ManifestVersion
	frameDigest := sha256.Sum256(frameBytes)
	manifest.FrameConfigSHA256 = hex.EncodeToString(frameDigest[:])
	path, digest, err := WriteFrameSnapshot(sessionDir, frameBytes)
	if err != nil {
		t.Fatal(err)
	}
	manifest.FrameSnapshotFile = FrameSnapshotFileName
	manifest.FrameSnapshotSHA256 = digest
	if err := WriteManifest(sessionDir, manifest); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}

	scan, err := InspectRecoveryRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Items) != 1 || scan.Items[0].Status != RecoveryInspectOnly {
		t.Fatalf("corrupt snapshot should be inspect-only: %+v", scan.Items)
	}
}

func TestManifestV2RequiresExplicitDeliveryAndPaymentPolicies(t *testing.T) {
	manifest := recoveryTestManifest()
	manifest.Version = ManifestVersion
	manifest.FrameSnapshotFile = FrameSnapshotFileName
	manifest.FrameSnapshotSHA256 = manifest.FrameConfigSHA256
	if err := ValidateManifest(manifest); err != nil {
		t.Fatalf("valid v2 policy metadata rejected: %v", err)
	}

	missingDelivery := manifest
	missingDelivery.MediaDeliveryMode = ""
	if err := ValidateManifest(missingDelivery); err == nil || !strings.Contains(err.Error(), "media_delivery_mode") {
		t.Fatalf("missing delivery policy error=%v, want media_delivery_mode rejection", err)
	}

	missingPayment := manifest
	missingPayment.PaymentMode = ""
	if err := ValidateManifest(missingPayment); err == nil || !strings.Contains(err.Error(), "payment_mode") {
		t.Fatalf("missing payment policy error=%v, want payment_mode rejection", err)
	}
}
