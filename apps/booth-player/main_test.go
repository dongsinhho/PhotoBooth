package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
	"github.com/dongsinhho/PhotoBooth/internal/booth/session"
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

func TestRunUsesLocalDeliveryAndDisabledPaymentEndToEnd(t *testing.T) {
	root := t.TempDir()
	framePath := filepath.Join("..", "..", "examples", "frames", "photo-strip-v1.json")
	absoluteFrame, err := filepath.Abs(framePath)
	if err != nil {
		t.Fatal(err)
	}
	cfg := stationconfig.Defaults()
	cfg.Camera.Provider = "fake"
	cfg.Capture.Shots = 3
	cfg.Capture.Countdown = "0s"
	cfg.Capture.Interval = "0s"
	cfg.Frame.ConfigPath = absoluteFrame
	cfg.Storage.SessionsDir = filepath.Join(root, "sessions")
	cfg.Storage.OutputDir = filepath.Join(root, "photos")
	cfg.MediaDelivery.Mode = stationconfig.MediaDeliveryModeLocal
	cfg.Payment.Mode = stationconfig.PaymentModeDisabled
	configPath := filepath.Join(root, "store-config-v1.json")
	if err := stationconfig.Save(configPath, cfg); err != nil {
		t.Fatalf("save station config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	if err := run(context.Background(), []string{"--config", configPath}, &stdout, &stderr); err != nil {
		t.Fatalf("run local/offline session: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "synthetic images") ||
		!strings.Contains(stdout.String(), "Locally saved output:") {
		t.Fatalf("CLI output does not show the synthetic/local path: %s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr for successful local/free session: %s", stderr.String())
	}

	sessions, err := os.ReadDir(cfg.Storage.SessionsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || !sessions[0].IsDir() {
		t.Fatalf("created session workspaces=%d, want exactly one: %+v", len(sessions), sessions)
	}
	sessionDir := filepath.Join(cfg.Storage.SessionsDir, sessions[0].Name())
	manifest, err := session.ReadManifest(sessionDir)
	if err != nil {
		t.Fatalf("read durable session manifest: %v", err)
	}
	if manifest.Version != session.ManifestVersion ||
		manifest.MediaDeliveryMode != stationconfig.MediaDeliveryModeLocal ||
		manifest.PaymentMode != stationconfig.PaymentModeDisabled ||
		manifest.Phase != session.PhaseOutputFinalized {
		t.Fatalf("manifest did not persist the selected local/free policies and finalized state: %+v", manifest)
	}
	if len(manifest.FrameSnapshotFile) == 0 || manifest.OutputPath == "" {
		t.Fatalf("session manifest is missing its durable frame/output references: %+v", manifest)
	}
	outputSessionDir := filepath.Join(cfg.Storage.OutputDir, manifest.SessionID)
	if filepath.Clean(filepath.Dir(manifest.OutputPath)) != filepath.Clean(outputSessionDir) {
		t.Fatalf("output escaped its session-local directory: %q", manifest.OutputPath)
	}
	if err := session.ValidateJPEGFile(manifest.OutputPath); err != nil {
		t.Fatalf("local output is missing or invalid: %v", err)
	}
	captures, err := os.ReadDir(filepath.Join(sessionDir, "captures"))
	if err != nil {
		t.Fatal(err)
	}
	if len(captures) != 3 {
		t.Fatalf("committed captures=%d, want 3", len(captures))
	}
}

func TestResolveSettingsRejectsUnsupportedPoliciesBeforeCreatingSessionWorkspace(t *testing.T) {
	root := t.TempDir()
	framePath, err := filepath.Abs(filepath.Join("..", "..", "examples", "frames", "photo-strip-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(*stationconfig.StoreConfig)
		want string
	}{
		{
			name: "LAN delivery",
			edit: func(cfg *stationconfig.StoreConfig) { cfg.MediaDelivery.Mode = "lan" },
			want: "unsupported media_delivery.mode",
		},
		{
			name: "cloud delivery",
			edit: func(cfg *stationconfig.StoreConfig) { cfg.MediaDelivery.Mode = "cloud" },
			want: "unsupported media_delivery.mode",
		},
		{
			name: "QR payment",
			edit: func(cfg *stationconfig.StoreConfig) { cfg.Payment.Mode = "qr" },
			want: "unsupported payment.mode",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := stationconfig.Defaults()
			cfg.Camera.Provider = "fake"
			cfg.Frame.ConfigPath = framePath
			cfg.Storage.SessionsDir = filepath.Join(root, tc.name, "sessions")
			cfg.Storage.OutputDir = filepath.Join(root, tc.name, "photos")
			tc.edit(&cfg)
			configPath := filepath.Join(root, tc.name, "store-config-v1.json")
			if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := stationconfig.Save(configPath, cfg); err == nil {
				t.Fatal("unsupported policy was persisted")
			}
			// Simulate an operator-edited/externally supplied invalid profile. Save
			// correctly refuses it, while Load/resolveSettings must reject it too.
			data, err := json.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(configPath, data, 0o600); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			runErr := run(context.Background(), []string{"--config", configPath}, &stdout, &stderr)
			if runErr == nil || !strings.Contains(runErr.Error(), tc.want) {
				t.Fatalf("CLI run error=%v, want %q", runErr, tc.want)
			}
			if stdout.Len() != 0 {
				t.Fatalf("unsupported policy reached the run/capture path: %s", stdout.String())
			}
			if _, err := os.Stat(cfg.Storage.SessionsDir); !os.IsNotExist(err) {
				t.Fatalf("unsupported policy created the sessions root: %v", err)
			}
		})
	}
}
