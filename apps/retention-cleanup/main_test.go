package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func createExpiredOutput(t *testing.T, root, id string) string {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "photo.jpg")
	if err := os.WriteFile(file, []byte("synthetic retention fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-40 * 24 * time.Hour)
	if err := os.Chtimes(file, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(dir, old, old); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRunDefaultsToDryRunAndDoesNotDelete(t *testing.T) {
	base := t.TempDir()
	sessions, outputs := filepath.Join(base, "sessions"), filepath.Join(base, "output")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outputs, 0o700); err != nil {
		t.Fatal(err)
	}
	dir := createExpiredOutput(t, outputs, "0123456789abcdef0123456789abcdef")
	var stdout, stderr bytes.Buffer
	err := run([]string{"--sessions-dir", sessions, "--output-dir", outputs, "--output-days", "30"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("dry-run: %v stderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "MODE=DRY-RUN") || !strings.Contains(stdout.String(), "CANDIDATES=1") {
		t.Fatalf("dry-run report missing mode/candidate: %s", stdout.String())
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("dry-run removed data: %v", err)
	}
}

func planDigestFromOutput(t *testing.T, output string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "PLAN_DIGEST=") {
			digest := strings.TrimSpace(strings.TrimPrefix(line, "PLAN_DIGEST="))
			if len(digest) == 64 {
				return digest
			}
			t.Fatalf("invalid plan digest %q in output", digest)
		}
	}
	t.Fatalf("PLAN_DIGEST not found in output: %s", output)
	return ""
}

func TestRunApplyRequiresReviewedDigestAndDeletesOnlyConfirmedFixture(t *testing.T) {
	base := t.TempDir()
	sessions, outputs := filepath.Join(base, "sessions"), filepath.Join(base, "output")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outputs, 0o700); err != nil {
		t.Fatal(err)
	}
	dir := createExpiredOutput(t, outputs, "abcdef0123456789abcdef0123456789")
	args := []string{"--sessions-dir", sessions, "--output-dir", outputs, "--output-days", "30"}
	var stdout, stderr bytes.Buffer
	if err := run(args, &stdout, &stderr); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	digest := planDigestFromOutput(t, stdout.String())
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("dry-run removed data: %v", err)
	}

	stdout.Reset()
	if err := run(append(append([]string{}, args...), "--apply"), &stdout, &stderr); err == nil ||
		!strings.Contains(err.Error(), "--confirm-plan") {
		t.Fatalf("apply without confirmation error=%v, want --confirm-plan requirement", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("unconfirmed apply changed data: %v", err)
	}

	stdout.Reset()
	wrongArgs := append(append([]string{}, args...), "--apply", "--confirm-plan", strings.Repeat("0", 64))
	if err := run(wrongArgs, &stdout, &stderr); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("apply with stale/wrong digest error=%v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("wrong digest deleted data: %v", err)
	}

	stdout.Reset()
	confirmedArgs := append(append([]string{}, args...), "--apply", "--confirm-plan", digest)
	if err := run(confirmedArgs, &stdout, &stderr); err != nil {
		t.Fatalf("confirmed apply: %v stderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "MODE=APPLY_REQUESTED") || !strings.Contains(stdout.String(), "REMOVED_CANDIDATES=1") {
		t.Fatalf("apply report missing confirmation/result: %s", stdout.String())
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expired fixture remains or stat failed: %v", err)
	}
}

func TestRunRejectsStalePlanWhenCandidateSetChanges(t *testing.T) {
	base := t.TempDir()
	sessions, outputs := filepath.Join(base, "sessions"), filepath.Join(base, "output")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outputs, 0o700); err != nil {
		t.Fatal(err)
	}
	first := createExpiredOutput(t, outputs, "11111111111111111111111111111111")
	args := []string{"--sessions-dir", sessions, "--output-dir", outputs, "--output-days", "30"}
	var stdout, stderr bytes.Buffer
	if err := run(args, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	digest := planDigestFromOutput(t, stdout.String())
	second := createExpiredOutput(t, outputs, "22222222222222222222222222222222")
	stdout.Reset()
	applyArgs := append(append([]string{}, args...), "--apply", "--confirm-plan", digest)
	if err := run(applyArgs, &stdout, &stderr); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("stale-plan apply error=%v, want digest mismatch", err)
	}
	for _, path := range []string{first, second} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("stale-plan apply removed data %q: %v", path, err)
		}
	}
}

func TestRunRejectsInvalidArgumentsAndKeepsCleanupOptIn(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"--help"}, &stdout, &stderr); err != nil {
		t.Fatalf("help should exit normally: %v", err)
	}
	if !strings.Contains(stderr.String(), "confirm-plan") {
		t.Fatalf("help should document confirmation flag: %s", stderr.String())
	}
	stderr.Reset()
	for _, args := range [][]string{{"--session-days", "-2"}, {"unexpected-positional"}} {
		if err := run(args, &stdout, &stderr); err == nil {
			t.Fatalf("run(%v) unexpectedly succeeded", args)
		}
	}
	if err := run([]string{"--session-days", "0", "--output-days", "0"}, &stdout, &stderr); err != nil {
		t.Fatalf("disabled policy should succeed: %v", err)
	}
	if !strings.Contains(stdout.String(), "Retention is disabled") {
		t.Fatalf("disabled policy not reported: %s", stdout.String())
	}
}
