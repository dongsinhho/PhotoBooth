//go:build windows

package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dongsinhho/PhotoBooth/internal/booth/output"
	"github.com/dongsinhho/PhotoBooth/internal/booth/session"
)

type copyResult struct {
	JobID                string        `json:"job_id"`
	SessionID            string        `json:"session_id"`
	AttemptID            string        `json:"attempt_id"`
	DocumentName         string        `json:"document_name"`
	ManifestStatus       string        `json:"manifest_print_status"`
	QueueInspectionFound bool          `json:"queue_inspection_found"`
	QueueStatus          []string      `json:"queue_status,omitempty"`
	LocalOutputFile      string        `json:"local_output_file"`
	LocalSHA256          string        `json:"local_sha256"`
	Converted            string        `json:"converted_jpeg"`
	SHA256               string        `json:"converted_sha256"`
	Width                int           `json:"width"`
	Height               int           `json:"height"`
	Duration             time.Duration `json:"duration"`
}

type smokeReport struct {
	Status                       string        `json:"status"`
	Printer                      string        `json:"printer"`
	MediaForm                    string        `json:"media_form,omitempty"`
	Landscape                    bool          `json:"landscape"`
	FlipVertical                 bool          `json:"flip_vertical"`
	CopiesRequested              int           `json:"copies_requested"`
	SourceSHA256                 string        `json:"source_sha256"`
	SourceWidth                  int           `json:"source_width"`
	SourceHeight                 int           `json:"source_height"`
	Jobs                         []copyResult  `json:"jobs"`
	LocalSavedCount              int           `json:"local_saved_count"`
	DurableIntentCount           int           `json:"durable_intent_count"`
	SpoolerAcceptedCount         int           `json:"spooler_accepted_count"`
	ConversionSeenCount          int           `json:"virtual_conversion_seen_count"`
	QueueInspectionCount         int           `json:"queue_inspection_count"`
	CoordinatorIndependenceCheck bool          `json:"coordinator_independence_check"`
	PhysicalPrintConfirmed       bool          `json:"physical_print_confirmed"`
	Duration                     time.Duration `json:"duration"`
	Error                        string        `json:"error,omitempty"`
}

