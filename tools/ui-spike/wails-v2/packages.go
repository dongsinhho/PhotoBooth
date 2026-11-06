package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
	"github.com/dongsinhho/PhotoBooth/internal/booth/framepackage"
	"github.com/dongsinhho/PhotoBooth/internal/booth/render"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// PlayerFramePackage is a narrow DTO for Wails. It excludes raw manifests and
// never exposes package scripts or executable plugin behavior.
type PlayerFramePackage struct {
	PackageID        string `json:"package_id"`
	PackageVersion   string `json:"package_version"`
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	Author           string `json:"author"`
	License          string `json:"license"`
	FramePath        string `json:"frame_path,omitempty"`
	PreviewPath      string `json:"preview_path,omitempty"`
	SlotCount        int    `json:"slot_count"`
	HasPreview       bool   `json:"has_preview"`
	AlreadyInstalled bool   `json:"already_installed,omitempty"`
	IsActive         bool   `json:"is_active"`
}

type PlayerFramePackageState struct {
	Packages    []PlayerFramePackage `json:"packages"`
	ActiveFrame string               `json:"active_frame"`
	CanRollback bool                 `json:"can_rollback"`
	LoadWarning string               `json:"load_warning,omitempty"`
}

func newFramePackageManager(configPath string) *framepackage.Manager {
	return framepackage.NewManager(filepath.Join(filepath.Dir(configPath), "frames"))
}

func toPlayerFramePackage(info framepackage.PackageInfo) PlayerFramePackage {
	return PlayerFramePackage{
		PackageID: info.PackageID, PackageVersion: info.PackageVersion, Name: info.Name,
		Description: info.Description, Author: info.Author, License: info.License,
		FramePath: info.FramePath, PreviewPath: info.PreviewPath, SlotCount: info.SlotCount,
		HasPreview: info.HasPreview, AlreadyInstalled: info.AlreadyInstalled,
	}
}

// BrowseFramePackage opens a local picker. No package is downloaded by this method.
func (a *App) BrowseFramePackage() (string, error) {
	if a.ctx == nil {
		return "", errors.New("the package file picker is unavailable before the UI starts")
	}
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:   "Choose a PhotoBooth frame package",
		Filters: []wailsruntime.FileFilter{{DisplayName: "PhotoBooth frame package", Pattern: "*.pbframe"}},
	})
	if err != nil {
		return "", fmt.Errorf("choose frame package: %w", err)
	}
	return path, nil
}

// ValidateFramePackage verifies archive bounds, manifest, payload digests, media
// and the production renderer's schema-v1 frame rules without installing anything.
func (a *App) ValidateFramePackage(path string) (PlayerFramePackage, error) {
	if err := a.preparePlayerPaths(); err != nil {
		return PlayerFramePackage{}, err
	}
	a.mu.Lock()
	configPath := a.settingsPath
	a.mu.Unlock()
	info, err := newFramePackageManager(configPath).Validate(context.Background(), path)
	if err != nil {
		return PlayerFramePackage{}, err
	}
	return toPlayerFramePackage(info), nil
}

// InstallFramePackage stores a verified immutable version, without activating it.
func (a *App) InstallFramePackage(path string) (PlayerFramePackage, error) {
	configPath, _, _, err := a.beginFramePackageOperation()
	if err != nil {
		return PlayerFramePackage{}, err
	}
	defer a.endFramePackageOperation()
	info, err := newFramePackageManager(configPath).Install(context.Background(), path)
	if err != nil {
		return PlayerFramePackage{}, err
	}
	return toPlayerFramePackage(info), nil
}

// GetFramePackageState verifies installed versions and recovers interrupted
// activation metadata by comparing the persisted profile with the pending target.
func (a *App) GetFramePackageState() (PlayerFramePackageState, error) {
	if err := a.preparePlayerPaths(); err != nil {
		return PlayerFramePackageState{}, err
	}
	a.mu.Lock()
	configPath, currentFrame, warning := a.settingsPath, a.framePath, a.settingsError
	a.mu.Unlock()
	if strings.TrimSpace(configPath) == "" || strings.TrimSpace(currentFrame) == "" {
		return PlayerFramePackageState{}, errors.New("station settings are not initialized")
	}
	manager := newFramePackageManager(configPath)
	packages, err := manager.List(context.Background())
	if err != nil {
		return PlayerFramePackageState{}, fmt.Errorf("verify installed frame packages: %w", err)
	}
	canRollback, rollbackErr := manager.CanRollback(context.Background(), currentFrame)
	if rollbackErr != nil {
		if warning == "" {
			warning = "Rollback is unavailable because its backup could not be verified: " + rollbackErr.Error()
		}
		canRollback = false
	}
	state := PlayerFramePackageState{
		Packages:    make([]PlayerFramePackage, 0, len(packages)),
		ActiveFrame: currentFrame, CanRollback: canRollback, LoadWarning: warning,
	}
	for _, info := range packages {
		item := toPlayerFramePackage(info)
		item.IsActive = sameLocalPath(info.FramePath, currentFrame)
		state.Packages = append(state.Packages, item)
	}
	return state, nil
}

// UninstallFramePackage removes an inactive package version only when it is not
// referenced by the current rollback transition. The manager re-checks the on-disk
// profile relationship before removing the package directory.
func (a *App) UninstallFramePackage(packageID, version string) error {
	configPath, currentFrame, _, err := a.beginFramePackageOperation()
	if err != nil {
		return err
	}
	defer a.endFramePackageOperation()
	if err := newFramePackageManager(configPath).Uninstall(context.Background(), packageID, version, currentFrame); err != nil {
		return err
	}
	return nil
}

