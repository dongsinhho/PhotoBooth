//go:build windows

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"time"

	camera "github.com/Kirizu-Official/windows-camera-go/camera/v1"
)

type deviceRecord struct {
	Name       string `json:"name"`
	SymbolLink string `json:"symbol_link"`
}

type sampleRecord struct {
	TimestampUTC     string         `json:"timestamp_utc"`
	Sample           int            `json:"sample"`
	ThreadRole       string         `json:"thread_role"`
	ElapsedMS        float64        `json:"elapsed_ms"`
	Status           string         `json:"status"`
	DeviceCount      int            `json:"device_count"`
	Devices          []deviceRecord `json:"devices"`
	InitDurationMS   float64        `json:"init_duration_ms"`
	EnumDurationMS   float64        `json:"enum_duration_ms"`
	SampleDurationMS float64        `json:"sample_duration_ms"`
	InitError        string         `json:"init_error,omitempty"`
	EnumerationError string         `json:"enumeration_error,omitempty"`
}

func main() {
	os.Exit(run())
}

func run() int {
	interval := flag.Duration("interval", 500*time.Millisecond, "Delay between Media Foundation enumeration samples")
	duration := flag.Duration("duration", 3*time.Minute, "Total observer runtime")
	output := flag.String("output", "", "New JSONL output file (must not already exist)")
	threadProbeTrigger := flag.String("thread-probe-trigger-file", "", "When this file appears, sample once on a second locked OS thread")
	flag.Parse()

	if *interval <= 0 {
		fmt.Fprintln(os.Stderr, "interval must be greater than zero")
		return 2
	}
	if *duration <= 0 {
		fmt.Fprintln(os.Stderr, "duration must be greater than zero")
		return 2
	}
	if *output == "" {
		fmt.Fprintln(os.Stderr, "output is required; refusing to run without a diagnostic log")
		return 2
	}

	// The vendored camera library initializes COM/MF on the calling OS thread.
	// Keep every Init -> EnumDevice -> Shutdown sequence pinned to this thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	outputPath, err := filepath.Abs(*output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve output path: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create output directory: %v\n", err)
		return 1
	}
	threadProbePath := ""
	if *threadProbeTrigger != "" {
		threadProbePath, err = filepath.Abs(*threadProbeTrigger)
		if err != nil {
			fmt.Fprintf(os.Stderr, "resolve thread-probe trigger path: %v\n", err)
			return 1
		}
		if _, statErr := os.Stat(threadProbePath); statErr == nil {
			fmt.Fprintln(os.Stderr, "thread-probe trigger file already exists; remove it before starting the observer")
			return 2
		} else if !errors.Is(statErr, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "check thread-probe trigger path: %v\n", statErr)
			return 1
		}
	}
	file, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create new output file (existing files are never overwritten): %v\n", err)
		return 1
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	started := time.Now()
	fmt.Printf("MF_OBSERVER_STARTED output=%s interval=%s duration=%s\n", outputPath, *interval, *duration)
	fmt.Println("MF observer uses a separate process and does not open a camera capture stream.")
	if threadProbePath != "" {
		fmt.Printf("MF_SECONDARY_THREAD_PROBE_WAITING_FOR=%s\n", threadProbePath)
	}

	signalContext, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopSignals()
	ctx, cancel := context.WithTimeout(signalContext, *duration)
	defer cancel()

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	sampleNumber := 0
	for {
		sampleNumber++
		record := collectSample(sampleNumber, started)
		record.ThreadRole = "primary_locked_thread"
		if err := writeSample(encoder, file, record); err != nil {
			fmt.Fprintf(os.Stderr, "write JSONL sample %d: %v\n", sampleNumber, err)
			return 1
		}

		if threadProbePath != "" {
			if _, statErr := os.Stat(threadProbePath); statErr == nil {
				if err := os.Remove(threadProbePath); err != nil {
					fmt.Fprintf(os.Stderr, "remove thread-probe trigger file: %v\n", err)
					return 1
				}
				sampleNumber++
				secondaryRecord := collectSampleOnNewLockedThread(sampleNumber, started)
				if err := writeSample(encoder, file, secondaryRecord); err != nil {
					fmt.Fprintf(os.Stderr, "write secondary-thread sample %d: %v\n", sampleNumber, err)
					return 1
				}
				fmt.Printf("MF_SECONDARY_THREAD_PROBE_RESULT status=%s devices=%d elapsed_ms=%.1f sample_ms=%.1f\n",
					secondaryRecord.Status, secondaryRecord.DeviceCount, secondaryRecord.ElapsedMS, secondaryRecord.SampleDurationMS)
			}
		}

		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				fmt.Printf("MF_OBSERVER_FINISHED reason=duration samples=%d output=%s\n", sampleNumber, outputPath)
			} else {
				fmt.Printf("MF_OBSERVER_FINISHED reason=interrupt samples=%d output=%s\n", sampleNumber, outputPath)
			}
			return 0
		case <-ticker.C:
		}
	}
}