func main() {
	printerName := flag.String("printer", "DS-RX1", "installed Windows printer queue name")
	mediaForm := flag.String("media-form", "", "exact Windows driver DEVMODEW.dmFormName; empty keeps the queue default")
	outputDir := flag.String("output-dir", "", "Multi File Port Monitor output folder; required to verify conversion")
	landscape := flag.Bool("landscape", true, "set landscape orientation for each job only")
	flipVertical := flag.Bool("flip-vertical", true, "flip source vertically for the configured DS-RX1 virtual converter only")
	copies := flag.Int("copies", 1, "number of synthetic virtual jobs to submit sequentially (1-20)")
	timeout := flag.Duration("timeout", 45*time.Second, "maximum wait per job for the virtual port converter to create its JPEG")
	expectedWidth := flag.Int("expected-width", 1200, "expected converted JPEG width")
	expectedHeight := flag.Int("expected-height", 1800, "expected converted JPEG height")
	reportPath := flag.String("report", "", "optional JSON report path")
	flag.Parse()

	if strings.TrimSpace(*outputDir) == "" {
		fatal(errors.New("-output-dir is required; no print job was submitted"))
	}
	if *copies < 1 || *copies > 20 {
		fatal(errors.New("-copies must be between 1 and 20"))
	}
	if *timeout < time.Second || *timeout > 3*time.Minute {
		fatal(errors.New("-timeout must be between 1s and 3m"))
	}
	if *expectedWidth <= 0 || *expectedHeight <= 0 {
		fatal(errors.New("expected output dimensions must be positive"))
	}
	absoluteOutputDir, err := filepath.Abs(*outputDir)
	if err != nil {
		fatal(fmt.Errorf("resolve output directory: %w", err))
	}
	if info, err := os.Stat(absoluteOutputDir); err != nil || !info.IsDir() {
		fatal(fmt.Errorf("output directory must already exist: %s", absoluteOutputDir))
	}

	tempRoot, err := os.MkdirTemp("", "photobooth-dsrx1-virtual-smoke-*")
	if err != nil {
		fatal(fmt.Errorf("create temporary fixture directory: %w", err))
	}
	defer os.RemoveAll(tempRoot)

	sourcePath := filepath.Join(tempRoot, "synthetic-color-and-alignment.jpg")
	if err := writeSyntheticFixture(sourcePath); err != nil {
		fatal(fmt.Errorf("create synthetic print fixture: %w", err))
	}
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		fatal(fmt.Errorf("read synthetic fixture for hash: %w", err))
	}
	sourceHash := sha256.Sum256(sourceBytes)

	target, err := output.NewWindowsPrinter(*printerName, output.WindowsPrinterOptions{
		MediaForm:    *mediaForm,
		Landscape:    *landscape,
		FlipVertical: *flipVertical,
	})
	if err != nil {
		fatal(err)
	}
	sessionsRoot := filepath.Join(tempRoot, "sessions")
	if err := os.MkdirAll(sessionsRoot, 0o700); err != nil {
		fatal(fmt.Errorf("create disposable printer session root: %w", err))
	}
	durablePrinter, err := output.NewDurablePrinter(sessionsRoot, *printerName, target)
	if err != nil {
		fatal(fmt.Errorf("configure durable printer: %w", err))
	}

	started := time.Now()
	report := smokeReport{
		Status:                 "running",
		Printer:                strings.TrimSpace(*printerName),
		MediaForm:              strings.TrimSpace(*mediaForm),
		Landscape:              *landscape,
		FlipVertical:           *flipVertical,
		CopiesRequested:        *copies,
		SourceSHA256:           hex.EncodeToString(sourceHash[:]),
		SourceWidth:            1200,
		SourceHeight:           1800,
		Jobs:                   make([]copyResult, 0, *copies),
		PhysicalPrintConfirmed: false,
	}

	for index := 1; index <= *copies; index++ {
		before, err := listFileNames(absoluteOutputDir)
		if err != nil {
			failReport(*reportPath, &report, started, fmt.Errorf("snapshot output names before copy %d: %w", index, err))
		}
		sessionID, err := newID()
		if err != nil {
			failReport(*reportPath, &report, started, fmt.Errorf("create synthetic session ID for copy %d: %w", index, err))
		}
		renderedPath, sessionDir, err := prepareDurablePrintSession(sessionsRoot, sessionID, sourceBytes)
		if err != nil {
			failReport(*reportPath, &report, started, fmt.Errorf("prepare durable session for copy %d: %w", index, err))
		}
		jobStarted := time.Now()
		localRoot := filepath.Join(tempRoot, "local-output")
		coordinator := output.NewCoordinator(output.NewLocal(localRoot), durablePrinter)
		dispatch, err := coordinator.Dispatch(context.Background(), output.Job{
			SessionID:  sessionID,
			SourcePath: renderedPath,
		})
		if err != nil {
			failReport(*reportPath, &report, started, fmt.Errorf("dispatch local and printer targets for copy %d: %w", index, err))
		}
		if len(dispatch.Results) != 2 {
			failReport(*reportPath, &report, started, fmt.Errorf("copy %d returned %d output results; expected local and printer", index, len(dispatch.Results)))
		}
		localResult, printerResult := dispatch.Results[0], dispatch.Results[1]
		if localResult.Name != output.TargetLocalStorage || localResult.Status != output.TargetSucceeded || localResult.OutputPath == "" {
			failReport(*reportPath, &report, started, fmt.Errorf("copy %d local target did not save successfully: %+v", index, localResult))
		}
		localHash, err := hashFile(localResult.OutputPath)
		if err != nil {
			failReport(*reportPath, &report, started, fmt.Errorf("hash local saved output for copy %d: %w", index, err))
		}
		expectedSourceHash := hex.EncodeToString(sourceHash[:])
		if localHash != expectedSourceHash {
			failReport(*reportPath, &report, started, fmt.Errorf("copy %d local output hash does not match the rendered source", index))
		}
		if printerResult.Name != output.TargetPrinter || printerResult.Status != output.TargetQueued || printerResult.JobID == "" {
			failReport(*reportPath, &report, started, fmt.Errorf("copy %d printer result was not queued with a job ID: %+v", index, printerResult))
		}
		if dispatch.Succeeded != 1 || dispatch.Queued != 1 || dispatch.Failed+dispatch.Cancelled+dispatch.TimedOut != 0 {
			failReport(*reportPath, &report, started, fmt.Errorf("copy %d coordinator counts were unexpected: %+v", index, dispatch))
		}
		manifest, err := session.ReadManifest(sessionDir)
		if err != nil {
			failReport(*reportPath, &report, started, fmt.Errorf("read durable printer manifest for copy %d: %w", index, err))
		}
		if len(manifest.PrintAttempts) != 1 {
			failReport(*reportPath, &report, started, fmt.Errorf("copy %d manifest contains %d print attempts; expected exactly one", index, len(manifest.PrintAttempts)))
		}
		attempt := manifest.PrintAttempts[0]
		if attempt.Status != session.PrintStatusQueued || attempt.JobID != printerResult.JobID || attempt.QueueName != *printerName {
			failReport(*reportPath, &report, started, fmt.Errorf("copy %d manifest printer attempt does not match queued result: %+v", index, attempt))
		}
		if attempt.DocumentName != session.PrintDocumentName(sessionID, attempt.AttemptID) {
			failReport(*reportPath, &report, started, fmt.Errorf("copy %d manifest document name is inconsistent", index))
		}
		inspection, err := output.InspectWindowsPrinterJob(*printerName, attempt.DocumentName, attempt.JobID)
		if err != nil {
			failReport(*reportPath, &report, started, fmt.Errorf("inspect Windows spooler after copy %d: %w", index, err))
		}
		queueStatus := []string(nil)
		if inspection.Found {
			for _, queueJob := range inspection.Jobs {
				queueStatus = append(queueStatus, queueJob.Status...)
			}
		}
		report.QueueInspectionCount++
		// Mirror the workflow's successful local publication update so the
		// disposable recovery fixture ends in a finalized local-output phase.
		manifest.Phase = session.PhaseOutputFinalized
		manifest.OutputPath = localResult.OutputPath
		manifest.OutputFinalizedAt = time.Now().UTC()
		manifest.UpdatedAt = manifest.OutputFinalizedAt
		if err := session.WriteManifest(sessionDir, manifest); err != nil {
			failReport(*reportPath, &report, started, fmt.Errorf("finalize disposable session manifest for copy %d: %w", index, err))
		}
		report.LocalSavedCount++
		report.DurableIntentCount++
		report.SpoolerAcceptedCount++
		report.CoordinatorIndependenceCheck = true

		convertedPath, err := waitForNewJPEG(absoluteOutputDir, before, jobStarted, *timeout)
		if err != nil {
			failReport(*reportPath, &report, started, fmt.Errorf("wait for converted JPEG for copy %d (spooler job %s): %w", index, printerResult.JobID, err))
		}
		convertedFile, err := os.Open(convertedPath)
		if err != nil {
			failReport(*reportPath, &report, started, fmt.Errorf("open converted virtual output for copy %d: %w", index, err))
		}
		config, _, decodeErr := image.DecodeConfig(convertedFile)
		closeErr := convertedFile.Close()
		if decodeErr != nil {
			failReport(*reportPath, &report, started, fmt.Errorf("decode converted virtual JPEG header for copy %d: %w", index, decodeErr))
		}
		if closeErr != nil {
			failReport(*reportPath, &report, started, fmt.Errorf("close converted virtual JPEG for copy %d: %w", index, closeErr))
		}
		if config.Width != *expectedWidth || config.Height != *expectedHeight {
			failReport(*reportPath, &report, started, fmt.Errorf("copy %d converted to %dx%d; expected %dx%d", index, config.Width, config.Height, *expectedWidth, *expectedHeight))
		}
		convertedHash, err := hashFile(convertedPath)
		if err != nil {
			failReport(*reportPath, &report, started, fmt.Errorf("hash converted virtual JPEG for copy %d: %w", index, err))
		}
		report.ConversionSeenCount++
		report.Jobs = append(report.Jobs, copyResult{
			JobID:                printerResult.JobID,
			SessionID:            sessionID,
			AttemptID:            attempt.AttemptID,
			DocumentName:         attempt.DocumentName,
			ManifestStatus:       attempt.Status,
			QueueInspectionFound: inspection.Found,
			QueueStatus:          queueStatus,
			LocalOutputFile:      filepath.Base(localResult.OutputPath),
			LocalSHA256:          localHash,
			Converted:            convertedPath,
			SHA256:               convertedHash,
			Width:                config.Width,
			Height:               config.Height,
			Duration:             time.Since(jobStarted),
		})
		fmt.Printf("COPY %d/%d PASS local-save=PASS printer-job=%s virtual-jpeg=%s\n", index, *copies, printerResult.JobID, filepath.Base(convertedPath))
	}

	report.Status = "pass_virtual_pipeline_only"
	report.Duration = time.Since(started)
	writeReport(*reportPath, report)
	encoded, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(encoded))
}

