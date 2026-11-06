package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateDistinctStorageRootsRejectsIdenticalAndNestedRoots(t *testing.T) {
	base := t.TempDir()
	sessions := filepath.Join(base, "sessions")
	nested := filepath.Join(sessions, "photos")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		session string
		output  string
	}{
		{"identical", sessions, sessions},
		{"output inside sessions", sessions, nested},
		{"sessions inside output", nested, sessions},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDistinctStorageRoots(tt.session, tt.output)
			if err == nil || !strings.Contains(err.Error(), "non-overlapping roots") {
				t.Fatalf("ValidateDistinctStorageRoots() error=%v, want overlap rejection", err)
			}
		})
	}
}

func TestValidateDistinctStorageRootsAcceptsSiblingRoots(t *testing.T) {
	base := t.TempDir()
	sessions := filepath.Join(base, "sessions")
	output := filepath.Join(base, "photos")
	if err := ValidateDistinctStorageRoots(sessions, output); err != nil {
		t.Fatalf("sibling roots should be valid: %v", err)
	}
}

func TestValidateDistinctStorageRootsDetectsSymlinkAlias(t *testing.T) {
	base := t.TempDir()
	sessions := filepath.Join(base, "sessions")
	alias := filepath.Join(base, "sessions-alias")
	if err := os.Mkdir(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sessions, alias); err != nil {
		t.Skipf("directory symlink unavailable: %v", err)
	}
	if err := ValidateDistinctStorageRoots(sessions, filepath.Join(alias, "photos")); err == nil {
		t.Fatal("storage roots that overlap through a symlink must be rejected")
	}
}

func TestStoreConfigValidateRejectsOverlappingStorageRoots(t *testing.T) {
	base := t.TempDir()
	cfg := Defaults()
	cfg.Storage.SessionsDir = filepath.Join(base, "station-data")
	cfg.Storage.OutputDir = filepath.Join(base, "station-data", "photos")
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "non-overlapping roots") {
		t.Fatalf("Validate error=%v, want overlapping storage roots rejected", err)
	}
}

func TestSaveRejectsOverlappingStorageRootsWithoutReplacingExistingConfig(t *testing.T) {
	base := t.TempDir()
	configPath := filepath.Join(base, "store-config-v1.json")
	original := []byte("{\"schema_version\":1}")
	if err := os.WriteFile(configPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := Defaults()
	cfg.Storage.SessionsDir = "station-data"
	cfg.Storage.OutputDir = filepath.Join("station-data", "photos")
	if err := Save(configPath, cfg); err == nil || !strings.Contains(err.Error(), "non-overlapping roots") {
		t.Fatalf("Save error=%v, want overlapping roots rejected", err)
	}
	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("rejected Save changed existing config: %s", got)
	}
}

func TestValidateStorageChildRejectsSymlinkThatRedirectsOutsideRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "sessions")
	outside := filepath.Join(base, "redirected")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "session-one")
	target := filepath.Join(outside, "session-one")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, child); err != nil {
		t.Skipf("directory symlink unavailable: %v", err)
	}
	if err := ValidateStorageChild(root, child); err == nil {
		t.Fatal("redirected session child must be rejected")
	}
}
