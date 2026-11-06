package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
	"github.com/dongsinhho/PhotoBooth/internal/booth/render"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// PlayerSettings is the small, supported portion of station configuration
// editable from the Wails player. Camera/backend and delivery/payment policies
// remain operator-managed configuration rather than guest-facing controls.
type PlayerSettings struct {
	ShotCount            int    `json:"shot_count"`
	Countdown            string `json:"countdown"`
	Interval             string `json:"interval"`
	SessionRetentionDays int    `json:"session_retention_days"`
	OutputRetentionDays  int    `json:"output_retention_days"`
	FramePath            string `json:"frame_path"`
	ConfigPath           string `json:"config_path"`
	LoadWarning          string `json:"load_warning,omitempty"`
}

func (a *App) GetPlayerSettings() (PlayerSettings, error) {
	if err := a.preparePlayerPaths(); err != nil {
		return PlayerSettings{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.playerSettingsLocked(), nil
}

func (a *App) playerSettingsLocked() PlayerSettings {
	cfg := a.stationConfig
	shotCount := cfg.Capture.Shots
	if shotCount < 1 {
		shotCount = playerShotCount
	}
	countdown := strings.TrimSpace(cfg.Capture.Countdown)
	if countdown == "" {
		countdown = "3s"
	}
	interval := strings.TrimSpace(cfg.Capture.Interval)
	if interval == "" {
		interval = "1s"
	}
	framePath := cfg.Frame.ConfigPath
	if framePath == "" {
		framePath = a.framePath
	}
	return PlayerSettings{
		ShotCount:            shotCount,
		Countdown:            countdown,
		Interval:             interval,
		SessionRetentionDays: cfg.Storage.SessionRetentionDays,
		OutputRetentionDays:  cfg.Storage.OutputRetentionDays,
		FramePath:            framePath,
		ConfigPath:           a.settingsPath,
		LoadWarning:          a.settingsError,
	}
}

// BrowseFrameConfig opens a local JSON file picker. It never uploads the
// selected file; SavePlayerSettings validates it before persisting the path.
func (a *App) BrowseFrameConfig() (string, error) {
	if a.ctx == nil {
		return "", errors.New("the frame file picker is unavailable before the UI starts")
	}
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:   "Choose a PhotoBooth frame JSON",
		Filters: []wailsruntime.FileFilter{{DisplayName: "PhotoBooth frame JSON", Pattern: "*.json"}},
	})
	if err != nil {
		return "", fmt.Errorf("choose frame JSON: %w", err)
	}
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve selected frame path: %w", err)
	}
	return absolute, nil
}

