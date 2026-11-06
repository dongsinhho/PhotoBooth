package retention

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func createOldSession(t *testing.T, root, id string, now time.Time, days int) string {
	t.Helper()
	dir := filepath.Join(root, id)
	captures := filepath.Join(dir, "captures")
	if err := os.MkdirAll(captures, 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(path string) {
		if err := os.WriteFile(path, []byte("synthetic test data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(captures, "capture-01.jpg"))
	write(filepath.Join(dir, "rendered.jpg"))
	old := now.Add(-time.Duration(days)*24*time.Hour - time.Hour)
	for _, path := range []string{filepath.Join(captures, "capture-01.jpg"), captures, filepath.Join(dir, "rendered.jpg"), dir} {
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func createOldOutput(t *testing.T, root, id string, now time.Time, days int) string {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "photo.jpg")
	if err := os.WriteFile(file, []byte("synthetic output"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := now.Add(-time.Duration(days)*24*time.Hour - time.Hour)
	if err := os.Chtimes(file, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(dir, old, old); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestBuildPlanIsDryRunAndApplyRemovesOnlyExpiredKnownLayouts(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	base := t.TempDir()
	sessions := filepath.Join(base, "sessions")
	outputs := filepath.Join(base, "output")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outputs, 0o700); err != nil {
		t.Fatal(err)
	}
	oldSession := createOldSession(t, sessions, "0123456789abcdef0123456789abcdef", now, 40)
	oldOutput := createOldOutput(t, outputs, "abcdef0123456789abcdef0123456789", now, 40)
	newSession := createOldSession(t, sessions, "11111111111111111111111111111111", now, 2)
	newOutput := createOldOutput(t, outputs, "22222222222222222222222222222222", now, 2)

	plan, err := BuildPlan(context.Background(), Options{
		SessionsDir: sessions, OutputDir: outputs,
		Policy: Policy{SessionRetentionDays: 30, OutputRetentionDays: 30}, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Candidates) != 2 {
		t.Fatalf("plan candidates=%d, want 2: %+v", len(plan.Candidates), plan.Candidates)
	}
	if _, err := os.Stat(oldSession); err != nil {
		t.Fatalf("dry run changed expired session: %v", err)
	}
	if _, err := os.Stat(oldOutput); err != nil {
		t.Fatalf("dry run changed expired output: %v", err)
	}

	result, err := Apply(context.Background(), plan, digestPlanForTest(t, plan))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(result.Removed) != 2 {
		t.Fatalf("removed=%d, want 2", len(result.Removed))
	}
	for _, path := range []string{oldSession, oldOutput} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expired candidate remains %q: %v", path, err)
		}
	}
	for _, path := range []string{newSession, newOutput} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("recent data was removed %q: %v", path, err)
		}
	}
}

func TestBuildPlanSkipsUnknownAndSymlinkedContent(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	base := t.TempDir()
	sessions, outputs := filepath.Join(base, "sessions"), filepath.Join(base, "output")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outputs, 0o700); err != nil {
		t.Fatal(err)
	}
	unknown := createOldSession(t, sessions, "33333333333333333333333333333333", now, 60)
	if err := os.WriteFile(filepath.Join(unknown, "operator-notes.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(base, "outside.jpg")
	if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	linkID := "44444444444444444444444444444444"
	if err := os.Symlink(outside, filepath.Join(sessions, linkID)); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	_ = createOldOutput(t, outputs, "55555555555555555555555555555555", now, 60)

	plan, err := BuildPlan(context.Background(), Options{
		SessionsDir: sessions, OutputDir: outputs,
		Policy: Policy{SessionRetentionDays: 30, OutputRetentionDays: 30}, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Candidates) != 1 || plan.Candidates[0].Kind != KindOutput {
		t.Fatalf("unexpected cleanup candidates (unknown session should be skipped): %+v skipped=%d", plan.Candidates, plan.SkippedCount)
	}
	if _, err := os.Stat(unknown); err != nil {
		t.Fatalf("unknown session removed during planning: %v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("symlink target changed: %v", err)
	}
}

func TestApplyAbortsIfExpiredSetChangesAfterPlan(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	base := t.TempDir()
	sessions, outputs := filepath.Join(base, "sessions"), filepath.Join(base, "output")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outputs, 0o700); err != nil {
		t.Fatal(err)
	}
	first := createOldSession(t, sessions, "66666666666666666666666666666666", now, 60)
	plan, err := BuildPlan(context.Background(), Options{SessionsDir: sessions, OutputDir: outputs,
		Policy: Policy{SessionRetentionDays: 30}, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	second := createOldSession(t, sessions, "77777777777777777777777777777777", now, 60)
	if _, err := Apply(context.Background(), plan, digestPlanForTest(t, plan)); err == nil || !strings.Contains(err.Error(), "changed since planning") {
		t.Fatalf("Apply error=%v, want changed-set refusal", err)
	}
	for _, path := range []string{first, second} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("data removed after plan mismatch %q: %v", path, err)
		}
	}
}

func TestBuildPlanRejectsOverlappingRootsAndInvalidPolicy(t *testing.T) {
	base := t.TempDir()
	tests := []struct {
		name    string
		options Options
		want    string
	}{
		{"overlapping", Options{SessionsDir: filepath.Join(base, "data"), OutputDir: filepath.Join(base, "data", "output"), Policy: Policy{SessionRetentionDays: 10, OutputRetentionDays: 10}}, "non-overlapping roots"},
		{"negative", Options{SessionsDir: filepath.Join(base, "sessions"), OutputDir: filepath.Join(base, "output"), Policy: Policy{SessionRetentionDays: -1}}, "retention days"},
		{"excessive", Options{SessionsDir: filepath.Join(base, "sessions"), OutputDir: filepath.Join(base, "output"), Policy: Policy{OutputRetentionDays: 3651}}, "retention days"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := BuildPlan(context.Background(), tt.options); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("BuildPlan error=%v, want %q", err, tt.want)
			}
		})
	}
}

func TestBuildPlanDefaultsToDisabledAndHonorsCancellation(t *testing.T) {
	base := t.TempDir()
	opts := Options{SessionsDir: filepath.Join(base, "sessions"), OutputDir: filepath.Join(base, "output"), Now: time.Now()}
	plan, err := BuildPlan(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Disabled || len(plan.Candidates) != 0 {
		t.Fatalf("default plan must be disabled: %+v", plan)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := BuildPlan(ctx, opts); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled BuildPlan error=%v", err)
	}
	if _, err := Apply(ctx, plan, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Apply error=%v", err)
	}
}
func TestPlanDigestIsStableAndChangesWhenCandidateSetChanges(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	base := t.TempDir()
	sessions, outputs := filepath.Join(base, "sessions"), filepath.Join(base, "output")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outputs, 0o700); err != nil {
		t.Fatal(err)
	}
	first := createOldOutput(t, outputs, "88888888888888888888888888888888", now, 40)
	options := Options{SessionsDir: sessions, OutputDir: outputs, Policy: Policy{OutputRetentionDays: 30}, Now: now}
	plan1, err := BuildPlan(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	digest1, err := PlanDigest(plan1)
	if err != nil {
		t.Fatal(err)
	}
	options.Now = now.Add(10 * time.Second)
	plan2, err := BuildPlan(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	digest2, err := PlanDigest(plan2)
	if err != nil {
		t.Fatal(err)
	}
	if digest1 != digest2 {
		t.Fatalf("digest should ignore plan creation time: %s != %s", digest1, digest2)
	}

	second := createOldOutput(t, outputs, "99999999999999999999999999999999", now, 40)
	plan3, err := BuildPlan(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	digest3, err := PlanDigest(plan3)
	if err != nil {
		t.Fatal(err)
	}
	if digest3 == digest1 {
		t.Fatalf("digest did not change after candidate set changed: %s", digest3)
	}
	for _, path := range []string{first, second} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("digest test unexpectedly changed data %q: %v", path, err)
		}
	}
}
func TestBuildPlanSkipsIncompleteSessionAndTemporaryOutput(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	base := t.TempDir()
	sessions, outputs := filepath.Join(base, "sessions"), filepath.Join(base, "output")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outputs, 0o700); err != nil {
		t.Fatal(err)
	}

	incomplete := filepath.Join(sessions, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err := os.MkdirAll(filepath.Join(incomplete, "captures"), 0o700); err != nil {
		t.Fatal(err)
	}
	capture := filepath.Join(incomplete, "captures", "capture-01.jpg")
	if err := os.WriteFile(capture, []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := now.Add(-60 * 24 * time.Hour)
	_ = os.Chtimes(capture, old, old)
	_ = os.Chtimes(filepath.Join(incomplete, "captures"), old, old)
	_ = os.Chtimes(incomplete, old, old)

	tempOutput := filepath.Join(outputs, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err := os.MkdirAll(tempOutput, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"photo.jpg": "photo", ".photo-stale.tmp": "partial"} {
		if err := os.WriteFile(filepath.Join(tempOutput, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	_ = os.Chtimes(filepath.Join(tempOutput, "photo.jpg"), old, old)
	_ = os.Chtimes(filepath.Join(tempOutput, ".photo-stale.tmp"), old, old)
	_ = os.Chtimes(tempOutput, old, old)

	plan, err := BuildPlan(context.Background(), Options{
		SessionsDir: sessions, OutputDir: outputs,
		Policy: Policy{SessionRetentionDays: 30, OutputRetentionDays: 30}, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Candidates) != 0 {
		t.Fatalf("incomplete session/temporary output must not be cleanup candidates: %+v", plan.Candidates)
	}
	if plan.SkippedCount != 2 {
		t.Fatalf("skipped=%d, want 2", plan.SkippedCount)
	}
	if _, err := os.Stat(incomplete); err != nil {
		t.Fatalf("incomplete session changed: %v", err)
	}
	if _, err := os.Stat(tempOutput); err != nil {
		t.Fatalf("temporary output changed: %v", err)
	}
}

func TestApplyAbortsIfCandidateBecomesRecentAfterDryRun(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	base := t.TempDir()
	sessions, outputs := filepath.Join(base, "sessions"), filepath.Join(base, "output")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outputs, 0o700); err != nil {
		t.Fatal(err)
	}
	dir := createOldOutput(t, outputs, "cccccccccccccccccccccccccccccccc", now, 60)
	plan, err := BuildPlan(context.Background(), Options{SessionsDir: sessions, OutputDir: outputs,
		Policy: Policy{OutputRetentionDays: 30}, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "photo.jpg")
	recent := now.Add(-time.Hour)
	if err := os.Chtimes(file, recent, recent); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(dir, recent, recent); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(context.Background(), plan, digestPlanForTest(t, plan)); err == nil {
		t.Fatal("Apply should refuse a candidate that became recent after planning")
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("recent file was deleted: %v", err)
	}
}
func TestBuildPlanRejectsSymlinkedRootParent(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "real")
	if err := os.MkdirAll(filepath.Join(target, "sessions"), 0o700); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(base, "output")
	if err := os.MkdirAll(output, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "linked")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable on this platform/account: %v", err)
	}
	_, err := BuildPlan(context.Background(), Options{
		SessionsDir: filepath.Join(link, "sessions"), OutputDir: output,
		Policy: Policy{SessionRetentionDays: 30}, Now: time.Now(),
	})
	if err == nil || !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("BuildPlan error=%v, want symlinked root ancestor rejection", err)
	}
}
func digestPlanForTest(t *testing.T, plan Plan) string {
	t.Helper()
	digest, err := PlanDigest(plan)
	if err != nil {
		t.Fatalf("PlanDigest: %v", err)
	}
	return digest
}

func TestApplyRequiresMatchingConfirmationDigest(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	base := t.TempDir()
	sessions, outputs := filepath.Join(base, "sessions"), filepath.Join(base, "output")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outputs, 0o700); err != nil {
		t.Fatal(err)
	}
	dir := createOldOutput(t, outputs, "dddddddddddddddddddddddddddddddd", now, 40)
	plan, err := BuildPlan(context.Background(), Options{SessionsDir: sessions, OutputDir: outputs,
		Policy: Policy{OutputRetentionDays: 30}, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	for _, confirmation := range []string{"", strings.Repeat("0", 64)} {
		if _, err := Apply(context.Background(), plan, confirmation); err == nil || !strings.Contains(err.Error(), "confirmation digest") {
			t.Fatalf("Apply confirmation=%q error=%v, want confirmation guard", confirmation, err)
		}
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("rejected Apply changed data: %v", err)
		}
	}
	if _, err := Apply(context.Background(), plan, digestPlanForTest(t, plan)); err != nil {
		t.Fatalf("Apply with matching digest: %v", err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("confirmed candidate remains: %v", err)
	}
}

func TestBuildPlanRejectsOverlappingRootsEvenWhenOnePolicyIsDisabled(t *testing.T) {
	root := t.TempDir()
	sessions := filepath.Join(root, "sessions")
	outputs := filepath.Join(sessions, "photos")
	if err := os.MkdirAll(outputs, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := BuildPlan(context.Background(), Options{
		SessionsDir: sessions,
		OutputDir:   outputs,
		Policy:      Policy{SessionRetentionDays: 30, OutputRetentionDays: 0},
		Now:         time.Now(),
	})
	if err == nil || !strings.Contains(err.Error(), "non-overlapping roots") {
		t.Fatalf("BuildPlan error=%v, want root overlap rejection", err)
	}
}
