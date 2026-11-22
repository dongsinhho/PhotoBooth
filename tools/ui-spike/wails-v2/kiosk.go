package main

import (
	"os"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const kioskEnvironmentVariable = "PHOTOBOOTH_KIOSK"

// kioskModeRequested defaults to fullscreen Customer Mode. Windowed mode is
// an explicit diagnostic/development opt-out; an authenticated operator can
// still leave fullscreen through the protected Wails bindings.
func kioskModeRequested() bool {
	return kioskModeFromArgsAndEnv(os.Args, os.Getenv(kioskEnvironmentVariable))
}

func kioskModeFromArgsAndEnv(args []string, environmentValue string) bool {
	for _, arg := range args {
		if strings.EqualFold(strings.TrimSpace(arg), "--windowed") {
			return false
		}
	}
	for _, arg := range args {
		if strings.EqualFold(strings.TrimSpace(arg), "--kiosk") {
			return true
		}
	}
	switch strings.ToLower(strings.TrimSpace(environmentValue)) {
	case "0", "false", "no", "off", "windowed":
		return false
	default:
		// Empty and unknown values retain the secure guest fullscreen default.
		return true
	}
}

func (a *App) setKioskMode(enabled bool) {
	a.kioskMu.Lock()
	a.kioskMode = enabled
	a.kioskMu.Unlock()
}

// IsKioskMode reports the last mode requested through the Wails window API.
func (a *App) IsKioskMode() bool {
	a.kioskMu.RLock()
	defer a.kioskMu.RUnlock()
	return a.kioskMode
}

// ToggleKioskMode switches between a normal resizable window and fullscreen.
// F11 invokes this method from the frontend. The method is harmless in unit
// tests or before Wails has supplied its runtime context.
func (a *App) ToggleKioskMode() bool {
	if err := a.requireOperator(); err != nil {
		return false
	}
	if a.ctx == nil {
		return a.IsKioskMode()
	}
	if a.IsKioskMode() {
		wailsruntime.WindowUnfullscreen(a.ctx)
		a.setKioskMode(false)
		return false
	}
	wailsruntime.WindowFullscreen(a.ctx)
	a.setKioskMode(true)
	return true
}

// ExitKioskMode exits fullscreen without closing PhotoBooth. Escape invokes
// this method so an operator always has a keyboard escape path.
func (a *App) ExitKioskMode() bool {
	if err := a.requireOperator(); err != nil {
		return false
	}
	if a.ctx == nil || !a.IsKioskMode() {
		return false
	}
	wailsruntime.WindowUnfullscreen(a.ctx)
	a.setKioskMode(false)
	return true
}
