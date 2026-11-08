package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"time"

	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
	"github.com/dongsinhho/PhotoBooth/internal/booth/session"
	"github.com/dongsinhho/PhotoBooth/internal/booth/workflow"
)

type fixtureReport struct {
	ContinueID       string            `json:"continue_id"`
	OutputPendingID  string            `json:"output_pending_id"`
	RestartInspectID string            `json:"restart_inspect_id"`
	HideInspectID    string            `json:"hide_inspect_id"`
	ExportInspectID  string            `json:"export_inspect_id"`
	OriginalHashes   map[string]string `json:"original_hashes"`
}

func main() {
	dataDir := flag.String("data-dir", "", "isolated PhotoBooth data directory")
	flag.Parse()
	if *dataDir == "" {
		fail("data-dir is required")
	}
	absoluteDataDir, err := filepath.Abs(*dataDir)
	if err != nil {
		fail("resolve data dir: %v", err)
	}
	configPath := filepath.Join(absoluteDataDir, "store-config-v1.json")
	cfg, err := stationconfig.Load(configPath)
	if err != nil {
		fail("load isolated station config: %v", err)
	}
	countdown, err := cfg.CountdownDuration()
	if err != nil {
		fail("parse countdown: %v", err)
	}
	interval, err := cfg.IntervalDuration()
	if err != nil {
		fail("parse interval: %v", err)
	}
	if cfg.Capture.Shots != 3 || countdown != 0 || interval != 0 {
		fail("fixture profile must use 3 shots and zero countdown/interval")
	}
	if cfg.Camera.Provider != "windowsmf" {
		fail("UI profile must retain windowsmf as configured camera provider; preview-off switch must be the only camera guard")
	}

	framePath, err := filepath.Abs(cfg.Frame.ConfigPath)
	if err != nil {
		fail("resolve frame path: %v", err)
	}
	run := workflow.Config{
		FrameConfigPath:     framePath,
		WorkDir:             cfg.Storage.SessionsDir,
		OutputDir:           cfg.Storage.OutputDir,
		CaptureCount:        cfg.Capture.Shots,
		Countdown:           countdown,
		Interval:            interval,
		CaptureWidth:        cfg.Camera.Width,
		CaptureHeight:       cfg.Camera.Height,
		ConfigSchemaVersion: cfg.SchemaVersion,
		CameraProvider:      "fake",
		CameraDevice:        "",
		CameraFPS:           0,
		DemoMode:            true,
	}

	report := fixtureReport{
		ContinueID:       "a1111111111111111111111111111111",
		OutputPendingID:  "b2222222222222222222222222222222",
		RestartInspectID: "c3333333333333333333333333333333",
		HideInspectID:    "d4444444444444444444444444444444",
		ExportInspectID:  "e5555555555555555555555555555555",
		OriginalHashes:   make(map[string]string),
	}
	seedSession(cfg.Storage.SessionsDir, run, report.ContinueID, session.PhaseCapturing, 1, false, report.OriginalHashes)
	seedSession(cfg.Storage.SessionsDir, run, report.OutputPendingID, session.PhaseRendered, 3, false, report.OriginalHashes)
	seedSession(cfg.Storage.SessionsDir, run, report.RestartInspectID, session.PhaseCapturing, 1, true, report.OriginalHashes)
	seedSession(cfg.Storage.SessionsDir, run, report.HideInspectID, session.PhaseCapturing, 1, true, report.OriginalHashes)
	seedSession(cfg.Storage.SessionsDir, run, report.ExportInspectID, session.PhaseCapturing, 1, true, report.OriginalHashes)

	outputPath := filepath.Join(absoluteDataDir, "recovery-smoke-fixtures.json")
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fail("encode fixture report: %v", err)
	}
	if err := os.WriteFile(outputPath, data, 0o600); err != nil {
		fail("write fixture report: %v", err)
	}
	fmt.Printf("RECOVERY_FIXTURES_READY=%s\n", outputPath)
	fmt.Printf("CONTINUE_ID=%s\nOUTPUT_PENDING_ID=%s\nRESTART_INSPECT_ID=%s\nHIDE_INSPECT_ID=%s\nEXPORT_INSPECT_ID=%s\n",
		report.ContinueID, report.OutputPendingID, report.RestartInspectID, report.HideInspectID, report.ExportInspectID)
}

func seedSession(root string, base workflow.Config, id string, phase session.Phase, completed int, unknownEntry bool, hashes map[string]string) {
	directory := filepath.Join(root, id)
	captures := filepath.Join(directory, "captures")
	if err := os.MkdirAll(captures, 0o700); err != nil {
		fail("create session %s: %v", id, err)
	}
	for shot := 1; shot <= completed; shot++ {
		path := filepath.Join(captures, fmt.Sprintf("capture-%02d.jpg", shot))
		writeJPEG(path, byte(20+shot*20))
		hashFile(path, hashes)
	}
	if phase == session.PhaseRendered || phase == session.PhaseOutputFinalized {
		rendered := filepath.Join(directory, "rendered.jpg")
		writeJPEG(rendered, 180)
		hashFile(rendered, hashes)
	}
	if unknownEntry {
		note := filepath.Join(directory, "operator-note.txt")
		if err := os.WriteFile(note, []byte("preserve this unknown entry during restart"), 0o600); err != nil {
			fail("write inspect-only marker: %v", err)
		}
		hashFile(note, hashes)
	}

	run := base
	run.SessionID = id
	frameBytes, err := os.ReadFile(run.FrameConfigPath)
	if err != nil {
		fail("read frame snapshot source for %s: %v", id, err)
	}
	manifest, err := workflow.NewSessionManifestFromFrameBytes(run, frameBytes)
	if err != nil {
		fail("create manifest %s: %v", id, err)
	}
	_, snapshotHash, err := session.WriteFrameSnapshot(directory, frameBytes)
	if err != nil {
		fail("write frame snapshot for %s: %v", id, err)
	}
	manifest.FrameSnapshotFile = session.FrameSnapshotFileName
	manifest.FrameSnapshotSHA256 = snapshotHash
	hashFile(filepath.Join(directory, session.FrameSnapshotFileName), hashes)
	manifest.Phase = phase
	manifest.CompletedShots = completed
	manifest.UpdatedAt = time.Now().UTC()
	if phase == session.PhaseRendered || phase == session.PhaseOutputFinalized {
		manifest.RenderedAt = manifest.UpdatedAt
	}
	if phase == session.PhaseOutputFinalized {
		manifest.OutputPath = filepath.Join(base.OutputDir, id, "photo.jpg")
		manifest.OutputFinalizedAt = manifest.UpdatedAt
	}
	if err := session.WriteManifest(directory, manifest); err != nil {
		fail("write manifest %s: %v", id, err)
	}
	hashFile(filepath.Join(directory, session.ManifestFileName), hashes)
}

func writeJPEG(path string, seed byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		fail("create JPEG directory: %v", err)
	}
	file, err := os.Create(path)
	if err != nil {
		fail("create JPEG %s: %v", path, err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			value := uint8(int(seed) + x + y)
			img.Set(x, y, color.RGBA{R: value, G: uint8(80 + y), B: uint8(120 + x), A: 255})
		}
	}
	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: 90}); err != nil {
		_ = file.Close()
		fail("encode JPEG %s: %v", path, err)
	}
	if err := file.Close(); err != nil {
		fail("close JPEG %s: %v", path, err)
	}
}

func hashFile(path string, hashes map[string]string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fail("read fixture for hash %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	hashes[path] = hex.EncodeToString(sum[:])
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
