package output

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const retryFixtureSessionID = "0123456789abcdef0123456789abcdef"

func makeRetrySession(t *testing.T, captureCount int) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	sessionsRoot := filepath.Join(root, "sessions")
	outputRoot := filepath.Join(root, "output")
	captureDir := filepath.Join(sessionsRoot, retryFixtureSessionID, "captures")
	if err := os.MkdirAll(captureDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for shot := 1; shot <= captureCount; shot++ {
		path := filepath.Join(captureDir, fmt.Sprintf("capture-%02d.jpg", shot))
		if err := os.WriteFile(path, []byte("capture-"+strconv.Itoa(shot)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	renderedPath := filepath.Join(sessionsRoot, retryFixtureSessionID, "rendered.jpg")
	if err := os.WriteFile(renderedPath, []byte("rendered-photo-content"), 0o600); err != nil {
		t.Fatal(err)
	}
	return sessionsRoot, outputRoot, renderedPath
}

func TestRetryLocalFromSessionPublishesPreservedRender(t *testing.T) {
	sessionsRoot, outputRoot, renderedPath := makeRetrySession(t, 3)
	path, err := RetryLocalFromSession(context.Background(), sessionsRoot, outputRoot, retryFixtureSessionID, 3)
	if err != nil {
		t.Fatalf("RetryLocalFromSession: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read published photo: %v", err)
	}
	source, err := os.ReadFile(renderedPath)
	if err != nil {
		t.Fatalf("read preserved render: %v", err)
	}
	if string(got) != string(source) {
		t.Fatalf("published content %q differs from preserved render %q", got, source)
	}
	if filepath.Base(path) != "photo.jpg" {
		t.Fatalf("output path=%q, want photo.jpg", path)
	}
	for shot := 1; shot <= 3; shot++ {
		capture := filepath.Join(filepath.Dir(renderedPath), "captures", fmt.Sprintf("capture-%02d.jpg", shot))
		if _, err := os.Stat(capture); err != nil {
			t.Fatalf("capture %d was not preserved: %v", shot, err)
		}
	}
	if _, err := os.Stat(renderedPath); err != nil {
		t.Fatalf("rendered source was not preserved: %v", err)
	}
}

func TestRetryLocalFromSessionRejectsUnsafeOrIncompleteInput(t *testing.T) {
	t.Run("invalid session id", func(t *testing.T) {
		sessions, outputs, _ := makeRetrySession(t, 3)
		if _, err := RetryLocalFromSession(context.Background(), sessions, outputs, "../bad", 3); err == nil {
			t.Fatal("expected invalid session ID to be rejected")
		}
	})
	t.Run("missing capture", func(t *testing.T) {
		sessions, outputs, rendered := makeRetrySession(t, 3)
		missing := filepath.Join(filepath.Dir(rendered), "captures", "capture-03.jpg")
		if err := os.Remove(missing); err != nil {
			t.Fatal(err)
		}
		if _, err := RetryLocalFromSession(context.Background(), sessions, outputs, retryFixtureSessionID, 3); err == nil || !strings.Contains(err.Error(), "capture 3") {
			t.Fatalf("error=%v, want missing capture 3", err)
		}
	})
	t.Run("overlapping roots", func(t *testing.T) {
		sessions, _, _ := makeRetrySession(t, 3)
		if _, err := RetryLocalFromSession(context.Background(), sessions, filepath.Join(sessions, "nested-output"), retryFixtureSessionID, 3); err == nil || !strings.Contains(err.Error(), "non-overlapping roots") {
			t.Fatalf("error=%v, want overlapping roots refusal", err)
		}
	})
	t.Run("cancelled context", func(t *testing.T) {
		sessions, outputs, _ := makeRetrySession(t, 3)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := RetryLocalFromSession(ctx, sessions, outputs, retryFixtureSessionID, 3); !errors.Is(err, context.Canceled) {
			t.Fatalf("error=%v, want context cancellation", err)
		}
	})
	t.Run("zero capture count", func(t *testing.T) {
		sessions, outputs, _ := makeRetrySession(t, 3)
		if _, err := RetryLocalFromSession(context.Background(), sessions, outputs, retryFixtureSessionID, 0); err == nil {
			t.Fatal("expected zero capture count to be rejected")
		}
	})
}

func TestRetryLocalFromSessionRefusesExistingOutputWithoutChangingIt(t *testing.T) {
	sessionsRoot, outputRoot, _ := makeRetrySession(t, 3)
	outputDir := filepath.Join(outputRoot, retryFixtureSessionID)
	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		t.Fatal(err)
	}
	finalPath := filepath.Join(outputDir, "photo.jpg")
	previous := []byte("previously-published-photo")
	if err := os.WriteFile(finalPath, previous, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := RetryLocalFromSession(context.Background(), sessionsRoot, outputRoot, retryFixtureSessionID, 3); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("error=%v, want no-overwrite refusal", err)
	}
	got, err := os.ReadFile(finalPath)
	if err != nil || string(got) != string(previous) {
		t.Fatalf("existing photo changed: got=%q err=%v", got, err)
	}
}

func TestRetryLocalFromSessionConcurrentCallsPublishAtMostOnce(t *testing.T) {
	sessionsRoot, outputRoot, _ := makeRetrySession(t, 3)
	const callers = 2
	var wg sync.WaitGroup
	errs := make(chan error, callers)
	paths := make(chan string, callers)
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			path, err := RetryLocalFromSession(context.Background(), sessionsRoot, outputRoot, retryFixtureSessionID, 3)
			if err != nil {
				errs <- err
				return
			}
			paths <- path
		}()
	}
	wg.Wait()
	close(errs)
	close(paths)
	successes, failures := 0, 0
	for path := range paths {
		successes++
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("published output missing: %v", err)
		}
	}
	for err := range errs {
		failures++
		if !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("unexpected competing retry error: %v", err)
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("successes=%d failures=%d, want exactly one publication", successes, failures)
	}
}

func TestRetryLocalFromSessionRejectsSymlinkedSessionDirectory(t *testing.T) {
	sessionsRoot, outputRoot, rendered := makeRetrySession(t, 3)
	sessionDir := filepath.Dir(rendered)
	backup := sessionDir + ".backup"
	if err := os.Rename(sessionDir, backup); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, sessionDir); err != nil {
		if restoreErr := os.Rename(backup, sessionDir); restoreErr != nil {
			t.Fatal(restoreErr)
		}
		t.Skipf("symlink creation is unavailable: %v", err)
	}
	defer func() {
		_ = os.Remove(sessionDir)
		_ = os.Rename(backup, sessionDir)
	}()
	if _, err := RetryLocalFromSession(context.Background(), sessionsRoot, outputRoot, retryFixtureSessionID, 3); err == nil || !strings.Contains(err.Error(), "session identity") {
		t.Fatalf("error=%v, want redirected session identity refusal", err)
	}
}