// SavePlayerSettings validates the frame and shot/slot relationship before
// atomically persisting the profile. Settings cannot change during a session.
func (a *App) SavePlayerSettings(settings PlayerSettings) (PlayerSettings, error) {
	if err := a.preparePlayerPaths(); err != nil {
		return PlayerSettings{}, err
	}
	a.mu.Lock()
	if a.playerRunning {
		a.mu.Unlock()
		return PlayerSettings{}, errors.New("settings cannot be changed while a photo session is running")
	}
	if a.outputRetrying {
		a.mu.Unlock()
		return PlayerSettings{}, errors.New("settings cannot be changed while a local output retry is running")
	}
	if a.settingsSaving {
		a.mu.Unlock()
		return PlayerSettings{}, errors.New("a settings save is already in progress")
	}
	if a.retentionBusy {
		a.mu.Unlock()
		return PlayerSettings{}, errors.New("settings cannot be changed while retention cleanup is running")
	}
	configPath := strings.TrimSpace(a.settingsPath)
	if configPath == "" {
		a.mu.Unlock()
		return PlayerSettings{}, errors.New("station settings path is not initialized")
	}
	currentFramePath := a.framePath
	a.settingsSaving = true
	cfg := a.stationConfig
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.settingsSaving = false
		a.mu.Unlock()
	}()

	settings.FramePath = strings.TrimSpace(settings.FramePath)
	if settings.FramePath == "" {
		return PlayerSettings{}, errors.New("frame path is required")
	}
	if !filepath.IsAbs(settings.FramePath) {
		settings.FramePath = filepath.Join(filepath.Dir(configPath), settings.FramePath)
	}
	framePath, err := filepath.Abs(settings.FramePath)
	if err != nil {
		return PlayerSettings{}, fmt.Errorf("resolve frame path: %w", err)
	}
	managedPackages := filepath.Join(filepath.Dir(configPath), "frames", "packages")
	if pathWithin(managedPackages, framePath) && !sameLocalPath(framePath, currentFramePath) {
		return PlayerSettings{}, errors.New("activate installed package frames from the Frame packages list so a rollback point is created")
	}
	info, err := os.Stat(framePath)
	if err != nil {
		return PlayerSettings{}, fmt.Errorf("inspect frame JSON: %w", err)
	}
	if !info.Mode().IsRegular() {
		return PlayerSettings{}, errors.New("frame path must point to a regular JSON file")
	}
	if settings.ShotCount < 1 || settings.ShotCount > 12 {
		return PlayerSettings{}, errors.New("shot count must be between 1 and 12")
	}

	cfg.SchemaVersion = stationconfig.CurrentSchemaVersion
	cfg.Camera.Provider = "windowsmf"
	if cfg.Camera.Width < 1 {
		cfg.Camera.Width = playerWidth
	}
	if cfg.Camera.Height < 1 {
		cfg.Camera.Height = playerHeight
	}
	if cfg.Camera.FPS <= 0 {
		cfg.Camera.FPS = 30
	}
	cfg.Capture.Shots = settings.ShotCount
	cfg.Capture.Countdown = strings.TrimSpace(settings.Countdown)
	cfg.Capture.Interval = strings.TrimSpace(settings.Interval)
	cfg.Storage.SessionRetentionDays = settings.SessionRetentionDays
	cfg.Storage.OutputRetentionDays = settings.OutputRetentionDays
	cfg.Frame.ConfigPath = framePath
	cfg.Storage.SessionsDir = a.workDir
	cfg.Storage.OutputDir = a.outputDir
	if cfg.MediaDelivery.Mode == "" {
		cfg.MediaDelivery.Mode = "local"
	}
	if cfg.Payment.Mode == "" {
		cfg.Payment.Mode = "disabled"
	}
	if err := cfg.Validate(); err != nil {
		return PlayerSettings{}, err
	}
	slots, err := render.New().SlotCount(context.Background(), framePath)
	if err != nil {
		return PlayerSettings{}, fmt.Errorf("validate frame JSON: %w", err)
	}
	if slots != cfg.Capture.Shots {
		return PlayerSettings{}, fmt.Errorf("shot count is %d but the selected frame defines %d slots", cfg.Capture.Shots, slots)
	}
	countdown, err := cfg.CountdownDuration()
	if err != nil {
		return PlayerSettings{}, err
	}
	interval, err := cfg.IntervalDuration()
	if err != nil {
		return PlayerSettings{}, err
	}
	if err := stationconfig.Save(configPath, cfg); err != nil {
		return PlayerSettings{}, fmt.Errorf("save station config: %w", err)
	}

	a.mu.Lock()
	a.stationConfig = cfg
	a.settingsError = ""
	a.retentionPlan = nil
	a.retentionPlanDigest = ""
	a.framePath = cfg.Frame.ConfigPath
	a.workDir = cfg.Storage.SessionsDir
	a.outputDir = cfg.Storage.OutputDir
	a.playerCountdown = countdown
	a.playerInterval = interval
	a.playerShotCount = cfg.Capture.Shots
	if a.player.State == "idle" {
		a.player.Total = cfg.Capture.Shots
	}
	snapshot := a.playerSettingsLocked()
	a.mu.Unlock()

	if !sameLocalPath(framePath, currentFramePath) {
		if err := newFramePackageManager(configPath).ClearRollback(); err != nil {
			return snapshot, fmt.Errorf("settings were saved, but obsolete frame rollback cleanup failed: %w", err)
		}
	}
	return snapshot, nil
}

