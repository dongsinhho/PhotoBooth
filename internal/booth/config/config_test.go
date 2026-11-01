package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeConfig(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDefaultsAreLocalOfflineAndValid(t *testing.T) {
	cfg := Defaults()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Defaults().Validate(): %v", err)
	}
	if cfg.SchemaVersion != 1 || cfg.Camera.Provider != "fake" ||
		cfg.MediaDelivery.Mode != "local" || cfg.Payment.Mode != "disabled" {
		t.Fatalf("unexpected safe defaults: %+v", cfg)
	}
	countdown, err := cfg.CountdownDuration()
	if err != nil || countdown != 3*time.Second {
		t.Fatalf("countdown=%s err=%v", countdown, err)
	}
	interval, err := cfg.IntervalDuration()
	if err != nil || interval != time.Second {
		t.Fatalf("interval=%s err=%v", interval, err)
	}
}

func TestLoadPartialConfigInheritsDefaultsAndResolvesRelativePaths(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "config.json", `{
		"schema_version": 1,
		"capture": {"shots": 5, "countdown": "0s"}
	}`)

	cfg, err := Load(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Capture.Shots != 5 || cfg.Capture.Interval != "1s" {
		t.Fatalf("partial capture config did not inherit defaults: %+v", cfg.Capture)
	}
	if cfg.Camera.Provider != "fake" || cfg.Camera.Width != 1280 || cfg.Camera.Height != 720 {
		t.Fatalf("camera defaults were lost: %+v", cfg.Camera)
	}
	if cfg.Capture.Countdown != "0s" {
		t.Fatalf("countdown=%q, want 0s", cfg.Capture.Countdown)
	}
	if !filepath.IsAbs(cfg.Frame.ConfigPath) || !filepath.IsAbs(cfg.Storage.OutputDir) || !filepath.IsAbs(cfg.Storage.SessionsDir) {
		t.Fatalf("paths should be absolute after loading: %+v", cfg)
	}
	duration, err := cfg.CountdownDuration()
	if err != nil || duration != 0 {
		t.Fatalf("CountdownDuration()=%s, %v", duration, err)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "config.json", `{"schema_version":1,"capture":{"shots":3,"shotz":2}}`)
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("Load() error=%v, want unknown-field error", err)
	}
}

func TestLoadRejectsUnsupportedSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "config.json", `{"schema_version":999}`)
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "unsupported store config schema_version") {
		t.Fatalf("Load() error=%v, want unsupported version", err)
	}
}

func TestLoadRequiresSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "config.json", `{"capture":{"shots":3}}`)
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "must include schema_version") {
		t.Fatalf("Load() error=%v, want missing version error", err)
	}
}

func TestLoadRejectsTrailingJSONDocument(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "config.json", `{"schema_version":1} {"schema_version":1}`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() should reject a second JSON document")
	}
}

func TestLoadRejectsOversizedConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"padding":"`+strings.Repeat("x", maxConfigBytes)+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("Load() error=%v, want size-limit error", err)
	}
}

