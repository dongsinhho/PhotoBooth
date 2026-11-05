package camera

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestAllCameraSourceMetadataFailed(t *testing.T) {
	metadataErr := errors.New("symbolic link attribute unavailable")
	indexedErr := fmt.Errorf("camera source 0: %w", metadataErr)
	err := allCameraSourceMetadataFailed(1, []error{indexedErr})
	if err == nil {
		t.Fatal("expected an error when a native source exists but all metadata reads fail")
	}
	for _, want := range []string{
		"Media Foundation returned 1 camera source(s)",
		"metadata lookup failed for all",
		"camera source 0",
		"symbolic link attribute unavailable",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
	if !errors.Is(err, metadataErr) {
		t.Fatalf("wrapped metadata error is not discoverable with errors.Is: %v", err)
	}
}

func TestAllCameraSourceMetadataFailedDoesNotMaskEmptyEnumeration(t *testing.T) {
	if err := allCameraSourceMetadataFailed(0, nil); err != nil {
		t.Fatalf("zero native sources should remain a clean empty enumeration, got: %v", err)
	}
}
