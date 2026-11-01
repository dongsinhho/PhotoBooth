package output

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type diskFullTempFile struct {
	file  *os.File
	mode  string
	cause error
}

func (f *diskFullTempFile) Name() string { return f.file.Name() }
func (f *diskFullTempFile) Close() error {
	err := f.file.Close()
	if f.mode == "close" {
		return errors.Join(err, f.cause)
	}
	return err
}

func (f *diskFullTempFile) Write(data []byte) (int, error) {
	if f.mode != "write" {
		return f.file.Write(data)
	}
	partial := len(data) / 2
	if partial == 0 && len(data) > 0 {
		partial = 1
	}
	written, err := f.file.Write(data[:partial])
	if err != nil {
		return written, err
	}
	return written, f.cause
}

func (f *diskFullTempFile) Sync() error {
	if f.mode == "sync" {
		return f.cause
	}
	return f.file.Sync()
}

type diskFullFileOps struct {
	osLocalFileOps
	mode       string
	cause      error
	tempPath   string
	mkdirCalls int
}

func (f *diskFullFileOps) MkdirAll(path string, perm os.FileMode) error {
	f.mkdirCalls++
	if f.mode == "mkdir" {
		return f.cause
	}
	return f.osLocalFileOps.MkdirAll(path, perm)
}

func (f *diskFullFileOps) Rename(oldPath, newPath string) error {
	if f.mode == "rename" {
		return f.cause
	}
	return f.osLocalFileOps.Rename(oldPath, newPath)
}

func (f *diskFullFileOps) CreateTemp(dir, pattern string) (localTempFile, error) {
	if f.mode == "create-temp" {
		return nil, f.cause
	}
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	f.tempPath = file.Name()
	return &diskFullTempFile{file: file, mode: f.mode, cause: f.cause}, nil
}

func TestLocalSaveHandlesDiskFullWithoutPublishingPartialPhoto(t *testing.T) {
	for _, mode := range []string{"mkdir", "create-temp", "write", "sync", "close", "rename"} {
		t.Run(mode, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "photos")
			sessionID := testSessionID
			source := filepath.Join(t.TempDir(), "rendered.jpg")
			sourceBytes := []byte("synthetic-rendered-photo-data-that-is-long-enough-for-a-partial-write")
			if err := os.WriteFile(source, sourceBytes, 0o600); err != nil {
				t.Fatal(err)
			}

			cause := diskFullCauseForTest()
			ops := &diskFullFileOps{mode: mode, cause: cause}
			provider := &LocalProvider{RootDir: root, ops: ops}
			finalPath, err := provider.Save(context.Background(), sessionID, source)
			if err == nil {
				t.Fatal("Save unexpectedly succeeded when the filesystem reported disk full")
			}
			if finalPath != "" {
				t.Fatalf("Save returned a published path after failure: %q", finalPath)
			}
			if !errors.Is(err, ErrInsufficientStorage) {
				t.Fatalf("error %v does not wrap ErrInsufficientStorage", err)
			}
			if !errors.Is(err, cause) {
				t.Fatalf("error %v does not preserve the underlying disk-full cause", err)
			}
			if !strings.Contains(err.Error(), "free disk space") || !strings.Contains(err.Error(), "preserved") {
				t.Fatalf("error is missing operator recovery guidance: %v", err)
			}

			if _, statErr := os.Stat(filepath.Join(root, sessionID, "photo.jpg")); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("partial final photo was published or stat failed unexpectedly: %v", statErr)
			}
			if ops.tempPath != "" {
				if _, statErr := os.Stat(ops.tempPath); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("partial temp file was not removed: %v", statErr)
				}
			}
			if got, readErr := os.ReadFile(source); readErr != nil {
				t.Fatalf("source rendered image was lost: %v", readErr)
			} else if string(got) != string(sourceBytes) {
				t.Fatal("source rendered image was modified")
			}
		})
	}
}

func TestStorageOperationErrorDistinguishesDiskFullFromGenericFailure(t *testing.T) {
	diskFull := storageOperationError("write temporary output", diskFullCauseForTest())
	if !errors.Is(diskFull, ErrInsufficientStorage) {
		t.Fatalf("disk-full error %v must wrap ErrInsufficientStorage", diskFull)
	}
	generic := storageOperationError("write temporary output", errors.New("access denied"))
	if errors.Is(generic, ErrInsufficientStorage) {
		t.Fatalf("generic error %v must not be classified as insufficient storage", generic)
	}
}

func TestLocalSaveSucceedsAfterDiskFullConditionIsCleared(t *testing.T) {
	root := filepath.Join(t.TempDir(), "photos")
	source := filepath.Join(t.TempDir(), "rendered.jpg")
	sourceBytes := []byte("synthetic-photo-after-operator-frees-disk-space")
	if err := os.WriteFile(source, sourceBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	provider := NewLocal(root)
	path, err := provider.Save(context.Background(), testSessionID, source)
	if err != nil {
		t.Fatalf("retry after disk-space recovery: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved photo: %v", err)
	}
	if string(got) != string(sourceBytes) {
		t.Fatal("saved output does not match rendered source")
	}
}
func TestLocalSavePreservesPreviouslyPublishedPhotoWhenReplacementFails(t *testing.T) {
	root := filepath.Join(t.TempDir(), "photos")
	sessionDir := filepath.Join(root, testSessionID)
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	finalPath := filepath.Join(sessionDir, "photo.jpg")
	previous := []byte("previous-complete-photo")
	if err := os.WriteFile(finalPath, previous, 0o600); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "rendered.jpg")
	if err := os.WriteFile(source, []byte("new-rendered-photo"), 0o600); err != nil {
		t.Fatal(err)
	}

	ops := &diskFullFileOps{mode: "rename", cause: diskFullCauseForTest()}
	provider := &LocalProvider{RootDir: root, ops: ops}
	if _, err := provider.Save(context.Background(), testSessionID, source); !errors.Is(err, ErrInsufficientStorage) {
		t.Fatalf("Save error=%v, want insufficient storage", err)
	}
	got, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("previously published photo disappeared: %v", err)
	}
	if string(got) != string(previous) {
		t.Fatalf("previous photo was modified: got %q want %q", got, previous)
	}
	if ops.tempPath != "" {
		if _, err := os.Stat(ops.tempPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("failed replacement temp file remains: %v", err)
		}
	}
}
func TestTargetResultKeepsTypedCauseOutOfJSON(t *testing.T) {
	cause := errors.New("private typed cause")
	result := TargetResult{
		Name:   TargetLocalStorage,
		Status: TargetFailed,
		Error:  "insufficient storage space; free disk space and retry",
		Cause:  cause,
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "Cause") || strings.Contains(text, cause.Error()) {
		t.Fatalf("typed cause must not be serialized to UI JSON: %s", text)
	}
	if !strings.Contains(text, "insufficient storage space") {
		t.Fatalf("human-readable diagnostic was lost from JSON: %s", text)
	}
}