// ActivateFramePackage creates a rollback point, then updates the station config.
// The configured shot count follows the selected frame's validated slot count.
func (a *App) ActivateFramePackage(packageID, version string) (PlayerSettings, error) {
	configPath, currentFrame, cfg, err := a.beginFramePackageOperation()
	if err != nil {
		return PlayerSettings{}, err
	}
	defer a.endFramePackageOperation()
	manager := newFramePackageManager(configPath)
	info, err := manager.PrepareActivation(context.Background(), packageID, version, currentFrame)
	if err != nil {
		return PlayerSettings{}, err
	}
	cfg.Frame.ConfigPath = info.FramePath
	cfg.Capture.Shots = info.SlotCount
	if err := cfg.Validate(); err != nil {
		if abortErr := manager.AbortActivation(); abortErr != nil {
			return PlayerSettings{}, fmt.Errorf("validate frame profile: %v; abort activation: %w", err, abortErr)
		}
		return PlayerSettings{}, fmt.Errorf("validate frame profile: %w", err)
	}
	if err := stationconfig.Save(configPath, cfg); err != nil {
		if abortErr := manager.AbortActivation(); abortErr != nil {
			return PlayerSettings{}, fmt.Errorf("save station config: %v; abort activation: %w", err, abortErr)
		}
		return PlayerSettings{}, fmt.Errorf("save station config; previous frame remains active: %w", err)
	}
	countdown, _ := cfg.CountdownDuration()
	interval, _ := cfg.IntervalDuration()
	a.mu.Lock()
	a.stationConfig, a.settingsError, a.framePath = cfg, "", cfg.Frame.ConfigPath
	a.playerCountdown, a.playerInterval, a.playerShotCount = countdown, interval, cfg.Capture.Shots
	if a.player.State == "idle" {
		a.player.Total = cfg.Capture.Shots
	}
	snapshot := a.playerSettingsLocked()
	a.mu.Unlock()
	if err := manager.CommitActivation(info.FramePath); err != nil {
		return snapshot, fmt.Errorf("frame is activated and saved; rollback metadata needs recovery on refresh: %w", err)
	}
	return snapshot, nil
}

// RollbackActiveFrame switches the profile to a validated restored copy. The
// installed package remains in the catalog and the last-known-good copy is only
// removed after the profile has been saved successfully.
func (a *App) RollbackActiveFrame() (PlayerSettings, error) {
	configPath, currentFrame, cfg, err := a.beginFramePackageOperation()
	if err != nil {
		return PlayerSettings{}, err
	}
	defer a.endFramePackageOperation()
	manager := newFramePackageManager(configPath)
	restoredPath, err := manager.RollbackFramePath(context.Background(), currentFrame)
	if err != nil {
		return PlayerSettings{}, err
	}
	slots, err := render.New().SlotCount(context.Background(), restoredPath)
	if err != nil {
		return PlayerSettings{}, fmt.Errorf("validate restored frame: %w", err)
	}
	cfg.Frame.ConfigPath, cfg.Capture.Shots = restoredPath, slots
	if err := cfg.Validate(); err != nil {
		return PlayerSettings{}, fmt.Errorf("validate restored station config: %w", err)
	}
	if err := stationconfig.Save(configPath, cfg); err != nil {
		return PlayerSettings{}, fmt.Errorf("save restored frame selection; the previous backup is preserved: %w", err)
	}
	countdown, _ := cfg.CountdownDuration()
	interval, _ := cfg.IntervalDuration()
	a.mu.Lock()
	a.stationConfig, a.settingsError, a.framePath = cfg, "", cfg.Frame.ConfigPath
	a.playerCountdown, a.playerInterval, a.playerShotCount = countdown, interval, cfg.Capture.Shots
	if a.player.State == "idle" {
		a.player.Total = cfg.Capture.Shots
	}
	snapshot := a.playerSettingsLocked()
	a.mu.Unlock()
	if err := manager.ClearRollback(); err != nil {
		return snapshot, fmt.Errorf("frame was restored but rollback metadata cleanup failed: %w", err)
	}
	return snapshot, nil
}

func (a *App) beginFramePackageOperation() (string, string, stationconfig.StoreConfig, error) {
	if err := a.preparePlayerPaths(); err != nil {
		return "", "", stationconfig.StoreConfig{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.playerRunning {
		return "", "", stationconfig.StoreConfig{}, errors.New("frame packages cannot be changed while a photo session is running")
	}
	if a.outputRetrying {
		return "", "", stationconfig.StoreConfig{}, errors.New("frame packages cannot be changed while a local output retry is running")
	}
	if a.settingsSaving {
		return "", "", stationconfig.StoreConfig{}, errors.New("another settings/package operation is already running")
	}
	if a.retentionBusy {
		return "", "", stationconfig.StoreConfig{}, errors.New("frame packages cannot be changed while local retention cleanup is running")
	}
	if strings.TrimSpace(a.settingsPath) == "" || strings.TrimSpace(a.framePath) == "" {
		return "", "", stationconfig.StoreConfig{}, errors.New("station settings are not initialized")
	}
	a.settingsSaving = true
	return a.settingsPath, a.framePath, a.stationConfig, nil
}

func (a *App) endFramePackageOperation() {
	a.mu.Lock()
	a.settingsSaving = false
	a.mu.Unlock()
}

func sameLocalPath(a, b string) bool {
	left, errLeft := filepath.Abs(a)
	right, errRight := filepath.Abs(b)
	if errLeft == nil && errRight == nil {
		left, right = filepath.Clean(left), filepath.Clean(right)
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
