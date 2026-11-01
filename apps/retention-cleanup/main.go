package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
	"github.com/dongsinhho/PhotoBooth/internal/booth/retention"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "retention-cleanup:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("retention-cleanup", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "optional local station config JSON")
	sessionsDir := flags.String("sessions-dir", "", "override local sessions directory")
	outputDir := flags.String("output-dir", "", "override finalized local output directory")
	sessionDays := flags.Int("session-days", -1, "override session retention days (0 disables)")
	outputDays := flags.Int("output-days", -1, "override finalized output retention days (0 disables)")
	apply := flags.Bool("apply", false, "request deletion; also requires --confirm-plan from a reviewed dry-run")
	confirmPlan := flags.String("confirm-plan", "", "64-hex PLAN_DIGEST copied from a reviewed dry-run")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}
	if strings.TrimSpace(*confirmPlan) != "" && !*apply {
		return errors.New("--confirm-plan requires --apply")
	}
	if *sessionDays < -1 || *outputDays < -1 {
		return errors.New("retention overrides must be -1 (use config) or 0-3650")
	}
	if *apply {
		fmt.Fprintln(stdout, "MODE=APPLY_REQUESTED (deletion requires a matching --confirm-plan token from a reviewed dry-run)")
	} else {
		fmt.Fprintln(stdout, "MODE=DRY-RUN (read-only; no data is changed)")
	}

	cfg := stationconfig.Defaults()
	if strings.TrimSpace(*configPath) != "" {
		loaded, err := stationconfig.Load(*configPath)
		if err != nil {
			return err
		}
		cfg = loaded
	}
	if strings.TrimSpace(*sessionsDir) != "" {
		cfg.Storage.SessionsDir = *sessionsDir
	}
	if strings.TrimSpace(*outputDir) != "" {
		cfg.Storage.OutputDir = *outputDir
	}
	if *sessionDays >= 0 {
		cfg.Storage.SessionRetentionDays = *sessionDays
	}
	if *outputDays >= 0 {
		cfg.Storage.OutputRetentionDays = *outputDays
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	plan, err := retention.BuildPlan(context.Background(), retention.Options{
		SessionsDir: cfg.Storage.SessionsDir,
		OutputDir:   cfg.Storage.OutputDir,
		Policy: retention.Policy{
			SessionRetentionDays: cfg.Storage.SessionRetentionDays,
			OutputRetentionDays:  cfg.Storage.OutputRetentionDays,
		},
		Now: time.Now(),
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "SESSION_RETENTION_DAYS=%d OUTPUT_RETENTION_DAYS=%d\n", cfg.Storage.SessionRetentionDays, cfg.Storage.OutputRetentionDays)
	fmt.Fprintf(stdout, "SCANNED=%d SKIPPED=%d CANDIDATES=%d\n", plan.ScannedCount, plan.SkippedCount, len(plan.Candidates))
	digest, err := retention.PlanDigest(plan)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "PLAN_DIGEST=%s\n", digest)
	if plan.Disabled {
		fmt.Fprintln(stdout, "Retention is disabled; no files were changed.")
		return nil
	}
	if len(plan.Candidates) == 0 {
		fmt.Fprintln(stdout, "No expired PhotoBooth data matched the enabled policies.")
		return nil
	}
	var totalBytes int64
	for _, item := range plan.Candidates {
		totalBytes += item.SizeBytes
		fmt.Fprintf(stdout, "- kind=%s session=%s modified=%s files=%d size_bytes=%d path=%s\n",
			item.Kind, item.SessionID, item.LastModified.Format(time.RFC3339), item.FileCount, item.SizeBytes, item.Path)
	}
	fmt.Fprintf(stdout, "TOTAL_EXPIRED_FILES=%d TOTAL_EXPIRED_BYTES=%d\n", countFiles(plan.Candidates), totalBytes)
	if !*apply {
		fmt.Fprintf(stdout, "Dry-run complete. Review every listed path, then rerun the same command with --apply --confirm-plan %s.\n", digest)
		return nil
	}
	if strings.TrimSpace(*confirmPlan) == "" {
		return errors.New("--apply requires --confirm-plan <digest> copied from a reviewed dry-run; no data was changed")
	}
	if !strings.EqualFold(strings.TrimSpace(*confirmPlan), digest) {
		return errors.New("confirmation digest does not match the current candidate set; no data was changed; rerun a dry-run and review the new plan")
	}
	result, err := retention.Apply(context.Background(), plan, digest)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "REMOVED_CANDIDATES=%d\n", len(result.Removed))
	return nil
}

func countFiles(items []retention.Candidate) int {
	total := 0
	for _, item := range items {
		total += item.FileCount
	}
	return total
}
