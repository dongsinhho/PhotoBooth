package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
)

const retryTestSessionID = "0123456789abcdef0123456789abcdef"

func createRetryFixture(t *testing.T) (string, string, string, string) {
	t.Helper()
	root := t.TempDir()
	sessions := filepath.Join(root, "sessions")
	outputs := filepath.Join(root, "output")
	sessionID := retryTestSessionID
	captureDir := filepath.Join(sessions, sessionID, "captures")
	if err := os.MkdirAll(captureDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for shot := 1; shot <= 3; shot++ {
		path := filepath.Join(captureDir, fmt.Sprintf("capture-%02d.jpg", shot))
		if err := os.WriteFile(path, []byte(fmt.Sprintf("synthetic-capture-%d", shot)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rendered := filepath.Join(sessions, sessionID, "rendered.jpg")
	renderedBytes := []byte("synthetic-rendered-photo-for-output-retry")
	if err := os.WriteFile(rendered, renderedBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := stationconfig.Defaults()
	cfg.Storage.SessionsDir = sessions
	cfg.Storage.OutputDir = outputs
	configPath := filepath.Join(root, "store-config-v1.json")
	if err := stationconfig.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	return configPath, sessionID, rendered, outputs
}

func TestRunRetriesOnlyPreservedRenderedOutput(t *testing.T) {
	configPath, sessionID, rendered, outputs := createRetryFixture(t)
	var stdout, stderr bytes.Buffer
	err := run([]string{"--config", configPath, "--session-id", sessionID}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v stderr=%s", err, stderr.String())
	}
	finalPath := filepath.Join(outputs, sessionID, "photo.jpg")
	got, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("final local output: %v", err)
	}
	sourceBytes, err := os.ReadFile(rendered)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(sourceBytes) {
		t.Fatal("retry output does not match the preserved rendered image")
	}
	if !strings.Contains(stdout.String(), "OUTPUT_RETRY_SUCCEEDED") ||
		!strings.Contains(stdout.String(), "SESSION_ID="+sessionID) {
		t.Fatalf("success output missing session information: %s", stdout.String())
	}
	for shot := 1; shot <= 3; shot++ {
		path := filepath.Join(filepath.Dir(rendered), "captures", fmt.Sprintf("capture-%02d.jpg", shot))
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("retry removed capture %d: %v", shot, err)
		}
	}
}

func TestRunRefusesToOverwriteExistingPhoto(t *testing.T) {
	configPath, sessionID, _, outputs := createRetryFixture(t)
	outDir := filepath.Join(outputs, sessionID)
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		t.Fatal(err)
	}
	finalPath := filepath.Join(outDir, "photo.jpg")
	previous := []byte("previously-saved-photo-must-remain-unchanged")
	if err := os.WriteFile(finalPath, previous, 0o600); err != nil {
		t.Fatal(err)
	}
	err := run([]string{"--config", configPath, "--session-id", sessionID}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("run error=%v, want existing-output refusal", err)
	}
	got, readErr := os.ReadFile(finalPath)
	if readErr != nil || string(got) != string(previous) {
		t.Fatalf("existing output changed: got=%q err=%v", got, readErr)
	}
}

func TestRunRejectsIncompleteSessionAndInvalidSessionID(t *testing.T) {
	configPath, sessionID, rendered, _ := createRetryFixture(t)
	missingCapture := filepath.Join(filepath.Dir(rendered), "captures", "capture-03.jpg")
	if err := os.Remove(missingCapture); err != nil {
		t.Fatal(err)
	}
	err := run([]string{"--config", configPath, "--session-id", sessionID}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "capture 3") {
		t.Fatalf("incomplete session error=%v, want capture-3 validation", err)
	}
	err = run([]string{"--config", configPath, "--session-id", "../other"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "32-character lowercase hexadecimal") {
		t.Fatalf("invalid session ID error=%v", err)
	}
}

func TestRunRejectsOverlappingStorageRootsAndSymlinkedSession(t *testing.T) {
	configPath, sessionID, rendered, _ := createRetryFixture(t)
	cfg, err := stationconfig.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Storage.OutputDir = cfg.Storage.SessionsDir
	if err := stationconfig.Save(configPath, cfg); err == nil || !strings.Contains(err.Error(), "non-overlapping roots") {
		t.Fatalf("overlapping roots save error=%v, want config rejection", err)
	}

	// Replace the session folder with a symlink where supported. The failed
	// config save above must leave the original, separated roots untouched.
	sessionDir := filepath.Dir(rendered)
	if err := os.Rename(sessionDir, sessionDir+".saved"); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, sessionDir); err != nil {
		if restoreErr := os.Rename(sessionDir+".saved", sessionDir); restoreErr != nil {
			t.Fatal(restoreErr)
		}
		t.Skipf("symlink unavailable: %v", err)
	}
	defer func() {
		_ = os.Remove(sessionDir)
		_ = os.Rename(sessionDir+".saved", sessionDir)
	}()
	err = run([]string{"--config", configPath, "--session-id", sessionID}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "session identity") {
		t.Fatalf("symlinked session error=%v", err)
	}
}

func TestConfigFixtureRemainsValidJSON(t *testing.T) {
	configPath, _, _, _ := createRetryFixture(t)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("saved station config is not JSON: %v", err)
	}
}