func writeSyntheticFixture(path string) error {
	const width, height = 1200, 1800
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)

	patches := []color.RGBA{
		{R: 255, A: 255}, {G: 255, A: 255},
		{B: 255, A: 255}, {G: 255, B: 255, A: 255},
		{R: 255, B: 255, A: 255}, {R: 255, G: 255, A: 255},
		{R: 64, G: 64, B: 64, A: 255}, {R: 128, G: 128, B: 128, A: 255},
		{R: 191, G: 191, B: 191, A: 255}, {R: 255, G: 255, B: 255, A: 255},
	}
	for i, patch := range patches {
		col, row := i%2, i/2
		x := 60 + col*540
		y := 210 + row*245
		rect := image.Rect(x, y, x+480, y+155)
		draw.Draw(img, rect, &image.Uniform{C: patch}, image.Point{}, draw.Src)
	}
	// A non-symmetric marker makes a vertical flip visible in the converted JPEG.
	draw.Draw(img, image.Rect(590, 1450, 610, 1680), &image.Uniform{C: color.Black}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(590, 1450, 680, 1470), &image.Uniform{C: color.Black}, image.Point{}, draw.Src)

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	encodeErr := jpeg.Encode(file, img, &jpeg.Options{Quality: 95})
	closeErr := file.Close()
	if encodeErr != nil {
		return encodeErr
	}
	return closeErr
}

