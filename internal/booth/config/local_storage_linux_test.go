//go:build linux

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLinuxStoragePathFollowsSymlinkAndMissingChildren(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "real")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("directory symlinks are unavailable: %v", err)
	}

	got, err := resolveLinuxStoragePath(filepath.Join(link, "not-created", "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(target, "not-created", "sessions")
	if got != want {
		t.Fatalf("resolved path=%q, want %q", got, want)
	}
}

func TestResolveLinuxStoragePathRejectsBrokenSymlink(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(root, "broken")
	if err := os.Symlink(filepath.Join(root, "missing-target"), link); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if _, err := resolveLinuxStoragePath(filepath.Join(link, "sessions")); err == nil {
		t.Fatal("broken symlink must fail closed")
	}
}

func TestResolveLinuxStoragePathRejectsExistingFileComponent(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(file, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveLinuxStoragePath(filepath.Join(file, "sessions")); err == nil {
		t.Fatal("file path component must not be accepted as a storage directory")
	}
}
