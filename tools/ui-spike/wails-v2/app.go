package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx           context.Context
	startedAt     time.Time
	benchmarkMode bool
	reportPath    string

	mu      sync.Mutex
	session DemoSession
}

type DemoSession struct {
	ID       string `json:"id"`
	State    string `json:"state"`
	Captured int    `json:"captured"`
	Total    int    `json:"total"`
}

type BenchmarkReport struct {
	CreatedAt         time.Time `json:"created_at"`
	GoVersion         string    `json:"go_version"`
	OS                string    `json:"os"`
	Architecture      string    `json:"architecture"`
	SampleCount       int       `json:"sample_count"`
	StartupToReadyMS  float64   `json:"startup_to_ready_ms"`
	MeanRoundTripMS   float64   `json:"mean_round_trip_ms"`
	P50RoundTripMS    float64   `json:"p50_round_trip_ms"`
	P95RoundTripMS    float64   `json:"p95_round_trip_ms"`
	WorkingSetNote    string    `json:"working_set_note"`
	BenchmarkModeNote string    `json:"benchmark_mode_note"`
}

func NewApp() *App {
	return &App{session: DemoSession{State: "idle", Total: 3}}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.startedAt = time.Now()
	a.benchmarkMode = slices.Contains(os.Args, "--benchmark")
	a.reportPath = os.Getenv("PHOTOBOOTH_UI_SPIKE_REPORT")
	if a.reportPath == "" {
		a.reportPath = filepath.Join(os.TempDir(), "photobooth-kiosk-ui-spike-report.json")
	}
}

func (a *App) IsBenchmarkMode() bool { return a.benchmarkMode }

// Ready is called from the WebView after the UI bundle has loaded.
func (a *App) Ready() map[string]any {
	startupMS := float64(0)
	if !a.startedAt.IsZero() {
		startupMS = float64(time.Since(a.startedAt).Microseconds()) / 1000
	}
	return map[string]any{
		"startup_to_ready_ms": startupMS,
		"go_version":          runtime.Version(),
		"os":                  runtime.GOOS,
		"architecture":        runtime.GOARCH,
		"benchmark_mode":      a.benchmarkMode,
	}
}

// Ping measures a minimal UI-to-Go method dispatch. The browser measures the
// full round trip; this method reports the time spent inside the Go handler.
func (a *App) Ping(sequence int) int {
	return sequence
}

func (a *App) StartDemoSession() DemoSession {
	a.mu.Lock()
	defer a.mu.Unlock()
	id := make([]byte, 12)
	if _, err := rand.Read(id); err != nil {
		id = []byte(fmt.Sprintf("%d", time.Now().UnixNano()))
	}
	a.session = DemoSession{ID: hex.EncodeToString(id), State: "capturing", Total: 3}
	return a.session
}

func (a *App) RecordDemoShot(sessionID string) (DemoSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if sessionID == "" || sessionID != a.session.ID {
		return a.session, errors.New("session ID does not match the active demo session")
	}
	if a.session.State != "capturing" {
		return a.session, fmt.Errorf("cannot capture a shot while session state is %q", a.session.State)
	}
	if a.session.Captured >= a.session.Total {
		return a.session, errors.New("all demo shots have already been captured")
	}
	a.session.Captured++
	if a.session.Captured == a.session.Total {
		a.session.State = "ready_to_finish"
	}
	return a.session, nil
}

func (a *App) FinishDemoSession(sessionID string) (DemoSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if sessionID == "" || sessionID != a.session.ID {
		return a.session, errors.New("session ID does not match the active demo session")
	}
	if a.session.State != "ready_to_finish" || a.session.Captured != a.session.Total {
		return a.session, errors.New("cannot finish before all demo shots are captured")
	}
	a.session.State = "completed"
	return a.session, nil
}

func (a *App) ResetDemoSession() DemoSession {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.session = DemoSession{State: "idle", Total: 3}
	return a.session
}

// CompleteBenchmark persists measurements supplied by the frontend so the
// PowerShell harness can collect process working-set measurements separately.
func (a *App) CompleteBenchmark(roundTripsMS []float64, startupToReadyMS float64) error {
	if !a.benchmarkMode {
		return errors.New("benchmark report is only enabled in --benchmark mode")
	}
	if len(roundTripsMS) == 0 {
		return errors.New("round-trip measurements must not be empty")
	}
	roundTrips := append([]float64(nil), roundTripsMS...)
	sort.Float64s(roundTrips)
	report := BenchmarkReport{
		CreatedAt:         time.Now(),
		GoVersion:         runtime.Version(),
		OS:                runtime.GOOS,
		Architecture:      runtime.GOARCH,
		SampleCount:       len(roundTrips),
		StartupToReadyMS:  startupToReadyMS,
		MeanRoundTripMS:   mean(roundTrips),
		P50RoundTripMS:    percentile(roundTrips, 0.50),
		P95RoundTripMS:    percentile(roundTrips, 0.95),
		WorkingSetNote:    "Captured by the external PowerShell harness; Go bridge report does not measure process RAM.",
		BenchmarkModeNote: "Hidden Wails window; this measures WebView-to-Go binding overhead and startup, not camera capture/render or visible UI input latency.",
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(a.reportPath, data, 0o600); err != nil {
		return fmt.Errorf("write benchmark report: %w", err)
	}
	time.AfterFunc(500*time.Millisecond, func() {
		wailsruntime.Quit(a.ctx)
	})
	return nil
}

func mean(values []float64) float64 {
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func percentile(sortedValues []float64, p float64) float64 {
	if len(sortedValues) == 1 {
		return sortedValues[0]
	}
	index := int(float64(len(sortedValues)-1) * p)
	return sortedValues[index]
}
