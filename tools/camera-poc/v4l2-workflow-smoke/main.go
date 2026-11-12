package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/dongsinhho/PhotoBooth/internal/booth/camera"
	"github.com/dongsinhho/PhotoBooth/internal/booth/camera/recovery"
	"github.com/dongsinhho/PhotoBooth/internal/booth/camera/v4l2"
	"github.com/dongsinhho/PhotoBooth/internal/booth/output"
	"github.com/dongsinhho/PhotoBooth/internal/booth/render"
	"github.com/dongsinhho/PhotoBooth/internal/booth/session"
	"github.com/dongsinhho/PhotoBooth/internal/booth/workflow"
)

const (
	recoveryAttempts      = 60
	recoveryRetryDelay    = 500 * time.Millisecond
	recoveryMaxRetryDelay = 2 * time.Second
	recoveryCycleOverhead = 15 * time.Second
)

func main() {
	defer func() {
		if recovered := recover(); recovered != nil {
			if failure, ok := recovered.(smokeFailure); ok {
				fmt.Fprintf(os.Stderr, "FAIL %s\n", failure.message)
				os.Exit(1)
			}
			panic(recovered)
		}
	}()

	device := flag.String("device", "/dev/video0", "V4L2 capture device node")
	width := flag.Int("width", 640, "requested capture width")
	height := flag.Int("height", 480, "requested capture height")
	fps := flag.Float64("fps", 15, "requested integer FPS")
	shots := flag.Int("shots", 3, "number of photos (must match frame slots)")
	frame := flag.String("frame", "tools/camera-poc/sample-frame.json", "frame JSON relative to current directory")
	testHotplug := flag.Bool("test-hotplug", false, "arm physical USB detach/attach recovery checks during shot countdowns")
	hotplugCycles := flag.Int("hotplug-cycles", 1, "number of physical detach/attach recovery cycles to require (must not exceed shots)")
	hotplugCountdown := flag.Duration("hotplug-countdown", 20*time.Second, "per-shot countdown for each hotplug test cycle")
	flag.Parse()

	if *shots < 1 || *shots > 12 {
		fatalf("-shots must be between 1 and 12")
	}
	if *testHotplug && (*hotplugCountdown < 10*time.Second || *hotplugCountdown > 60*time.Second) {
		fatalf("-hotplug-countdown must be between 10s and 60s for a hotplug test")
	}
	if *testHotplug && (*hotplugCycles < 1 || *hotplugCycles > *shots) {
		fatalf("-hotplug-cycles must be between 1 and %d", *shots)
	}
	if !*testHotplug && *hotplugCycles != 1 {
		fatalf("-hotplug-cycles requires -test-hotplug")
	}

	framePath, err := filepath.Abs(*frame)
	if err != nil {
		fatalf("resolve frame path: %v", err)
	}
	if _, err := os.Stat(framePath); err != nil {
		fatalf("frame config: %v", err)
	}

	tempRoot, err := os.MkdirTemp("", "photobooth-v4l2-workflow-smoke-*")
	if err != nil {
		fatalf("create temporary test root: %v", err)
	}
	defer os.RemoveAll(tempRoot)

	workDir := filepath.Join(tempRoot, "sessions")
	outputDir := filepath.Join(tempRoot, "output")
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		fatalf("create temporary sessions root: %v", err)
	}
	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		fatalf("create temporary output root: %v", err)
	}

	cameraOptions := v4l2.Options{
		DevicePath:   *device,
		Width:        *width,
		Height:       *height,
		FPS:          *fps,
		BufferCount:  4,
		FrameTimeout: 15 * time.Second,
	}
	var successfulHotplugRecoveries atomic.Int32
	recoveringCamera := recovery.New(func() camera.Provider {
		return v4l2.New(cameraOptions)
	}, recovery.Options{
		OpenAttempts:     3,
		RecoveryAttempts: recoveryAttempts,
		RetryDelay:       recoveryRetryDelay,
		OnEvent: func(event recovery.Event) {
			if event.Type == "recovery_succeeded" {
				successfulHotplugRecoveries.Add(1)
			}
			fmt.Printf("CAMERA_RECOVERY type=%s attempt=%d/%d message=%q error=%v\n",
				event.Type, event.Attempt, event.Total, event.Message, event.Err)
		},
	})
	runner := &workflow.Runner{
		Camera:   recoveringCamera,
		Renderer: render.New(),
		Output:   output.NewLocal(outputDir),
		OnEvent: func(event workflow.Event) {
			switch event.Type {
			case "failed":
				fmt.Fprintf(os.Stderr, "WORKFLOW_EVENT type=failed state=%s message=%q\n", event.State, event.Message)
			case "state":
				fmt.Printf("WORKFLOW_STATE state=%s message=%q\n", event.State, event.Message)
			case "countdown":
				if event.Seconds == 0 || event.Seconds == int((*hotplugCountdown)/time.Second) || event.Seconds%5 == 0 {
					fmt.Printf("WORKFLOW_COUNTDOWN shot=%d seconds=%d\n", event.Shot, event.Seconds)
				}
			}
		},
	}

	countdown := time.Duration(0)
	if *testHotplug {
		countdown = *hotplugCountdown
		fmt.Printf("HOTPLUG_TEST_ARMED device=%s busid=6-4 shots=%d required_cycles=%d per_shot_countdown=%s\n",
			*device, *shots, *hotplugCycles, countdown)
		fmt.Println("For each of the first required shot countdowns: detach B525 from Windows, keep it detached until recovery-open failures appear, then attach it again. Wait for recovery_succeeded before the next shot.")
	}

	workflowTimeout := 180 * time.Second
	if *testHotplug {
		retryBudget := recoveryRetryBudget(recoveryAttempts, recoveryRetryDelay, recoveryMaxRetryDelay)
		workflowTimeout = time.Duration(*hotplugCycles)*(*hotplugCountdown+retryBudget+recoveryCycleOverhead) + 60*time.Second
		fmt.Printf("HOTPLUG_TIMEOUT_BUDGET cycles=%d countdown_per_cycle=%s recovery_retry_budget_per_cycle=%s overhead_per_cycle=%s total=%s\n",
			*hotplugCycles, *hotplugCountdown, retryBudget, recoveryCycleOverhead, workflowTimeout)
	}
	ctx, cancel := context.WithTimeout(context.Background(), workflowTimeout)
	defer cancel()

	started := time.Now()
	result, err := runner.Run(ctx, workflow.Config{
		FrameConfigPath:     framePath,
		WorkDir:             workDir,
		OutputDir:           outputDir,
		CaptureCount:        *shots,
		Countdown:           countdown,
		CaptureWidth:        *width,
		CaptureHeight:       *height,
		CameraProvider:      "v4l2-experimental",
		CameraDevice:        *device,
		CameraFPS:           *fps,
		ConfigSchemaVersion: 1,
		MediaDeliveryMode:   "local",
		PaymentMode:         "disabled",
	})
	if err != nil {
		fatalf("workflow run: %v (state=%s session=%s)", err, result.State, result.SessionID)
	}
	if result.State != session.Completed {
		fatalf("workflow state = %s, want completed", result.State)
	}
	if *testHotplug {
		observed := int(successfulHotplugRecoveries.Load())
		if observed < *hotplugCycles {
			fatalf("observed %d successful camera recoveries, want at least %d", observed, *hotplugCycles)
		}
		fmt.Printf("HOTPLUG_RECOVERY_PASS requested_cycles=%d observed_successes=%d\n", *hotplugCycles, observed)
	}
	if len(result.CapturePaths) != *shots {
		fatalf("workflow committed %d captures, want %d", len(result.CapturePaths), *shots)
	}

	for index, path := range result.CapturePaths {
		if err := verifyJPEG(path, *width, *height); err != nil {
			fatalf("capture %d validation: %v", index+1, err)
		}
		fmt.Printf("WORKFLOW_CAPTURE_PASS sequence=%d path=%s\n", index+1, path)
	}
	if err := verifyJPEG(result.RenderedPath, -1, -1); err != nil {
		fatalf("rendered JPEG validation: %v", err)
	}
	if result.OutputPath == "" {
		fatalf("workflow returned empty local output path")
	}
	if err := verifyJPEG(result.OutputPath, -1, -1); err != nil {
		fatalf("local output JPEG validation: %v", err)
	}

	fmt.Printf("WORKFLOW_PASS session=%s state=%s captures=%d rendered=%s local_output=%s total_elapsed=%s\n",
		result.SessionID, result.State, len(result.CapturePaths), result.RenderedPath, result.OutputPath, time.Since(started))
}

