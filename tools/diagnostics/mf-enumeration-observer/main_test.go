//go:build windows

package main

import "testing"

func TestDeviceStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		deviceCount int
		want        string
	}{
		{name: "native enumeration returned no sources", deviceCount: 0, want: "zero_sources"},
		{name: "one camera found", deviceCount: 1, want: "camera_found"},
		{name: "multiple cameras found", deviceCount: 2, want: "camera_found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := deviceStatus(tt.deviceCount); got != tt.want {
				t.Fatalf("deviceStatus(%d) = %q, want %q", tt.deviceCount, got, tt.want)
			}
		})
	}
}
