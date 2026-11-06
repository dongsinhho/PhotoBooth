package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	CurrentSchemaVersion = 1
	maxConfigBytes       = 64 << 10
	maxDimension         = 4096
	maxCaptureShots      = 12
	maxDuration          = 5 * time.Minute
)

// StoreConfig is the versioned, local configuration for one PhotoBooth station.
// Its initial policy intentionally supports local delivery and disabled payment
// only; unsupported network/payment modes fail validation instead of silently
// behaving as local/free mode.
type StoreConfig struct {
	SchemaVersion int                   `json:"schema_version"`
	Camera        CameraSettings        `json:"camera"`
	Capture       CaptureSettings       `json:"capture"`
	Frame         FrameSettings         `json:"frame"`
	Storage       StorageSettings       `json:"storage"`
	MediaDelivery MediaDeliverySettings `json:"media_delivery"`
	Payment       PaymentSettings       `json:"payment"`
}

type CameraSettings struct {
	Provider string  `json:"provider"`
	Device   string  `json:"device,omitempty"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	FPS      float64 `json:"fps"`
}

type CaptureSettings struct {
	Shots     int    `json:"shots"`
	Countdown string `json:"countdown"`
	Interval  string `json:"interval"`
}

type FrameSettings struct {
	ConfigPath string `json:"config_path"`
}

type StorageSettings struct {
	SessionsDir          string `json:"sessions_dir"`
	OutputDir            string `json:"output_dir"`
	SessionRetentionDays int    `json:"session_retention_days,omitempty"`
	OutputRetentionDays  int    `json:"output_retention_days,omitempty"`
}

type MediaDeliverySettings struct {
	Mode string `json:"mode"`
}

type PaymentSettings struct {
	Mode string `json:"mode"`
}

// Defaults returns current local/offline defaults. Paths remain relative in this
// value; Load resolves them from the directory containing the config file.
func Defaults() StoreConfig {
	return StoreConfig{
		SchemaVersion: CurrentSchemaVersion,
		Camera:        CameraSettings{Provider: "fake", Width: 1280, Height: 720, FPS: 30},
		Capture:       CaptureSettings{Shots: 3, Countdown: "3s", Interval: "1s"},
		Frame:         FrameSettings{ConfigPath: filepath.Join("frames", "photo-strip-v1.json")},
		Storage: StorageSettings{
			SessionsDir: filepath.Join("data", "sessions"),
			OutputDir:   filepath.Join("data", "output"),
		},
		MediaDelivery: MediaDeliverySettings{Mode: "local"},
		Payment:       PaymentSettings{Mode: "disabled"},
	}
}

// Load reads a strict JSON config. Unknown properties, multiple JSON documents,
// oversized files and unsupported schema versions are rejected. Missing
// fields inherit defaults, and relative paths resolve from the config file's
// directory.
func Load(path string) (StoreConfig, error) {
	var cfg StoreConfig
	path = strings.TrimSpace(path)
	if path == "" {
		return cfg, errors.New("store config path is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return cfg, fmt.Errorf("open store config: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return cfg, fmt.Errorf("stat store config: %w", err)
	}
	if !info.Mode().IsRegular() {
		return cfg, errors.New("store config must be a regular file")
	}
	if info.Size() > maxConfigBytes {
		return cfg, fmt.Errorf("store config exceeds the %d-byte limit", maxConfigBytes)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxConfigBytes+1))
	if err != nil {
		return cfg, fmt.Errorf("read store config: %w", err)
	}
	if len(data) > maxConfigBytes {
		return cfg, fmt.Errorf("store config exceeds the %d-byte limit", maxConfigBytes)
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return cfg, fmt.Errorf("parse store config JSON: %w", err)
	}
	rawVersion, ok := envelope["schema_version"]
	if !ok {
		return cfg, errors.New("store config must include schema_version")
	}
	if strings.TrimSpace(string(rawVersion)) == "" || strings.TrimSpace(string(rawVersion)) == "null" {
		return cfg, errors.New("store config schema_version must be an integer")
	}
	var schemaVersion int
	if err := json.Unmarshal(rawVersion, &schemaVersion); err != nil {
		return cfg, fmt.Errorf("store config schema_version must be an integer: %w", err)
	}
	if schemaVersion != CurrentSchemaVersion {
		return cfg, fmt.Errorf("unsupported store config schema_version %d (supported: %d)", schemaVersion, CurrentSchemaVersion)
	}

	cfg = Defaults()
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return StoreConfig{}, fmt.Errorf("decode store config: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return StoreConfig{}, errors.New("store config must contain exactly one JSON document")
		}
		return StoreConfig{}, fmt.Errorf("read trailing store config data: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return StoreConfig{}, err
	}

	base, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return StoreConfig{}, fmt.Errorf("resolve store config directory: %w", err)
	}
	cfg.Frame.ConfigPath = resolvePath(base, cfg.Frame.ConfigPath)
	cfg.Storage.SessionsDir = resolvePath(base, cfg.Storage.SessionsDir)
	cfg.Storage.OutputDir = resolvePath(base, cfg.Storage.OutputDir)
	if err := validateLocalStoragePath(cfg.Storage.SessionsDir); err != nil {
		return StoreConfig{}, fmt.Errorf("storage.sessions_dir: %w", err)
	}
	if err := validateLocalStoragePath(cfg.Storage.OutputDir); err != nil {
		return StoreConfig{}, fmt.Errorf("storage.output_dir: %w", err)
	}
	return cfg, nil
}

// Save validates and atomically persists a station config. Paths contained
// inside the config directory are stored relatively to keep station folders
// portable; paths outside it remain absolute.
func Save(path string, cfg StoreConfig) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("store config path is required")
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve store config path: %w", err)
	}
	base := filepath.Dir(absolutePath)
	resolvedSessions := resolvePath(base, cfg.Storage.SessionsDir)
	resolvedOutput := resolvePath(base, cfg.Storage.OutputDir)
	if err := validateLocalStoragePath(resolvedSessions); err != nil {
		return fmt.Errorf("storage.sessions_dir: %w", err)
	}
	if err := validateLocalStoragePath(resolvedOutput); err != nil {
		return fmt.Errorf("storage.output_dir: %w", err)
	}
	persisted := cfg
	persisted.Frame.ConfigPath = portablePath(base, cfg.Frame.ConfigPath)
	persisted.Storage.SessionsDir = portablePath(base, cfg.Storage.SessionsDir)
	persisted.Storage.OutputDir = portablePath(base, cfg.Storage.OutputDir)

	data, err := json.MarshalIndent(persisted, "", "  ")
	if err != nil {
		return fmt.Errorf("encode store config: %w", err)
	}
	if len(data) > maxConfigBytes {
		return fmt.Errorf("encoded store config exceeds the %d-byte limit", maxConfigBytes)
	}
	if err := os.MkdirAll(base, 0o700); err != nil {
		return fmt.Errorf("create store config directory: %w", err)
	}
	temp, err := os.CreateTemp(base, ".store-config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary store config: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return fmt.Errorf("secure temporary store config: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write temporary store config: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("flush temporary store config: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary store config: %w", err)
	}
	if err := os.Rename(tempPath, absolutePath); err != nil {
		return fmt.Errorf("replace store config: %w", err)
	}
	return nil
}

func portablePath(base, value string) string {
	if !filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	relative, err := filepath.Rel(base, value)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return filepath.Clean(value)
	}
	return relative
}

func resolvePath(base, value string) string {
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Clean(filepath.Join(base, value))
}

// Validate checks currently implemented modes and configuration boundaries.
// Frame-file existence and slot-count matching remain the workflow's job.
func (cfg StoreConfig) Validate() error {
	if cfg.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("unsupported store config schema_version %d (supported: %d)", cfg.SchemaVersion, CurrentSchemaVersion)
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Camera.Provider)) {
	case "fake", "windowsmf":
	default:
		return fmt.Errorf("unsupported camera.provider %q (choose fake or windowsmf)", cfg.Camera.Provider)
	}
	if cfg.Camera.Width < 1 || cfg.Camera.Width > maxDimension {
		return fmt.Errorf("camera.width must be between 1 and %d", maxDimension)
	}
	if cfg.Camera.Height < 1 || cfg.Camera.Height > maxDimension {
		return fmt.Errorf("camera.height must be between 1 and %d", maxDimension)
	}
	if math.IsNaN(cfg.Camera.FPS) || math.IsInf(cfg.Camera.FPS, 0) || cfg.Camera.FPS <= 0 || cfg.Camera.FPS > 120 {
		return errors.New("camera.fps must be greater than 0 and at most 120")
	}
	if cfg.Capture.Shots < 1 || cfg.Capture.Shots > maxCaptureShots {
		return fmt.Errorf("capture.shots must be between 1 and %d", maxCaptureShots)
	}
	if _, err := parseDuration("capture.countdown", cfg.Capture.Countdown); err != nil {
		return err
	}
	if _, err := parseDuration("capture.interval", cfg.Capture.Interval); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.Frame.ConfigPath) == "" {
		return errors.New("frame.config_path is required")
	}
	if strings.TrimSpace(cfg.Storage.SessionsDir) == "" {
		return errors.New("storage.sessions_dir is required")
	}
	if strings.TrimSpace(cfg.Storage.OutputDir) == "" {
		return errors.New("storage.output_dir is required")
	}
	if err := validateLocalStoragePath(cfg.Storage.SessionsDir); err != nil {
		return fmt.Errorf("storage.sessions_dir: %w", err)
	}
	if err := validateLocalStoragePath(cfg.Storage.OutputDir); err != nil {
		return fmt.Errorf("storage.output_dir: %w", err)
	}
	if cfg.Storage.SessionRetentionDays < 0 || cfg.Storage.SessionRetentionDays > 3650 {
		return errors.New("storage.session_retention_days must be 0 (disabled) or between 1 and 3650")
	}
	if cfg.Storage.OutputRetentionDays < 0 || cfg.Storage.OutputRetentionDays > 3650 {
		return errors.New("storage.output_retention_days must be 0 (disabled) or between 1 and 3650")
	}
	if mode := strings.ToLower(strings.TrimSpace(cfg.MediaDelivery.Mode)); mode != "local" {
		return fmt.Errorf("unsupported media_delivery.mode %q (only local is implemented)", cfg.MediaDelivery.Mode)
	}
	if mode := strings.ToLower(strings.TrimSpace(cfg.Payment.Mode)); mode != "disabled" {
		return fmt.Errorf("unsupported payment.mode %q (only disabled/free mode is implemented)", cfg.Payment.Mode)
	}
	return nil
}

func (cfg StoreConfig) CountdownDuration() (time.Duration, error) {
	return parseDuration("capture.countdown", cfg.Capture.Countdown)
}

func (cfg StoreConfig) IntervalDuration() (time.Duration, error) {
	return parseDuration("capture.interval", cfg.Capture.Interval)
}

func parseDuration(field, value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("%s is required (use \"0s\" to disable)", field)
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a Go duration such as \"3s\" or \"0s\": %w", field, err)
	}
	if duration < 0 || duration > maxDuration {
		return 0, fmt.Errorf("%s must be between 0 and %s", field, maxDuration)
	}
	return duration, nil
}

// ValidateLocalStoragePath checks an absolute or working-directory-relative
// storage root against the current platform's known network-filesystem rules.
// Call it again near the time of output creation; config validation alone
// cannot protect against mounts or directory links changing while the app runs.
func ValidateLocalStoragePath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("storage path is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve storage path: %w", err)
	}
	return validateLocalStoragePath(absolute)
}