func recoveryRetryBudget(attempts int, retryDelay, maxDelay time.Duration) time.Duration {
	if attempts <= 1 || retryDelay <= 0 || maxDelay <= 0 {
		return 0
	}
	var budget time.Duration
	for attempt := 1; attempt < attempts; attempt++ {
		delay := retryDelay * time.Duration(min(attempt, 4))
		if delay > maxDelay {
			delay = maxDelay
		}
		budget += delay
	}
	return budget
}

func verifyJPEG(path string, width, height int) error {
	if path == "" {
		return fmt.Errorf("JPEG path is empty")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	decoded, _, decodeErr := image.Decode(file)
	closeErr := file.Close()
	if decodeErr != nil {
		return decodeErr
	}
	if closeErr != nil {
		return closeErr
	}
	if width > 0 && decoded.Bounds().Dx() != width {
		return fmt.Errorf("width=%d, want %d", decoded.Bounds().Dx(), width)
	}
	if height > 0 && decoded.Bounds().Dy() != height {
		return fmt.Errorf("height=%d, want %d", decoded.Bounds().Dy(), height)
	}
	return nil
}

type smokeFailure struct {
	message string
}

func fatalf(format string, args ...any) {
	panic(smokeFailure{message: fmt.Sprintf(format, args...)})
}
