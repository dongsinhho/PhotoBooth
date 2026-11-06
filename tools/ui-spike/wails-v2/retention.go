package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
	"github.com/dongsinhho/PhotoBooth/internal/booth/retention"
)

const retentionPreviewLimit = 100

type RetentionCandidateView struct {
	Kind         string `json:"kind"`
	SessionID    string `json:"session_id"`
	Path         string `json:"path"`
	LastModified string `json:"last_modified"`
	FileCount    int    `json:"file_count"`
	SizeBytes    int64  `json:"size_bytes"`
}

type RetentionPreview struct {
	GeneratedAt          string                   `json:"generated_at"`
	Disabled             bool                     `json:"disabled"`
	SessionRetentionDays int                      `json:"session_retention_days"`
	OutputRetentionDays  int                      `json:"output_retention_days"`
	ScannedCount         int                      `json:"scanned_count"`
	SkippedCount         int                      `json:"skipped_count"`
	CandidateCount       int                      `json:"candidate_count"`
	TotalFiles           int                      `json:"total_files"`
	TotalBytes           int64                    `json:"total_bytes"`
	PlanDigest           string                   `json:"plan_digest"`
	Truncated            bool                     `json:"truncated"`
	Candidates           []RetentionCandidateView `json:"candidates"`
}

type RetentionApplyResult struct {
	RemovedCandidates int   `json:"removed_candidates"`
	RemovedFiles      int   `json:"removed_files"`
	RemovedBytes      int64 `json:"removed_bytes"`
}

func retentionOptions(cfg stationconfig.StoreConfig) retention.Options {
	return retention.Options{
		SessionsDir: cfg.Storage.SessionsDir,
		OutputDir:   cfg.Storage.OutputDir,
		Policy: retention.Policy{
			SessionRetentionDays: cfg.Storage.SessionRetentionDays,
			OutputRetentionDays:  cfg.Storage.OutputRetentionDays,
		},
		Now: time.Now(),
	}
}

func sameRetentionOptions(left, right retention.Options) bool {
	return filepath.Clean(left.SessionsDir) == filepath.Clean(right.SessionsDir) &&
		filepath.Clean(left.OutputDir) == filepath.Clean(right.OutputDir) &&
		left.Policy == right.Policy
}

// PreviewRetentionCleanup is read-only. Its digest is an in-memory, one-time
// confirmation token for the exact local roots, policy and candidate metadata.
func (a *App) PreviewRetentionCleanup() (RetentionPreview, error) {
	if err := a.preparePlayerPaths(); err != nil {
		return RetentionPreview{}, err
	}

	a.mu.Lock()
	if a.settingsError != "" {
		message := a.settingsError
		a.mu.Unlock()
		return RetentionPreview{}, fmt.Errorf("station settings need attention before retention preview: %s", message)
	}
	if a.playerRunning || a.outputRetrying || a.settingsSaving || a.retentionBusy {
		a.mu.Unlock()
		return RetentionPreview{}, errors.New("retention preview is unavailable while a session, output retry, settings save or cleanup operation is running")
	}
	a.retentionBusy = true
	cfg := a.stationConfig
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.retentionBusy = false
		a.mu.Unlock()
	}()

	options := retentionOptions(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	plan, err := retention.BuildPlan(ctx, options)
	if err != nil {
		a.clearRetentionPlan()
		return RetentionPreview{}, fmt.Errorf("build read-only retention plan: %w", err)
	}
	digest, err := retention.PlanDigest(plan)
	if err != nil {
		a.clearRetentionPlan()
		return RetentionPreview{}, fmt.Errorf("digest retention plan: %w", err)
	}

	preview := RetentionPreview{
		GeneratedAt:          plan.GeneratedAt.UTC().Format(time.RFC3339Nano),
		Disabled:             plan.Disabled,
		SessionRetentionDays: cfg.Storage.SessionRetentionDays,
		OutputRetentionDays:  cfg.Storage.OutputRetentionDays,
		ScannedCount:         plan.ScannedCount,
		SkippedCount:         plan.SkippedCount,
		CandidateCount:       len(plan.Candidates),
		PlanDigest:           digest,
		Candidates:           make([]RetentionCandidateView, 0, min(len(plan.Candidates), retentionPreviewLimit)),
	}
	for _, item := range plan.Candidates {
		preview.TotalFiles += item.FileCount
		preview.TotalBytes += item.SizeBytes
	}
	limit := min(len(plan.Candidates), retentionPreviewLimit)
	preview.Truncated = len(plan.Candidates) > limit
	for _, item := range plan.Candidates[:limit] {
		preview.Candidates = append(preview.Candidates, RetentionCandidateView{
			Kind: item.Kind, SessionID: item.SessionID, Path: item.Path,
			LastModified: item.LastModified.UTC().Format(time.RFC3339Nano), FileCount: item.FileCount, SizeBytes: item.SizeBytes,
		})
	}

	a.mu.Lock()
	a.retentionPlan = &plan
	a.retentionPlanDigest = digest
	a.retentionPlanOptions = options
	a.mu.Unlock()
	return preview, nil
}

