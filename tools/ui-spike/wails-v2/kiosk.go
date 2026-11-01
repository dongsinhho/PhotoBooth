package main

import (
	"os"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const kioskEnvironmentVariable = "PHOTOBOOTH_KIOSK"

// kioskModeRequested is intentionally opt-in. Unknown values are treated as
// false so a typo can never unexpectedly hide the window decorations.
func kioskModeRequested() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(kioskEnvironmentVariable))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
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
	if a.ctx == nil || !a.IsKioskMode() {
		return false
	}
	wailsruntime.WindowUnfullscreen(a.ctx)
	a.setKioskMode(false)
	return true
}
