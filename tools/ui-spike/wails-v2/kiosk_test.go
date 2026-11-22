package main

import "testing"

func TestKioskModeDefaultsToCustomerFullscreen(t *testing.T) {
	for _, value := range []string{"", "enabled", "tru", "unexpected"} {
		if !kioskModeFromArgsAndEnv(nil, value) {
			t.Errorf("kioskModeFromArgsAndEnv(nil, %q) = false; want fullscreen default", value)
		}
	}
}

func TestKioskModeCanBeExplicitlyWindowed(t *testing.T) {
	for _, value := range []string{"0", "false", "no", "off", "windowed"} {
		if kioskModeFromArgsAndEnv(nil, value) {
			t.Errorf("kioskModeFromArgsAndEnv(nil, %q) = true; want windowed", value)
		}
	}
	if kioskModeFromArgsAndEnv([]string{"--windowed"}, "1") {
		t.Fatal("--windowed must override an enabled kiosk environment variable")
	}
}

func TestKioskModeCanBeExplicitlyEnabled(t *testing.T) {
	for _, value := range []string{"1", "true", "TRUE", "yes", "On"} {
		if !kioskModeFromArgsAndEnv(nil, value) {
			t.Errorf("kioskModeFromArgsAndEnv(nil, %q) = false; want true", value)
		}
	}
	if !kioskModeFromArgsAndEnv([]string{"--kiosk"}, "0") {
		t.Fatal("--kiosk must explicitly enable fullscreen")
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
