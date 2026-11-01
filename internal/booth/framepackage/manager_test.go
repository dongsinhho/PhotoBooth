package framepackage

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixtureFrame(t *testing.T, dir, filename string, count int) string {
	t.Helper()
	slots := make([]string, 0, count)
	for i := 0; i < count; i++ {
		slots = append(slots, fmt.Sprintf("{\"id\":\"photo-%d\",\"x\":%d,\"y\":0,\"width\":320,\"height\":640,\"fit\":\"cover\"}", i+1, i*320))
	}
	data := fmt.Sprintf("{\"schema_version\":1,\"name\":\"Fixture\",\"canvas\":{\"width\":%d,\"height\":640,\"background\":\"#F7F3F7\"},\"slots\":[%s],\"jpeg_quality\":90}", count*320, strings.Join(slots, ","))
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFixturePreview(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "preview.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 12, 12))
	for y := 0; y < 12; y++ {
		for x := 0; x < 12; x++ {
			img.Set(x, y, color.RGBA{R: 220, G: 40, B: 140, A: 255})
		}
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func createFixturePackage(t *testing.T, dir, framePath, name, version string) string {
	t.Helper()
	output := filepath.Join(dir, name+"-"+version+ArchiveExtension)
	_, err := CreatePackage(context.Background(), CreateOptions{
		FramePath: framePath, OutputPath: output, PackageID: "sample-frame", PackageVersion: version,
		Name: name, Description: "A test frame", Author: "PhotoBooth Test", License: "CC0-1.0",
	})
	if err != nil {
		t.Fatalf("CreatePackage: %v", err)
	}
	return output
}

func rewriteArchive(t *testing.T, source, dest string, mutate func(map[string][]byte)) {
	t.Helper()
	reader, err := zip.OpenReader(source)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	entries := make(map[string][]byte, len(reader.File))
	for _, entry := range reader.File {
		f, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		var data bytes.Buffer
		if _, err := data.ReadFrom(f); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		entries[entry.Name] = data.Bytes()
	}
	mutate(entries)
	out, err := os.Create(dest)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(out)
	for name, data := range entries {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0o600)
		entry, err := w.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateValidateInstallListAndIdempotence(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	framePath := writeFixtureFrame(t, dir, "source.json", 3)
	previewPath := writeFixturePreview(t, dir)
	licensePath := filepath.Join(dir, "LICENSE.txt")
	if err := os.WriteFile(licensePath, []byte("Creator grants permission under CC0-1.0.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "holiday-strip"+ArchiveExtension)
	created, err := CreatePackage(ctx, CreateOptions{
		FramePath: framePath, OutputPath: archive, PackageID: "holiday-strip", PackageVersion: "1.2.0",
		Name: "Holiday strip", Description: "A local frame", Author: "Creator", License: "CC0-1.0",
		PreviewPath: previewPath, LicensePath: licensePath,
	})
	if err != nil {
		t.Fatalf("CreatePackage: %v", err)
	}
	if created.SlotCount != 3 || !created.HasPreview || created.PackageID != "holiday-strip" {
		t.Fatalf("unexpected created metadata: %+v", created)
	}
	original, err := os.ReadFile(framePath)
	if err != nil {
		t.Fatal(err)
	}

	manager := NewManager(filepath.Join(dir, "managed-frames"))
	validated, err := manager.Validate(ctx, archive)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if validated.SlotCount != 3 || !validated.HasPreview || validated.FramePath != "" {
		t.Fatalf("unexpected validation metadata: %+v", validated)
	}
	if _, err := os.Stat(filepath.Join(dir, "managed-frames", "packages")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Validate should not install content; stat error=%v", err)
	}

	installed, err := manager.Install(ctx, archive)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if installed.SlotCount != 3 || installed.FramePath == "" || installed.PreviewPath == "" || installed.AlreadyInstalled {
		t.Fatalf("unexpected installed package: %+v", installed)
	}
	if _, err := os.Stat(installed.FramePath); err != nil {
		t.Fatalf("installed frame missing: %v", err)
	}
	if _, err := os.Stat(installed.PreviewPath); err != nil {
		t.Fatalf("installed preview missing: %v", err)
	}
	again, err := manager.Install(ctx, archive)
	if err != nil {
		t.Fatalf("idempotent install: %v", err)
	}
	if !again.AlreadyInstalled {
		t.Fatalf("second install should be idempotent: %+v", again)
	}
	packages, err := manager.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(packages) != 1 || packages[0].PackageID != "holiday-strip" || packages[0].PackageVersion != "1.2.0" {
		t.Fatalf("unexpected installed list: %+v", packages)
	}
	after, err := os.ReadFile(framePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, after) {
		t.Fatal("package creation modified the source frame")
	}
}

func TestInstallRefusesDifferentContentForImmutableVersion(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	frame1 := writeFixtureFrame(t, dir, "frame1.json", 2)
	frame2 := writeFixtureFrame(t, dir, "frame2.json", 3)
	p1 := createFixturePackage(t, dir, frame1, "First", "1.0.0")
	p2 := filepath.Join(dir, "different"+ArchiveExtension)
	_, err := CreatePackage(ctx, CreateOptions{FramePath: frame2, OutputPath: p2, PackageID: "sample-frame", PackageVersion: "1.0.0", Name: "Different", Author: "Tester", License: "CC0-1.0"})
	if err != nil {
		t.Fatal(err)
	}
	manager := NewManager(filepath.Join(dir, "installed"))
	if _, err := manager.Install(ctx, p1); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Install(ctx, p2); err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("Install error=%v, want immutable-version conflict", err)
	}
}

func TestValidateRejectsDigestTamperingAndUnknownEntries(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	frame := writeFixtureFrame(t, dir, "frame.json", 3)
	archive := createFixturePackage(t, dir, frame, "Valid", "1.0.0")
	manager := NewManager(filepath.Join(dir, "installed"))

	tampered := filepath.Join(dir, "tampered"+ArchiveExtension)
	rewriteArchive(t, archive, tampered, func(files map[string][]byte) {
		files["definition.json"] = bytes.Replace(files["definition.json"], []byte("Fixture"), []byte("Picture"), 1)
	})
	if _, err := manager.Validate(ctx, tampered); err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("tampered package error=%v", err)
	}

	unknown := filepath.Join(dir, "unknown"+ArchiveExtension)
	rewriteArchive(t, archive, unknown, func(files map[string][]byte) { files["../escape.json"] = []byte("{}") })
	if _, err := manager.Validate(ctx, unknown); err == nil || !strings.Contains(err.Error(), "unsafe or unsupported") {
		t.Fatalf("unsafe entry error=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "escape.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unsafe archive created escaped file: %v", err)
	}
}

func TestCreatePackageValidatesInputsAndNeverOverwritesOutput(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	frame := writeFixtureFrame(t, dir, "frame.json", 2)
	output := filepath.Join(dir, "output"+ArchiveExtension)
	opts := CreateOptions{FramePath: frame, OutputPath: output, PackageID: "good-id", PackageVersion: "1.0.0", Name: "Frame", Author: "Author", License: "CC0-1.0"}
	if _, err := CreatePackage(ctx, opts); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreatePackage(ctx, opts); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("second CreatePackage error=%v", err)
	}
	after, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("attempted overwrite changed existing archive")
	}
	opts.PackageID = "../bad"
	opts.OutputPath = filepath.Join(dir, "bad"+ArchiveExtension)
	if _, err := CreatePackage(ctx, opts); err == nil {
		t.Fatal("invalid package ID should be rejected")
	}
	opts.PackageID = "good-id"
	opts.PackageVersion = "01.0.0"
	if _, err := CreatePackage(ctx, opts); err == nil {
		t.Fatal("noncanonical version should be rejected")
	}
}

func TestActivationCommitAndRollbackRestoresPriorFrame(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	oldFrame := writeFixtureFrame(t, dir, "old-frame.json", 2)
	newFrame := writeFixtureFrame(t, dir, "new-frame.json", 3)
	archive := createFixturePackage(t, dir, newFrame, "Three-shot", "1.0.0")
	manager := NewManager(filepath.Join(dir, "app", "frames"))
	installed, err := manager.Install(ctx, archive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.PrepareActivation(ctx, installed.PackageID, installed.PackageVersion, oldFrame); err != nil {
		t.Fatalf("PrepareActivation: %v", err)
	}
	if err := manager.CommitActivation(installed.FramePath); err != nil {
		t.Fatalf("CommitActivation: %v", err)
	}
	available, err := manager.CanRollback(ctx, installed.FramePath)
	if err != nil || !available {
		t.Fatalf("CanRollback=%t err=%v, want true", available, err)
	}
	restoredPath, err := manager.RollbackFramePath(ctx, installed.FramePath)
	if err != nil {
		t.Fatalf("RollbackFramePath: %v", err)
	}
	if restoredPath == oldFrame || restoredPath == installed.FramePath {
		t.Fatal("rollback should use a stable restored copy")
	}
	slots, err := renderSlotCount(ctx, restoredPath)
	if err != nil {
		t.Fatal(err)
	}
	if slots != 2 {
		t.Fatalf("restored frame has %d slots, want 2", slots)
	}
	if _, err := os.Stat(installed.FramePath); err != nil {
		t.Fatalf("rollback removed the package: %v", err)
	}
	if err := manager.ClearRollback(); err != nil {
		t.Fatal(err)
	}
	available, err = manager.CanRollback(ctx, restoredPath)
	if err != nil {
		t.Fatal(err)
	}
	if available {
		t.Fatal("rollback should be unavailable after clearing state")
	}
}

func renderSlotCount(ctx context.Context, path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return validateFrameBytes(ctx, data)
}

func TestAbortedActivationPreservesPreviousRollbackState(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	f1 := writeFixtureFrame(t, dir, "one.json", 2)
	f2 := writeFixtureFrame(t, dir, "two.json", 3)
	f3 := writeFixtureFrame(t, dir, "three.json", 1)
	manager := NewManager(filepath.Join(dir, "app", "frames"))
	p2, err := manager.Install(ctx, createFixturePackage(t, dir, f2, "Second", "1.0.0"))
	if err != nil {
		t.Fatal(err)
	}
	p3Archive := filepath.Join(dir, "third-1.0.0"+ArchiveExtension)
	if _, err := CreatePackage(ctx, CreateOptions{
		FramePath: f3, OutputPath: p3Archive, PackageID: "third-frame", PackageVersion: "1.0.0",
		Name: "Third", Author: "Tester", License: "CC0-1.0",
	}); err != nil {
		t.Fatal(err)
	}
	p3, err := manager.Install(ctx, p3Archive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.PrepareActivation(ctx, p2.PackageID, p2.PackageVersion, f1); err != nil {
		t.Fatal(err)
	}
	if err := manager.CommitActivation(p2.FramePath); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.PrepareActivation(ctx, p3.PackageID, p3.PackageVersion, p2.FramePath); err != nil {
		t.Fatal(err)
	}
	if err := manager.AbortActivation(); err != nil {
		t.Fatal(err)
	}
	available, err := manager.CanRollback(ctx, p2.FramePath)
	if err != nil || !available {
		t.Fatalf("prior rollback lost after abort: available=%t err=%v", available, err)
	}
}

func TestInterruptedActivationIsDiscardedWhenProfileStillUsesOldFrame(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	oldFrame := writeFixtureFrame(t, dir, "old.json", 2)
	newFrame := writeFixtureFrame(t, dir, "new.json", 3)
	manager := NewManager(filepath.Join(dir, "app", "frames"))
	pkg, err := manager.Install(ctx, createFixturePackage(t, dir, newFrame, "New", "1.0.0"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.PrepareActivation(ctx, pkg.PackageID, pkg.PackageVersion, oldFrame); err != nil {
		t.Fatal(err)
	}
	available, err := manager.CanRollback(ctx, oldFrame)
	if err != nil || available {
		t.Fatalf("old active path should discard uncommitted activation: available=%t err=%v", available, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "app", "frames", "rollback", "pending-v1.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pending state still exists: %v", err)
	}
}

func TestRollbackRefusesTamperedBackup(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	oldFrame := writeFixtureFrame(t, dir, "old.json", 2)
	newFrame := writeFixtureFrame(t, dir, "new.json", 3)
	manager := NewManager(filepath.Join(dir, "app", "frames"))
	pkg, err := manager.Install(ctx, createFixturePackage(t, dir, newFrame, "New", "1.0.0"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.PrepareActivation(ctx, pkg.PackageID, pkg.PackageVersion, oldFrame); err != nil {
		t.Fatal(err)
	}
	if err := manager.CommitActivation(pkg.FramePath); err != nil {
		t.Fatal(err)
	}
	state, err := manager.loadRollbackState()
	if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(manager.framesRoot, "rollback", state.BackupFileName)
	if err := os.WriteFile(backup, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.CanRollback(ctx, pkg.FramePath); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("CanRollback error=%v, want SHA-256 failure", err)
	}
}

func TestManagerRejectsCancelledContextAndInvalidRoot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	manager := NewManager(t.TempDir())
	if _, err := manager.Validate(ctx, "nope"+ArchiveExtension); !errors.Is(err, context.Canceled) {
		t.Fatalf("Validate error=%v, want cancellation", err)
	}
	invalid := NewManager("")
	if _, err := invalid.Install(context.Background(), "missing"+ArchiveExtension); err == nil {
		t.Fatal("invalid root should fail on install")
	}
}
func TestValidateRejectsMalformedPreviewEvenWhenManifestDigestMatches(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	frame := writeFixtureFrame(t, dir, "frame.json", 2)
	preview := writeFixturePreview(t, dir)
	archive := filepath.Join(dir, "valid-preview"+ArchiveExtension)
	if _, err := CreatePackage(ctx, CreateOptions{
		FramePath: frame, OutputPath: archive, PackageID: "preview-fixture", PackageVersion: "1.0.0",
		Name: "Preview fixture", Author: "Tester", License: "CC0-1.0", PreviewPath: preview,
	}); err != nil {
		t.Fatal(err)
	}

	corruptArchive := filepath.Join(dir, "corrupt-preview"+ArchiveExtension)
	rewriteArchive(t, archive, corruptArchive, func(files map[string][]byte) {
		badPreview := []byte("this is not a complete PNG image")
		files["preview.png"] = badPreview
		var manifest Manifest
		if err := json.Unmarshal(files["manifest.json"], &manifest); err != nil {
			t.Fatalf("unmarshal manifest: %v", err)
		}
		sum := sha256.Sum256(badPreview)
		for i := range manifest.Files {
			if manifest.Files[i].Path == "preview.png" {
				manifest.Files[i].Size = int64(len(badPreview))
				manifest.Files[i].SHA256 = hex.EncodeToString(sum[:])
			}
		}
		data, err := json.Marshal(manifest)
		if err != nil {
			t.Fatalf("marshal manifest: %v", err)
		}
		files["manifest.json"] = data
	})
	if _, err := NewManager(filepath.Join(dir, "managed")).Validate(ctx, corruptArchive); err == nil || !strings.Contains(err.Error(), "decode preview") {
		t.Fatalf("Validate error=%v, want full-preview decode failure", err)
	}
}

func TestListRejectsNonDirectoryPackageRoot(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "frames")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "packages"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewManager(root).List(context.Background()); err == nil || !strings.Contains(err.Error(), "installed package root must be a regular directory") {
		t.Fatalf("List error=%v, want malformed package-root error", err)
	}
}
func TestUninstallRemovesUnusedImmutableVersion(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	activeFrame := writeFixtureFrame(t, dir, "active.json", 3)
	newFrame := writeFixtureFrame(t, dir, "package-frame.json", 2)
	archive := createFixturePackage(t, dir, newFrame, "Unused", "1.0.0")
	manager := NewManager(filepath.Join(dir, "managed"))
	installed, err := manager.Install(ctx, archive)
	if err != nil {
		t.Fatal(err)
	}

	if err := manager.Uninstall(ctx, installed.PackageID, installed.PackageVersion, activeFrame); err != nil {
		t.Fatalf("Uninstall unused package: %v", err)
	}
	packages, err := manager.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 0 {
		t.Fatalf("package still listed after uninstall: %+v", packages)
	}
	if _, err := os.Stat(installed.FramePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("uninstalled definition still exists, stat error=%v", err)
	}
}

func TestUninstallBlocksActiveAndPendingActivationPackages(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	activeFrame := writeFixtureFrame(t, dir, "active.json", 3)
	nextFrame := writeFixtureFrame(t, dir, "next.json", 2)
	archive := createFixturePackage(t, dir, nextFrame, "Next", "1.0.0")
	manager := NewManager(filepath.Join(dir, "managed"))
	installed, err := manager.Install(ctx, archive)
	if err != nil {
		t.Fatal(err)
	}

	if err := manager.Uninstall(ctx, installed.PackageID, installed.PackageVersion, installed.FramePath); err == nil ||
		!strings.Contains(err.Error(), "currently active") {
		t.Fatalf("Uninstall active package error=%v, want active-package guard", err)
	}
	if _, err := manager.PrepareActivation(ctx, installed.PackageID, installed.PackageVersion, activeFrame); err != nil {
		t.Fatal(err)
	}
	if err := manager.Uninstall(ctx, installed.PackageID, installed.PackageVersion, activeFrame); err == nil ||
		!strings.Contains(err.Error(), "awaiting commit or abort") {
		t.Fatalf("Uninstall pending package error=%v, want pending-activation guard", err)
	}
	if err := manager.AbortActivation(); err != nil {
		t.Fatal(err)
	}
	if err := manager.Uninstall(ctx, installed.PackageID, installed.PackageVersion, activeFrame); err != nil {
		t.Fatalf("Uninstall after abort: %v", err)
	}
}

func TestUninstallProtectsPackageReferencedByRollback(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	oldFrame := writeFixtureFrame(t, dir, "old.json", 2)
	middleFrame := writeFixtureFrame(t, dir, "middle.json", 3)
	finalFrame := writeFixtureFrame(t, dir, "final.json", 1)
	archiveMiddle := createFixturePackage(t, dir, middleFrame, "Middle", "1.0.0")
	archiveFinal := filepath.Join(dir, "final-1.0.0"+ArchiveExtension)
	if _, err := CreatePackage(ctx, CreateOptions{
		FramePath: finalFrame, OutputPath: archiveFinal, PackageID: "final-frame", PackageVersion: "1.0.0",
		Name: "Final", Author: "Tester", License: "CC0-1.0",
	}); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(filepath.Join(dir, "managed"))
	middle, err := manager.Install(ctx, archiveMiddle)
	if err != nil {
		t.Fatal(err)
	}
	final, err := manager.Install(ctx, archiveFinal)
	if err != nil {
		t.Fatal(err)
	}

	// First activation makes middle active. The second activation replaces the
	// rollback target but retains middle as the previous source path.
	if _, err := manager.PrepareActivation(ctx, middle.PackageID, middle.PackageVersion, oldFrame); err != nil {
		t.Fatal(err)
	}
	if err := manager.CommitActivation(middle.FramePath); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.PrepareActivation(ctx, final.PackageID, final.PackageVersion, middle.FramePath); err != nil {
		t.Fatal(err)
	}
	if err := manager.CommitActivation(final.FramePath); err != nil {
		t.Fatal(err)
	}

	if err := manager.Uninstall(ctx, middle.PackageID, middle.PackageVersion, final.FramePath); err == nil ||
		!strings.Contains(err.Error(), "rollback state") {
		t.Fatalf("Uninstall rollback source error=%v, want rollback-reference guard", err)
	}

	restoredPath, err := manager.RollbackFramePath(ctx, final.FramePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.ClearRollback(); err != nil {
		t.Fatal(err)
	}
	if err := manager.Uninstall(ctx, middle.PackageID, middle.PackageVersion, restoredPath); err != nil {
		t.Fatalf("Uninstall after rollback was cleared: %v", err)
	}
	if _, err := os.Stat(final.FramePath); err != nil {
		t.Fatalf("uninstalling old package removed current package: %v", err)
	}
}

func TestUninstallHonorsCancelledContextAndRejectsSymlinkedPackageRoot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dir := t.TempDir()
	framesRoot := filepath.Join(dir, "frames")
	manager := NewManager(framesRoot)
	if err := manager.Uninstall(ctx, "sample-frame", "1.0.0", filepath.Join(dir, "active.json")); !errors.Is(err, context.Canceled) {
		t.Fatalf("Uninstall cancelled context error=%v, want context.Canceled", err)
	}

	outside := filepath.Join(dir, "outside")
	if err := os.MkdirAll(filepath.Join(outside, "sample-frame", "1.0.0"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(framesRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(framesRoot, "packages")); err != nil {
		t.Skipf("symlink unavailable on this platform/account: %v", err)
	}
	if err := manager.Uninstall(context.Background(), "sample-frame", "1.0.0", filepath.Join(dir, "active.json")); err == nil ||
		!strings.Contains(err.Error(), "regular directory") {
		t.Fatalf("Uninstall symlinked root error=%v, want symlink guard", err)
	}
}
func TestFindInstalledRejectsSymlinkedParentDirectory(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	frame := writeFixtureFrame(t, dir, "frame.json", 2)
	archive := createFixturePackage(t, dir, frame, "Parent-link", "1.0.0")
	outsideManager := NewManager(filepath.Join(dir, "outside-frames"))
	installed, err := outsideManager.Install(ctx, archive)
	if err != nil {
		t.Fatal(err)
	}

	managedRoot := filepath.Join(dir, "managed-frames")
	packagesRoot := filepath.Join(managedRoot, "packages")
	if err := os.MkdirAll(packagesRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(packagesRoot, installed.PackageID)
	if err := os.Symlink(filepath.Join(dir, "outside-frames", "packages", installed.PackageID), linkPath); err != nil {
		t.Skipf("symlink unavailable on this platform/account: %v", err)
	}
	manager := NewManager(managedRoot)
	if _, err := manager.findInstalled(ctx, installed.PackageID, installed.PackageVersion); err == nil ||
		!strings.Contains(err.Error(), "regular directory, not a symlink") {
		t.Fatalf("findInstalled error=%v, want symlink-parent guard", err)
	}
}

func TestRollbackStateReadersRejectSymlinkedRollbackDirectory(t *testing.T) {
	dir := t.TempDir()
	framesRoot := filepath.Join(dir, "frames")
	outside := filepath.Join(dir, "outside-rollback")
	if err := os.MkdirAll(framesRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(framesRoot, "rollback")); err != nil {
		t.Skipf("symlink unavailable on this platform/account: %v", err)
	}
	manager := NewManager(framesRoot)
	if _, err := manager.readPendingState(); err == nil || !strings.Contains(err.Error(), "rollback state directory must be a regular directory") {
		t.Fatalf("readPendingState error=%v, want rollback symlink guard", err)
	}
	if _, err := manager.loadRollbackState(); err == nil || !strings.Contains(err.Error(), "rollback state directory must be a regular directory") {
		t.Fatalf("loadRollbackState error=%v, want rollback symlink guard", err)
	}
}
func TestReadRegularFileRejectsSymlinkAndBoundsBytes(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	if err := os.WriteFile(source, []byte("safe bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := readRegularFile(source, 32)
	if err != nil || string(data) != "safe bytes" {
		t.Fatalf("readRegularFile data=%q err=%v", data, err)
	}
	if _, err := readRegularFile(source, 4); err == nil || !strings.Contains(err.Error(), "size") {
		t.Fatalf("oversized file error=%v, want size limit", err)
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(source, link); err != nil {
		t.Skipf("symlink unavailable on this platform/account: %v", err)
	}
	if _, err := readRegularFile(link, 32); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("symlink file error=%v, want symlink rejection", err)
	}
}
