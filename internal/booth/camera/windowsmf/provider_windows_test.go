//go:build windows

package windowsmf

import (
	"errors"
	"strings"
	"testing"

	mfconsts "github.com/Kirizu-Official/windows-camera-go/windows/consts"
)

func TestDiagnoseCaptureErrorExplainsBusyOrResourceLimitedCamera(t *testing.T) {
	original := mfconsts.HResultError{Code: 0xC00D3704}
	got := diagnoseCaptureError(original)
	if got == nil {
		t.Fatal("expected a diagnostic error")
	}
	message := got.Error()
	for _, expected := range []string{
		"MF_E_HW_MFT_FAILED_START_STREAMING",
		"0xC00D3704",
		"another application",
		"Close other camera apps",
	} {
		if !strings.Contains(message, expected) {
			t.Fatalf("diagnostic %q does not contain %q", message, expected)
		}
	}
	var preserved mfconsts.HResultError
	if !errors.As(got, &preserved) || preserved.Code != original.Code {
		t.Fatalf("original HRESULT was not preserved: %v", got)
	}
}

func TestDiagnoseCaptureErrorLeavesOtherErrorsUnchanged(t *testing.T) {
	original := errors.New("unrelated camera decode error")
	if got := diagnoseCaptureError(original); got != original {
		t.Fatalf("unrelated error should remain unchanged; got %v", got)
	}
}
