//go:build windows

package mf

import (
	"testing"

	"github.com/Kirizu-Official/windows-camera-go/windows/consts"
)

func TestControlSetResultError(t *testing.T) {
	tests := []struct {
		name      string
		hr        uintptr
		wantError bool
	}{
		{name: "S_OK is success", hr: uintptr(consts.S_OK), wantError: false},
		{name: "ERROR_NOT_FOUND is not a successful set", hr: uintptr(consts.ERROR_NOT_FOUND), wantError: true},
		{name: "E_FAIL is an error", hr: uintptr(consts.E_FAIL), wantError: true},
		{name: "S_FALSE is not a verified applied setting", hr: 1, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := controlSetResultError(tt.hr)
			if (err != nil) != tt.wantError {
				t.Fatalf("controlSetResultError(0x%x) error=%v, wantError=%v", tt.hr, err, tt.wantError)
			}
		})
	}
}
