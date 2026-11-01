//go:build windows

package main

import (
	"testing"

	mfconsts "github.com/Kirizu-Official/windows-camera-go/windows/consts"
)

func TestControlMode(t *testing.T) {
	tests := []struct {
		name       string
		flags      int32
		autoFlag   int32
		manualFlag int32
		want       string
	}{
		{name: "auto", flags: 1, autoFlag: 1, manualFlag: 2, want: "auto"},
		{name: "manual", flags: 2, autoFlag: 1, manualFlag: 2, want: "manual"},
		{name: "unknown flags", flags: 0, autoFlag: 1, manualFlag: 2, want: "unknown"},
		{name: "auto takes priority for combined flags", flags: 3, autoFlag: 1, manualFlag: 2, want: "auto"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := controlMode(tt.flags, tt.autoFlag, tt.manualFlag); got != tt.want {
				t.Fatalf("controlMode(%d)=%q, want %q", tt.flags, got, tt.want)
			}
		})
	}
}

func TestGainStateMatches(t *testing.T) {
	tests := []struct {
		name    string
		current controlValue
		value   int32
		mode    string
		flags   mfconsts.VideoProcAmpFlags
		want    bool
	}{
		{
			name:    "exact manual readback",
			current: controlValue{Available: true, Value: 128, RawFlags: 2, Mode: "manual"},
			value:   128, mode: "manual", flags: mfconsts.KSPROPERTY_VIDEOPROCAMP_FLAGS_MANUAL, want: true,
		},
		{
			name:    "manual value did not change",
			current: controlValue{Available: true, Value: 128, RawFlags: 2, Mode: "manual"},
			value:   255, mode: "manual", flags: mfconsts.KSPROPERTY_VIDEOPROCAMP_FLAGS_MANUAL, want: false,
		},
		{
			name:    "wrong flags despite matching value",
			current: controlValue{Available: true, Value: 128, RawFlags: 1, Mode: "auto"},
			value:   128, mode: "manual", flags: mfconsts.KSPROPERTY_VIDEOPROCAMP_FLAGS_MANUAL, want: false,
		},
		{
			name:    "auto value may vary but mode and flags must match",
			current: controlValue{Available: true, Value: 174, RawFlags: 1, Mode: "auto"},
			value:   128, mode: "auto", flags: mfconsts.KSPROPERTY_VIDEOPROCAMP_FLAGS_AUTO, want: true,
		},
		{
			name:    "unavailable control",
			current: controlValue{Available: false, Value: 128, RawFlags: 2, Mode: "manual"},
			value:   128, mode: "manual", flags: mfconsts.KSPROPERTY_VIDEOPROCAMP_FLAGS_MANUAL, want: false,
		},
		{
			name:    "readback error",
			current: controlValue{Available: true, Value: 128, RawFlags: 2, Mode: "manual", Error: "driver error"},
			value:   128, mode: "manual", flags: mfconsts.KSPROPERTY_VIDEOPROCAMP_FLAGS_MANUAL, want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gainStateMatches(tt.current, tt.value, tt.mode, tt.flags)
			if got != tt.want {
				t.Fatalf("gainStateMatches(%+v, %d, %q, %d)=%v, want %v",
					tt.current, tt.value, tt.mode, tt.flags, got, tt.want)
			}
		})
	}
}