// ApplyRetentionCleanup can only consume the exact, latest in-memory plan.
// The core retention package re-scans and revalidates every candidate before
// removal; any settings change invalidates the cached plan.
func (a *App) ApplyRetentionCleanup(confirmationDigest string) (RetentionApplyResult, error) {
	if err := a.preparePlayerPaths(); err != nil {
		return RetentionApplyResult{}, err
	}

	a.mu.Lock()
	if a.settingsError != "" {
		message := a.settingsError
		a.mu.Unlock()
		return RetentionApplyResult{}, fmt.Errorf("station settings need attention before cleanup: %s", message)
	}
	if a.playerRunning || a.outputRetrying || a.settingsSaving || a.retentionBusy {
		a.mu.Unlock()
		return RetentionApplyResult{}, errors.New("cleanup is unavailable while a session, output retry, settings save or another cleanup operation is running")
	}
	if a.retentionPlan == nil || a.retentionPlan.Disabled || len(a.retentionPlan.Candidates) == 0 {
		a.mu.Unlock()
		return RetentionApplyResult{}, errors.New("there is no non-empty reviewed retention plan; run Preview cleanup again")
	}
	if strings.TrimSpace(confirmationDigest) == "" ||
		!strings.EqualFold(strings.TrimSpace(confirmationDigest), a.retentionPlanDigest) {
		a.mu.Unlock()
		return RetentionApplyResult{}, errors.New("confirmation does not match the latest reviewed plan; no cleanup was started")
	}
	currentOptions := retentionOptions(a.stationConfig)
	if !sameRetentionOptions(currentOptions, a.retentionPlanOptions) {
		a.retentionPlan = nil
		a.retentionPlanDigest = ""
		a.mu.Unlock()
		return RetentionApplyResult{}, errors.New("station storage paths or retention policy changed since preview; run Preview cleanup again")
	}
	plan := a.retentionPlan
	a.retentionBusy = true
	a.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, applyErr := retention.Apply(ctx, *plan, confirmationDigest)

	a.mu.Lock()
	a.retentionBusy = false
	// A plan is one-use whether Apply succeeds or fails. This prevents a stale
	// confirmation from being replayed after a partial failure or filesystem change.
	a.retentionPlan = nil
	a.retentionPlanDigest = ""
	a.mu.Unlock()

	if applyErr != nil {
		return RetentionApplyResult{}, fmt.Errorf("apply reviewed retention plan: %w", applyErr)
	}
	summary := RetentionApplyResult{RemovedCandidates: len(result.Removed)}
	for _, item := range result.Removed {
		summary.RemovedFiles += item.FileCount
		summary.RemovedBytes += item.SizeBytes
	}
	return summary, nil
}

// DiscardRetentionPreview invalidates a previously generated cleanup plan.
func (a *App) DiscardRetentionPreview() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.retentionBusy {
		return errors.New("cannot discard a retention preview while cleanup is running")
	}
	a.retentionPlan = nil
	a.retentionPlanDigest = ""
	return nil
}

func (a *App) clearRetentionPlan() {
	a.mu.Lock()
	a.retentionPlan = nil
	a.retentionPlanDigest = ""
	a.mu.Unlock()
}
