package main

import "testing"

func TestKioskModeIsOptInAndRejectsUnknownValues(t *testing.T) {
	for _, value := range []string{"", "0", "false", "no", "off", "enabled", "tru"} {
		t.Setenv(kioskEnvironmentVariable, value)
		if kioskModeRequested() {
			t.Errorf("kioskModeRequested() = true for %q; want false", value)
		}
	}

	for _, value := range []string{"1", "true", "TRUE", "yes", "On"} {
		t.Setenv(kioskEnvironmentVariable, value)
		if !kioskModeRequested() {
			t.Errorf("kioskModeRequested() = false for %q; want true", value)
		}
	}
}

func TestKioskModeDefaultsOffAndCanBeInitialized(t *testing.T) {
	app := NewApp()
	if app.IsKioskMode() {
		t.Fatal("new app must default to windowed mode")
	}
	app.setKioskMode(true)
	if !app.IsKioskMode() {
		t.Fatal("initialized kiosk mode was not retained")
	}
	app.setKioskMode(false)
	if app.IsKioskMode() {
		t.Fatal("windowed mode was not retained")
	}
}

func TestKioskShortcutsAreSafeWithoutWailsContext(t *testing.T) {
	app := NewApp()
	if got := app.ToggleKioskMode(); got {
		t.Fatal("ToggleKioskMode without a Wails context must stay windowed")
	}
	if got := app.ExitKioskMode(); got {
		t.Fatal("ExitKioskMode without a Wails context must report no transition")
	}
	if app.IsKioskMode() {
		t.Fatal("shortcut calls without a Wails context must not change mode")
	}
}
