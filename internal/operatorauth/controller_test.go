package operatorauth

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSetupAuthenticateAndFreshProcessStartsCustomer(t *testing.T) {
	path := StorePath(filepath.Join(t.TempDir(), "PhotoBooth"))
	controller := New(path)
	state, err := controller.Load()
	if err != nil {
		t.Fatal(err)
	}
	if state.Mode != "setup_required" || state.Configured {
		t.Fatalf("unexpected unconfigured state: %+v", state)
	}
	if err := controller.Setup("1234"); !errors.Is(err, ErrInvalidPIN) {
		t.Fatalf("Setup invalid PIN error = %v", err)
	}
	if err := controller.Setup("12345678"); err != nil {
		t.Fatal(err)
	}
	if err := controller.Setup("87654321"); !errors.Is(err, ErrAlreadyConfigured) {
		t.Fatalf("second Setup error = %v", err)
	}

	state, err = controller.Authenticate("12345678")
	if err != nil {
		t.Fatal(err)
	}
	if state.Mode != "operator" || !state.Configured {
		t.Fatalf("unexpected authenticated state: %+v", state)
	}
	if err := controller.Authorize(); err != nil {
		t.Fatal(err)
	}

	restarted := New(path)
	state, err = restarted.Load()
	if err != nil {
		t.Fatal(err)
	}
	if state.Mode != "customer" || !state.Configured {
		t.Fatalf("restart restored operator access: %+v", state)
	}
	if err := restarted.Authorize(); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("Authorize after restart = %v", err)
	}
}

func TestFailedPINBackoffPersistsAcrossControllerRestart(t *testing.T) {
	path := StorePath(filepath.Join(t.TempDir(), "PhotoBooth"))
	first := New(path)
	if err := first.Setup("12345678"); err != nil {
		t.Fatal(err)
	}
	state, err := first.Authenticate("00000000")
	if !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("wrong PIN error = %v", err)
	}
	if state.RetryAfterSeconds < 1 {
		t.Fatalf("expected persisted backoff, got %+v", state)
	}

	restarted := New(path)
	if _, err := restarted.Load(); err != nil {
		t.Fatal(err)
	}
	state, err = restarted.Authenticate("12345678")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("valid PIN during backoff error = %v", err)
	}
	if state.RetryAfterSeconds < 1 {
		t.Fatalf("expected retry-after after restart, got %+v", state)
	}
}

func TestResetRequiresCurrentPINAndInvalidatesOperatorMode(t *testing.T) {
	path := StorePath(filepath.Join(t.TempDir(), "PhotoBooth"))
	controller := New(path)
	if err := controller.Setup("12345678"); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Authenticate("12345678"); err != nil {
		t.Fatal(err)
	}
	if err := controller.Reset("00000000", "87654321"); !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("Reset wrong current PIN error = %v", err)
	}
	if err := controller.Reset("12345678", "87654321"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("Reset during backoff error = %v", err)
	}

	store, err := readStore(path)
	if err != nil {
		t.Fatal(err)
	}
	store.NextAllowedAt = time.Now().Add(-time.Second)
	if err := writeStore(path, *store, true); err != nil {
		t.Fatal(err)
	}
	if err := controller.Reset("12345678", "87654321"); err != nil {
		t.Fatal(err)
	}
	if err := controller.Authorize(); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("authorization survived credential reset: %v", err)
	}
	if _, err := controller.Authenticate("12345678"); !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("old PIN accepted: %v", err)
	}

	store, err = readStore(path)
	if err != nil {
		t.Fatal(err)
	}
	store.NextAllowedAt = time.Time{}
	if err := writeStore(path, *store, true); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Authenticate("87654321"); err != nil {
		t.Fatalf("new PIN rejected: %v", err)
	}
}

func TestOperatorInactivityTimeoutRevokesAuthorization(t *testing.T) {
	path := StorePath(filepath.Join(t.TempDir(), "PhotoBooth"))
	controller := New(path)
	if err := controller.Setup("12345678"); err != nil {
		t.Fatal(err)
	}
	if err := controller.SetInactivityTimeout(25 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Authenticate("12345678"); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if controller.Status().Mode == "customer" {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("operator access did not expire")
}

func TestCorruptOrInsecureStoreFailsClosed(t *testing.T) {
	dir := t.TempDir()
	path := StorePath(dir)
	if err := os.WriteFile(path, []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	controller := New(path)
	state, err := controller.Load()
	if !errors.Is(err, ErrInvalidStore) {
		t.Fatalf("Load corrupt store error = %v", err)
	}
	if state.Mode != "startup_blocked" {
		t.Fatalf("corrupt store did not fail closed: %+v", state)
	}
}

func TestResetFromOSRequiresExistingStoreAndReplacesPIN(t *testing.T) {
	path := StorePath(filepath.Join(t.TempDir(), "PhotoBooth"))
	controller := New(path)
	if err := controller.ResetFromOS("87654321"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("ResetFromOS without a credential error = %v, want ErrNotConfigured", err)
	}
	if err := controller.Setup("12345678"); err != nil {
		t.Fatal(err)
	}
	if err := controller.ResetFromOS("87654321"); err != nil {
		t.Fatal(err)
	}
	restarted := New(path)
	state, err := restarted.Load()
	if err != nil {
		t.Fatal(err)
	}
	if state.Mode != "customer" || !state.Configured {
		t.Fatalf("reset startup state = %+v", state)
	}
	if _, err := restarted.Authenticate("12345678"); !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("old PIN accepted after OS recovery: %v", err)
	}
	store, err := readStore(path)
	if err != nil {
		t.Fatal(err)
	}
	store.NextAllowedAt = time.Time{}
	if err := writeStore(path, *store, true); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.Authenticate("87654321"); err != nil {
		t.Fatalf("new PIN rejected after OS recovery: %v", err)
	}
}