func writeSample(encoder *json.Encoder, file *os.File, record sampleRecord) error {
	if err := encoder.Encode(record); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	fmt.Printf("MF_SAMPLE=%d thread=%s status=%s devices=%d elapsed_ms=%.1f sample_ms=%.1f",
		record.Sample, record.ThreadRole, record.Status, record.DeviceCount, record.ElapsedMS, record.SampleDurationMS)
	if len(record.Devices) > 0 {
		fmt.Printf(" first_device=%q", record.Devices[0].Name)
	}
	if record.InitError != "" {
		fmt.Printf(" init_error=%q", record.InitError)
	}
	if record.EnumerationError != "" {
		fmt.Printf(" enum_error=%q", record.EnumerationError)
	}
	fmt.Println()
	return nil
}

// collectSampleOnNewLockedThread runs a one-off enumeration on an OS thread
// distinct from the primary observer thread, which remains pinned by run().
func collectSampleOnNewLockedThread(sampleNumber int, runStarted time.Time) sampleRecord {
	result := make(chan sampleRecord, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		record := collectSample(sampleNumber, runStarted)
		record.ThreadRole = "secondary_locked_thread"
		result <- record
	}()
	return <-result
}

func collectSample(sampleNumber int, runStarted time.Time) sampleRecord {
	sampleStarted := time.Now()
	record := sampleRecord{
		TimestampUTC: sampleStarted.UTC().Format(time.RFC3339Nano),
		Sample:       sampleNumber,
		ElapsedMS:    float64(sampleStarted.Sub(runStarted).Microseconds()) / 1000,
		DeviceCount:  -1,
		Devices:      []deviceRecord{},
		Status:       "init_error",
	}

	initStarted := time.Now()
	err := camera.Init()
	record.InitDurationMS = elapsedMilliseconds(initStarted)
	if err != nil {
		record.InitError = err.Error()
		record.SampleDurationMS = elapsedMilliseconds(sampleStarted)
		return record
	}

	enumStarted := time.Now()
	devices, enumErr := camera.EnumDevice()
	record.EnumDurationMS = elapsedMilliseconds(enumStarted)
	camera.Shutdown()

	if enumErr != nil {
		record.Status = "enumeration_error"
		record.EnumerationError = enumErr.Error()
	} else {
		record.DeviceCount = len(devices)
		record.Status = deviceStatus(record.DeviceCount)
		for _, device := range devices {
			if device == nil {
				continue
			}
			record.Devices = append(record.Devices, deviceRecord{
				Name:       device.Name,
				SymbolLink: device.SymbolLink,
			})
		}
		record.DeviceCount = len(record.Devices)
	}
	record.SampleDurationMS = elapsedMilliseconds(sampleStarted)
	return record
}

func deviceStatus(deviceCount int) string {
	if deviceCount == 0 {
		return "zero_sources"
	}
	return "camera_found"
}

func elapsedMilliseconds(start time.Time) float64 {
	return float64(time.Since(start).Microseconds()) / 1000
}
