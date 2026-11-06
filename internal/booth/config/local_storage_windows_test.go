//go:build windows

package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRejectsUNCStoragePaths(t *testing.T) {
	tests := []struct {
		name string
		edit func(*StoreConfig)
		want string
	}{
		{"sessions UNC", func(c *StoreConfig) { c.Storage.SessionsDir = "\\\\server\\share\\sessions" }, "storage.sessions_dir"},
		{"output UNC", func(c *StoreConfig) { c.Storage.OutputDir = "\\\\server\\share\\photos" }, "storage.output_dir"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Defaults()
			tt.edit(&cfg)
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) || !strings.Contains(err.Error(), "network storage") {
				t.Fatalf("Validate error=%v, want network path rejection for %s", err, tt.want)
			}
		})
	}
}

func TestLoadRejectsUNCStoragePathAfterResolvingRelativePaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	data := []byte("{\"schema_version\":1,\"storage\":{\"sessions_dir\":\"\\\\\\\\server\\\\share\\\\sessions\"}}")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "storage.sessions_dir") {
		t.Fatalf("Load error=%v, want UNC path rejection", err)
	}
}

func TestSaveRejectsUNCStoragePathWithoutReplacingExistingConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	original := []byte("{\"schema_version\":1}")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := Defaults()
	cfg.Storage.OutputDir = "\\\\server\\share\\photos"
	if err := Save(path, cfg); err == nil || !strings.Contains(err.Error(), "storage.output_dir") {
		t.Fatalf("Save error=%v, want UNC path rejection", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("rejected Save modified existing config: %s", got)
	}
}

func TestNormalizeWindowsFinalPathDetectsUncTarget(t *testing.T) {
	got := normalizeWindowsFinalPath("\\\\?\\UNC\\nas.example\\photos\\sessions")
	want := "\\\\nas.example\\photos\\sessions"
	if got != want {
		t.Fatalf("normalizeWindowsFinalPath()=%q, want %q", got, want)
	}
	got = normalizeWindowsFinalPath("\\\\?\\C:\\PhotoBooth\\photos")
	want = "C:\\PhotoBooth\\photos"
	if got != want {
		t.Fatalf("normalizeWindowsFinalPath()=%q, want %q", got, want)
	}
}

func TestResolveWindowsStorageTargetFollowsDirectoryJunction(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "real-target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	junction := filepath.Join(root, "junction")
	command := exec.Command("cmd", "/c", "mklink", "/J", junction, target)
	if output, err := command.CombinedOutput(); err != nil {
		t.Skipf("directory junction creation is unavailable in this environment: %v (%s)", err, output)
	}
	t.Cleanup(func() { _ = os.Remove(junction) })

	requested := filepath.Join(junction, "not-yet-created", "sessions")
	got, err := resolveWindowsStorageTarget(requested)
	if err != nil {
		t.Fatalf("resolveWindowsStorageTarget(%q): %v", requested, err)
	}
	want := filepath.Join(target, "not-yet-created", "sessions")
	if !strings.EqualFold(got, want) {
		t.Fatalf("resolved path=%q, want junction target %q", got, want)
	}
}

func TestNormalizeWindowsFinalPathPreservesVolumeGUIDPath(t *testing.T) {
	volumePath := "\\\\?\\Volume{01234567-89ab-cdef-0123-456789abcdef}\\PhotoBooth\\photos"
	got := normalizeWindowsFinalPath(volumePath)
	if got != volumePath {
		t.Fatalf("volume-GUID path was changed: got=%q want=%q", got, volumePath)
	}
	if isWindowsUNCPath(got) {
		t.Fatalf("local volume-GUID path %q was misclassified as UNC", got)
	}
	if isExtendedDrivePath(got) {
		t.Fatalf("volume-GUID path %q was misclassified as an extended drive-letter path", got)
	}
	if !filepath.IsAbs(got) || filepath.VolumeName(got) == "" {
		t.Fatalf("volume-GUID path %q lost its absolute volume identity", got)
	}
}