func TestValidateRejectsInvalidAndUnimplementedSettings(t *testing.T) {
	tests := []struct {
		name string
		edit func(*StoreConfig)
		want string
	}{
		{"too many shots", func(c *StoreConfig) { c.Capture.Shots = 13 }, "capture.shots"},
		{"invalid camera size", func(c *StoreConfig) { c.Camera.Width = 5000 }, "camera.width"},
		{"invalid fps", func(c *StoreConfig) { c.Camera.FPS = 0 }, "camera.fps"},
		{"negative countdown", func(c *StoreConfig) { c.Capture.Countdown = "-1s" }, "capture.countdown"},
		{"invalid interval", func(c *StoreConfig) { c.Capture.Interval = "soon" }, "capture.interval"},
		{"media mode not implemented", func(c *StoreConfig) { c.MediaDelivery.Mode = "cloud" }, "only local is implemented"},
		{"payment mode not implemented", func(c *StoreConfig) { c.Payment.Mode = "qr" }, "only disabled/free mode is implemented"},
		{"unknown provider", func(c *StoreConfig) { c.Camera.Provider = "dslr" }, "unsupported camera.provider"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Defaults()
			tt.edit(&cfg)
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error=%v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestLoadRejectsDirectoryAndMissingPath(t *testing.T) {
	if _, err := Load(""); err == nil {
		t.Fatal("Load empty path should fail")
	}
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("Load directory should fail")
	}
}
func TestLoadDefaultFramePathIsRelativeToConfigDirectory(t *testing.T) {
	dir := t.TempDir()
	framesDir := filepath.Join(dir, "frames")
	if err := os.MkdirAll(framesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	frame := filepath.Join(framesDir, "photo-strip-v1.json")
	if err := os.WriteFile(frame, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := writeConfig(t, dir, "store.json", `{"schema_version":1}`)
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Frame.ConfigPath != frame {
		t.Fatalf("default frame path=%q, want sibling frame %q", cfg.Frame.ConfigPath, frame)
	}
}
func TestLoadRejectsNullSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "store.json", `{"schema_version":null}`)
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "schema_version must be an integer") {
		t.Fatalf("Load() error=%v, want non-null integer schema version", err)
	}
}
func TestSaveRoundTripsAndStoresOwnedPathsRelative(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "frames"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := Defaults()
	cfg.Frame.ConfigPath = filepath.Join(dir, "frames", "photo-strip-v1.json")
	cfg.Storage.SessionsDir = filepath.Join(dir, "sessions")
	cfg.Storage.OutputDir = filepath.Join(dir, "photos")
	if err := Save(filepath.Join(dir, "store.json"), cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "store.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), dir) {
		t.Fatalf("paths inside the config folder should be stored relatively: %s", data)
	}
	got, err := Load(filepath.Join(dir, "store.json"))
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if got.Frame.ConfigPath != cfg.Frame.ConfigPath || got.Storage.SessionsDir != cfg.Storage.SessionsDir || got.Storage.OutputDir != cfg.Storage.OutputDir {
		t.Fatalf("round-trip paths got=%+v want=%+v", got, cfg)
	}
}

func TestSaveRejectsInvalidConfigWithoutReplacingExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "store.json", `{"schema_version":1}`)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Defaults()
	cfg.Capture.Shots = 0
	if err := Save(path, cfg); err == nil {
		t.Fatal("Save should reject invalid config")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("invalid save changed existing config: before=%s after=%s", before, after)
	}
}
func TestStorageRetentionDefaultsDisabledAndRejectsUnsafeRanges(t *testing.T) {
	cfg := Defaults()
	if cfg.Storage.SessionRetentionDays != 0 || cfg.Storage.OutputRetentionDays != 0 {
		t.Fatalf("retention must default to disabled; got session=%d output=%d",
			cfg.Storage.SessionRetentionDays, cfg.Storage.OutputRetentionDays)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config should validate: %v", err)
	}
	tests := []struct {
		name string
		edit func(*StoreConfig)
		want string
	}{
		{"negative session retention", func(c *StoreConfig) { c.Storage.SessionRetentionDays = -1 }, "storage.session_retention_days"},
		{"too large session retention", func(c *StoreConfig) { c.Storage.SessionRetentionDays = 3651 }, "storage.session_retention_days"},
		{"negative output retention", func(c *StoreConfig) { c.Storage.OutputRetentionDays = -1 }, "storage.output_retention_days"},
		{"too large output retention", func(c *StoreConfig) { c.Storage.OutputRetentionDays = 3651 }, "storage.output_retention_days"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Defaults()
			tt.edit(&cfg)
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate error=%v, want %s", err, tt.want)
			}
		})
	}
}

func TestStorageRetentionRoundTripsWithoutChangingDefaultPolicy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	cfg := Defaults()
	cfg.Storage.SessionsDir = filepath.Join(dir, "sessions")
	cfg.Storage.OutputDir = filepath.Join(dir, "output")
	cfg.Storage.SessionRetentionDays = 7
	cfg.Storage.OutputRetentionDays = 30
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Storage.SessionRetentionDays != 7 || got.Storage.OutputRetentionDays != 30 {
		t.Fatalf("retention did not round trip: %+v", got.Storage)
	}

	// Existing/partial schema-v1 profiles that omit the new fields inherit the
	// safe default: cleanup remains disabled until an operator opts in.
	partial := writeConfig(t, dir, "partial.json", `{"schema_version":1}`)
	got, err = Load(partial)
	if err != nil {
		t.Fatal(err)
	}
	if got.Storage.SessionRetentionDays != 0 || got.Storage.OutputRetentionDays != 0 {
		t.Fatalf("omitted retention policy must stay disabled: %+v", got.Storage)
	}
}
