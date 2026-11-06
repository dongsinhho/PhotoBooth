//go:build windows

package config

import (
	"os"
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