func prepareDurablePrintSession(sessionsRoot, sessionID string, renderedBytes []byte) (string, string, error) {
	sessionDir := filepath.Join(sessionsRoot, sessionID)
	captureDir := filepath.Join(sessionDir, "captures")
	if err := os.MkdirAll(captureDir, 0o700); err != nil {
		return "", "", fmt.Errorf("create disposable capture directory: %w", err)
	}

	frameBytes := []byte(`{"schema_version":1,"frame":"synthetic-durable-printer-smoke"}`)
	frameDigest := sha256.Sum256(frameBytes)
	configDigest := sha256.Sum256([]byte("synthetic-durable-printer-smoke-config"))
	framePath, snapshotHash, err := session.WriteFrameSnapshot(sessionDir, frameBytes)
	if err != nil {
		return "", "", fmt.Errorf("write disposable frame snapshot: %w", err)
	}
	manifest := session.NewManifest(
		sessionID, 3, framePath, hex.EncodeToString(frameDigest[:]), hex.EncodeToString(configDigest[:]),
	)
	manifest.FrameSnapshotFile = session.FrameSnapshotFileName
	manifest.FrameSnapshotSHA256 = snapshotHash
	for shot := 1; shot <= manifest.ExpectedShots; shot++ {
		path := filepath.Join(captureDir, fmt.Sprintf("capture-%02d.jpg", shot))
		if err := os.WriteFile(path, renderedBytes, 0o600); err != nil {
			return "", "", fmt.Errorf("write synthetic capture %d: %w", shot, err)
		}
	}
	renderedPath := filepath.Join(sessionDir, "rendered.jpg")
	if err := os.WriteFile(renderedPath, renderedBytes, 0o600); err != nil {
		return "", "", fmt.Errorf("write synthetic rendered image: %w", err)
	}
	manifest.Phase = session.PhaseRendered
	manifest.CompletedShots = manifest.ExpectedShots
	manifest.RenderedAt = time.Now().UTC()
	manifest.UpdatedAt = manifest.RenderedAt
	if err := session.WriteManifest(sessionDir, manifest); err != nil {
		return "", "", fmt.Errorf("persist synthetic rendered-session manifest: %w", err)
	}
	return renderedPath, sessionDir, nil
}

func listFileNames(dir string) (map[string]struct{}, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			names[entry.Name()] = struct{}{}
		}
	}
	return names, nil
}

func waitForNewJPEG(dir string, before map[string]struct{}, started time.Time, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	stableName := ""
	stableSize := int64(-1)
	stableSamples := 0
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return "", err
		}
		var candidates []os.FileInfo
		for _, entry := range entries {
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".jpg") {
				continue
			}
			if _, existed := before[entry.Name()]; existed {
				continue
			}
			info, infoErr := entry.Info()
			if infoErr == nil && info.ModTime().After(started.Add(-2*time.Second)) && info.Size() > 0 {
				candidates = append(candidates, info)
			}
		}
		if len(candidates) > 1 {
			return "", fmt.Errorf("observed %d new JPEGs for one printer job; refusing to choose an output", len(candidates))
		}
		if len(candidates) == 1 {
			current := candidates[0]
			if current.Name() == stableName && current.Size() == stableSize {
				stableSamples++
			} else {
				stableName, stableSize, stableSamples = current.Name(), current.Size(), 0
			}
			if stableSamples >= 2 {
				return filepath.Join(dir, stableName), nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return "", fmt.Errorf("no stable new JPEG appeared in %s within %s", dir, timeout)
}

func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func newID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func writeReport(path string, report smokeReport) {
	if strings.TrimSpace(path) == "" {
		return
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "report encode:", err)
		return
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "report write:", err)
	}
}

func failReport(path string, report *smokeReport, started time.Time, err error) {
	report.Status = "failed"
	report.Error = err.Error()
	report.Duration = time.Since(started)
	writeReport(path, *report)
	fatal(err)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "DS-RX1 virtual smoke failed:", err)
	os.Exit(1)
}
