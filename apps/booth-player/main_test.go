package main

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestResolveFrameConfigExplicitPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "frame.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := resolveFrameConfig(path)
	if err != nil {
		t.Fatalf("resolveFrameConfig: %v", err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != abs {
		t.Fatalf("got %q, want %q", got, abs)
	}
}

func TestResolveFrameConfigRejectsMissingExplicitPath(t *testing.T) {
	_, err := resolveFrameConfig(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("expected missing frame path to fail")
	}
}

func TestResolveSettingsLoadsStoreConfigAndResolvesPaths(t *testing.T) {
	dir := t.TempDir()
	framePath := filepath.Join(dir, "frame.json")
	if err := os.WriteFile(framePath, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "store.json")
	contents := `{
		"schema_version": 1,
		"camera": {"provider": "fake", "width": 640, "height": 480, "fps": 25},
		"capture": {"shots": 5, "countdown": "0s", "interval": "2s"},
		"frame": {"config_path": "frame.json"},
		"storage": {"sessions_dir": "sessions", "output_dir": "output"}
	}`
	if err := os.WriteFile(configPath, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := resolveSettings([]string{"--config", configPath}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Camera.Width != 640 || cfg.Camera.Height != 480 || cfg.Camera.FPS != 25 {
		t.Fatalf("camera config=%+v", cfg.Camera)
	}
	if cfg.Capture.Shots != 5 || cfg.Capture.Countdown != "0s" || cfg.Capture.Interval != "2s" {
		t.Fatalf("capture config=%+v", cfg.Capture)
	}
	if cfg.Frame.ConfigPath != framePath {
		t.Fatalf("frame path=%q, want %q", cfg.Frame.ConfigPath, framePath)
	}
	if cfg.Storage.SessionsDir != filepath.Join(dir, "sessions") || cfg.Storage.OutputDir != filepath.Join(dir, "output") {
		t.Fatalf("storage config=%+v", cfg.Storage)
	}
	if cfg.MediaDelivery.Mode != "local" || cfg.Payment.Mode != "disabled" {
		t.Fatalf("omitted policy defaults were not preserved: media=%+v payment=%+v", cfg.MediaDelivery, cfg.Payment)
	}
	if countdown, _ := cfg.CountdownDuration(); countdown != 0 {
		t.Fatalf("countdown=%s, want 0", countdown)
	}
	if interval, _ := cfg.IntervalDuration(); interval != 2*time.Second {
		t.Fatalf("interval=%s, want 2s", interval)
	}
}

func TestResolveSettingsExplicitCLIFlagsOverrideConfigOnlyWhenProvided(t *testing.T) {
	dir := t.TempDir()
	framePath := filepath.Join(dir, "frame.json")
	if err := os.WriteFile(framePath, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "store.json")
	contents := `{
		"schema_version": 1,
		"camera": {"provider": "fake", "width": 640, "height": 480, "fps": 25},
		"capture": {"shots": 5, "countdown": "0s", "interval": "2s"},
		"frame": {"config_path": "frame.json"},
		"storage": {"sessions_dir": "sessions", "output_dir": "output"}
	}`
	if err := os.WriteFile(configPath, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	overrideOutput := filepath.Join(dir, "cli-output")

	cfg, err := resolveSettings([]string{
		"--config", configPath,
		"--shots=2",
		"--countdown=7s",
		"--output-dir", overrideOutput,
	}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Capture.Shots != 2 || cfg.Capture.Countdown != "7s" {
		t.Fatalf("explicit capture flags did not override config: %+v", cfg.Capture)
	}
	if cfg.Camera.Width != 640 || cfg.Camera.Height != 480 || cfg.Camera.FPS != 25 {
		t.Fatalf("non-overridden config settings were lost: %+v", cfg.Camera)
	}
	if cfg.Storage.OutputDir != overrideOutput {
		t.Fatalf("output dir=%q, want CLI override %q", cfg.Storage.OutputDir, overrideOutput)
	}
	if cfg.Storage.SessionsDir != filepath.Join(dir, "sessions") {
		t.Fatalf("omitted sessions-dir flag should preserve config: %q", cfg.Storage.SessionsDir)
	}
}

func TestResolveSettingsKeepsLegacyCLIWithoutConfig(t *testing.T) {
	framePath := filepath.Join(t.TempDir(), "frame.json")
	if err := os.WriteFile(framePath, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := resolveSettings([]string{"--frame", framePath, "--camera=fake", "--shots=3", "--countdown=0s", "--interval=0s"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Camera.Provider != "fake" || cfg.Capture.Shots != 3 || cfg.Capture.Countdown != "0s" || cfg.Capture.Interval != "0s" {
		t.Fatalf("legacy CLI settings changed: %+v %+v", cfg.Camera, cfg.Capture)
	}
	if cfg.Frame.ConfigPath != framePath {
		t.Fatalf("frame path=%q, want %q", cfg.Frame.ConfigPath, framePath)
	}
}

func TestResolveSettingsRejectsUnsupportedPolicyAndMissingConfigPath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "store.json")
	contents := `{"schema_version":1,"media_delivery":{"mode":"cloud"}}`
	if err := os.WriteFile(configPath, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveSettings([]string{"--config", configPath}, io.Discard); err == nil {
		t.Fatal("expected unsupported media-delivery mode to fail")
	}
	if _, err := resolveSettings([]string{"--config="}, io.Discard); err == nil {
		t.Fatal("expected empty --config to fail")
	}
}

func TestExtractSkipCameraWarmup(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantArgs  []string
		wantSkip  bool
		wantError bool
	}{
		{name: "not supplied", args: []string{"--camera=windowsmf", "--shots=3"}, wantArgs: []string{"--camera=windowsmf", "--shots=3"}},
		{name: "standalone enables skip", args: []string{"--camera=windowsmf", "--skip-camera-warmup", "--shots=3"}, wantArgs: []string{"--camera=windowsmf", "--shots=3"}, wantSkip: true},
		{name: "explicit true", args: []string{"--skip-camera-warmup=true", "--camera=windowsmf"}, wantArgs: []string{"--camera=windowsmf"}, wantSkip: true},
		{name: "explicit false", args: []string{"--skip-camera-warmup=false", "--camera=windowsmf"}, wantArgs: []string{"--camera=windowsmf"}},
		{name: "duplicate rejected", args: []string{"--skip-camera-warmup", "--skip-camera-warmup=true"}, wantError: true},
		{name: "invalid boolean rejected", args: []string{"--skip-camera-warmup=sometimes"}, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotArgs, gotSkip, err := extractSkipCameraWarmup(tt.args)
			if tt.wantError {
				if err == nil {
					t.Fatal("expected parsing error")
				}
				return
			}
			if err != nil {
				t.Fatalf("extractSkipCameraWarmup: %v", err)
			}
			if gotSkip != tt.wantSkip {
				t.Fatalf("skip=%v, want %v", gotSkip, tt.wantSkip)
			}
			if !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Fatalf("args=%v, want %v", gotArgs, tt.wantArgs)
			}
		})
	}
}