func pathWithin(parent, child string) bool {
	parentAbsolute, errParent := filepath.Abs(parent)
	childAbsolute, errChild := filepath.Abs(child)
	if errParent != nil || errChild != nil {
		return false
	}
	relative, err := filepath.Rel(filepath.Clean(parentAbsolute), filepath.Clean(childAbsolute))
	if err != nil || relative == "." || filepath.IsAbs(relative) || relative == ".." ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

// loadStationConfig initializes the local, offline station profile only once.
// An invalid existing config is never overwritten automatically. A safe in-memory
// default keeps the settings screen available so the operator can repair it.
func (a *App) loadStationConfig(base string) error {
	framesDir := filepath.Join(base, "frames")
	defaultFrame := filepath.Join(framesDir, "photo-strip-v1.json")
	workDir := filepath.Join(base, "sessions")
	outputDir := filepath.Join(base, "photos")
	for _, dir := range []string{framesDir, workDir, outputDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("prepare PhotoBooth directory: %w", err)
		}
	}
	if _, err := os.Stat(defaultFrame); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(defaultFrame, embeddedSampleFrame, 0o600); err != nil {
			return fmt.Errorf("install sample frame: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("inspect sample frame: %w", err)
	}

	configPath := filepath.Join(base, "store-config-v1.json")
	cfg, err := stationconfig.Load(configPath)
	if errors.Is(err, os.ErrNotExist) {
		cfg = stationconfig.Defaults()
		cfg.Camera.Provider = "windowsmf"
		cfg.Camera.Device = strings.TrimSpace(os.Getenv("PHOTOBOOTH_CAMERA_DEVICE"))
		cfg.Frame.ConfigPath = defaultFrame
		cfg.Storage.SessionsDir = workDir
		cfg.Storage.OutputDir = outputDir
		if err := stationconfig.Save(configPath, cfg); err != nil {
			return fmt.Errorf("create default station config: %w", err)
		}
	} else if err != nil {
		cfg = stationconfig.Defaults()
		cfg.Camera.Provider = "windowsmf"
		cfg.Camera.Device = strings.TrimSpace(os.Getenv("PHOTOBOOTH_CAMERA_DEVICE"))
		cfg.Frame.ConfigPath = defaultFrame
		cfg.Storage.SessionsDir = workDir
		cfg.Storage.OutputDir = outputDir
		a.settingsError = "Saved station config is invalid and has not been overwritten: " + err.Error() + ". Review the settings and save a valid replacement."
	}

	if cfg.Camera.Provider != "windowsmf" && a.settingsError == "" {
		a.settingsError = fmt.Sprintf("Wails real-camera sessions require camera.provider=windowsmf; the saved value is %q. Save the settings to repair this profile.", cfg.Camera.Provider)
	}
	if slots, slotErr := render.New().SlotCount(context.Background(), cfg.Frame.ConfigPath); slotErr != nil {
		if a.settingsError == "" {
			a.settingsError = "The saved frame JSON cannot be loaded: " + slotErr.Error() + ". Choose a valid frame and save settings."
		}
	} else if slots != cfg.Capture.Shots && a.settingsError == "" {
		a.settingsError = fmt.Sprintf("The saved capture count is %d but the selected frame has %d slots. Adjust one of them and save settings.", cfg.Capture.Shots, slots)
	}

	cfg.Storage.SessionsDir = filepath.Clean(cfg.Storage.SessionsDir)
	cfg.Storage.OutputDir = filepath.Clean(cfg.Storage.OutputDir)
	for _, dir := range []string{cfg.Storage.SessionsDir, cfg.Storage.OutputDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("prepare configured PhotoBooth directory: %w", err)
		}
	}
	countdown, countdownErr := cfg.CountdownDuration()
	interval, intervalErr := cfg.IntervalDuration()
	if countdownErr != nil || intervalErr != nil {
		if a.settingsError == "" {
			a.settingsError = "Saved capture durations are invalid. Enter valid durations (for example 3s or 0s) and save settings."
		}
		countdown, interval = 3*time.Second, time.Second
	}
	if cfg.Capture.Shots < 1 || cfg.Capture.Shots > 12 {
		if a.settingsError == "" {
			a.settingsError = "Saved shot count is invalid. Choose 1–12 shots and save settings."
		}
		cfg.Capture.Shots = playerShotCount
	}
	a.settingsPath = configPath
	a.stationConfig = cfg
	a.framePath, a.workDir, a.outputDir = cfg.Frame.ConfigPath, cfg.Storage.SessionsDir, cfg.Storage.OutputDir
	a.playerCountdown, a.playerInterval, a.playerShotCount = countdown, interval, cfg.Capture.Shots
	return nil
}
