package camera

import (
	"errors"
	"reflect"
	"testing"
)

func TestShutdownAndReleaseMediaSourceOrdersCleanup(t *testing.T) {
	var calls []string
	err := shutdownAndReleaseMediaSource(
		func() error {
			calls = append(calls, "shutdown")
			return nil
		},
		func() error {
			calls = append(calls, "release")
			return nil
		},
	)
	if err != nil {
		t.Fatalf("cleanup returned an unexpected error: %v", err)
	}
	if want := []string{"shutdown", "release"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("cleanup order = %v, want %v", calls, want)
	}
}

func TestShutdownAndReleaseMediaSourceReleasesAfterShutdownFailure(t *testing.T) {
	shutdownErr := errors.New("source was invalidated")
	releaseErr := errors.New("release failed")
	released := false

	err := shutdownAndReleaseMediaSource(
		func() error { return shutdownErr },
		func() error {
			released = true
			return releaseErr
		},
	)
	if !released {
		t.Fatal("source COM reference was not released after Shutdown failed")
	}
	if !errors.Is(err, shutdownErr) {
		t.Errorf("cleanup error %v does not retain Shutdown error", err)
	}
	if !errors.Is(err, releaseErr) {
		t.Errorf("cleanup error %v does not retain Release error", err)
	}
}
