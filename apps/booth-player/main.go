package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dongsinhho/PhotoBooth/internal/booth/camera"
	fakecamera "github.com/dongsinhho/PhotoBooth/internal/booth/camera/fake"
	"github.com/dongsinhho/PhotoBooth/internal/booth/camera/recovery"
	"github.com/dongsinhho/PhotoBooth/internal/booth/camera/windowsmf"
	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
	"github.com/dongsinhho/PhotoBooth/internal/booth/output"
	"github.com/dongsinhho/PhotoBooth/internal/booth/render"
	"github.com/dongsinhho/PhotoBooth/internal/booth/session"
	"github.com/dongsinhho/PhotoBooth/internal/booth/workflow"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "PhotoBooth demo failed: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	settingsArgs, skipCameraWarmup, err := extractSkipCameraWarmup(args)
	if err != nil {
		return err
	}
	effective, err := resolveSettings(settingsArgs, stderr)
	if err != nil {
		return err
	}
	framePath := effective.Frame.ConfigPath
	cameraFlag := &effective.Camera.Provider
	deviceFlag := &effective.Camera.Device
	fpsFlag := &effective.Camera.FPS
	workDir := &effective.Storage.SessionsDir
	outputDir := &effective.Storage.OutputDir
	shots := &effective.Capture.Shots
	width := &effective.Camera.Width
	height := &effective.Camera.Height
	countdownValue, err := effective.CountdownDuration()
	if err != nil {
		return err
	}
	intervalValue, err := effective.IntervalDuration()
	if err != nil {
		return err
	}
	countdown := &countdownValue
	interval := &intervalValue

	modeLabel := "OFFLINE DEMO (synthetic camera; no physical camera is accessed)"
	if strings.EqualFold(strings.TrimSpace(*cameraFlag), "windowsmf") {
		modeLabel = "LOCAL CAMERA RUN (Windows Media Foundation; photos saved locally)"
	}
	fmt.Fprintf(stdout, "PhotoBooth - %s\n", modeLabel)
	fmt.Fprintf(stdout, "Frame: %s\n", framePath)
	fmt.Fprintf(stdout, "Provider: %s; device selector: %q\n", *cameraFlag, *deviceFlag)
	fmt.Fprintf(stdout, "Settings: %d shot(s), %dx%d, target FPS=%.1f, countdown=%s, interval=%s\n", *shots, *width, *height, *fpsFlag, *countdown, *interval)

	if skipCameraWarmup && !strings.EqualFold(strings.TrimSpace(*cameraFlag), "windowsmf") {
		return errors.New("--skip-camera-warmup is available only with --camera=windowsmf and is intended for diagnostics")
	}
	if skipCameraWarmup {
		fmt.Fprintln(stdout, "WARNING: camera stream warm-up is disabled for this diagnostic run; do not use this option for normal booth sessions.")
	}

	var cameraProvider camera.Provider
	switch strings.ToLower(strings.TrimSpace(*cameraFlag)) {
	case "fake":
		cameraProvider = fakecamera.New()
	case "windowsmf":
		cameraOptions := windowsmf.Options{
			DeviceID: *deviceFlag, Width: *width, Height: *height, FPS: *fpsFlag,
			SkipWarmupForDiagnostics: skipCameraWarmup,
		}
		cameraProvider = recovery.New(func() camera.Provider {
			return windowsmf.New(cameraOptions)
		}, recovery.Options{
			// Initial startup should fail quickly. After a mid-session USB
			// disconnect, keep this capture alive for a bounded reconnect window.
			OpenAttempts:     3,
			RecoveryAttempts: 30,
			RetryDelay:       250 * time.Millisecond,
			OnEvent: func(event recovery.Event) {
				switch event.Type {
				case "recovery_started":
					fmt.Fprintln(stderr, "Camera stream failed; attempting to reopen the camera...")
				case "recovery_open_attempt_failed":
					fmt.Fprintf(stderr, "Camera reconnect attempt %d/%d failed: %v\n", event.Attempt, event.Total, event.Err)
				case "recovery_provider_opened":
					fmt.Fprintf(stdout, "Camera provider reopened on attempt %d. Retrying capture.\n", event.Attempt)
				case "recovery_succeeded":
					fmt.Fprintln(stdout, "Camera recovery succeeded.")
				case "recovery_failed":
					fmt.Fprintf(stderr, "Camera recovery failed: %v\n", event.Err)
				}
			},
		})
	default:
		return fmt.Errorf("unsupported camera provider %q (choose fake or windowsmf)", *cameraFlag)
	}

	runner := &workflow.Runner{
		Camera:   cameraProvider,
		Renderer: render.New(),
		Output:   output.NewLocal(*outputDir),
		OnEvent: func(event workflow.Event) {
			switch event.Type {
			case "state":
				fmt.Fprintf(stdout, "[%s] %s\n", event.State, event.Message)
			case "countdown":
				if event.Seconds == 0 {
					fmt.Fprintf(stdout, "Shot %d/%d: countdown disabled\n", event.Shot, event.Total)
				} else {
					fmt.Fprintf(stdout, "Shot %d/%d in %dâ€¦\n", event.Shot, event.Total, event.Seconds)
				}
			case "captured":
				fmt.Fprintf(stdout, "Captured shot %d/%d in %s\n", event.Shot, event.Total, event.Duration.Round(time.Millisecond))
			case "rendered":
				fmt.Fprintf(stdout, "Rendered in %s\n", event.Duration.Round(time.Millisecond))
			case "output_target":
				if event.Status == string(output.TargetQueued) {
					fmt.Fprintf(stdout, "Output target %s queued job %s in %s\n", event.Target, event.JobID, event.Duration.Round(time.Millisecond))
				} else if event.Status == string(output.TargetSucceeded) {
					if event.Path != "" {
						fmt.Fprintf(stdout, "Output target %s succeeded in %s: %s\n", event.Target, event.Duration.Round(time.Millisecond), event.Path)
					} else {
						fmt.Fprintf(stdout, "Output target %s succeeded in %s\n", event.Target, event.Duration.Round(time.Millisecond))
					}
				} else {
					fmt.Fprintf(stderr, "Output target %s %s in %s: %s\n", event.Target, event.Status, event.Duration.Round(time.Millisecond), event.Message)
				}
			case "completed", "partial_success":
				fmt.Fprintf(stdout, "Session %s ended in state %s.\n", event.SessionID, event.State)
			case "failed":
				fmt.Fprintf(stderr, "Session ended in %s: %s\n", event.State, event.Message)
			}
		},
	}
	result, err := runner.Run(ctx, workflow.Config{
		FrameConfigPath: framePath,
		WorkDir:         *workDir,
		CaptureCount:    *shots,
		Countdown:       *countdown,
		Interval:        *interval,
		CaptureWidth:    *width,
		CaptureHeight:   *height,
	})
	if err != nil {
		if result.State == session.Failed || result.State == session.Cancelled || result.State == session.TimedOut {
			return fmt.Errorf("session %s ended in state %s: %w", result.SessionID, result.State, err)
		}
		return err
	}
	fmt.Fprintf(stdout, "Rendered image: %s\n", result.RenderedPath)
	if result.OutputPath != "" {
		fmt.Fprintf(stdout, "Locally saved output: %s\n", result.OutputPath)
	} else {
		fmt.Fprintln(stdout, "No local copy was saved; check the output-target results above.")
	}
	if strings.EqualFold(*cameraFlag, "fake") {
		fmt.Fprintln(stdout, "This run used synthetic images. Connect a real camera provider before using it with guests.")
	} else {
		fmt.Fprintln(stdout, "This run used a real camera stream. Confirm lighting, framing, and retention settings before production use.")
	}
	return nil
}

// extractSkipCameraWarmup removes the explicitly diagnostic-only A/B flag
// before the station-config flag parser processes the normal CLI arguments.
func extractSkipCameraWarmup(args []string) ([]string, bool, error) {
	filtered := make([]string, 0, len(args))
	skip := false
	seen := false
	for _, arg := range args {
		if arg == "--skip-camera-warmup" || arg == "-skip-camera-warmup" {
			if seen {
				return nil, false, errors.New("--skip-camera-warmup may be specified only once")
			}
			seen, skip = true, true
			continue
		}
		if strings.HasPrefix(arg, "--skip-camera-warmup=") || strings.HasPrefix(arg, "-skip-camera-warmup=") {
			if seen {
				return nil, false, errors.New("--skip-camera-warmup may be specified only once")
			}
			seen = true
			value := strings.SplitN(arg, "=", 2)[1]
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return nil, false, fmt.Errorf("parse --skip-camera-warmup: %w", err)
			}
			skip = parsed
			continue
		}
		filtered = append(filtered, arg)
	}
	return filtered, skip, nil
}

// resolveSettings loads an optional station config then applies only flags
// explicitly supplied by the operator. This preserves file values when unrelated
// flags are omitted, while keeping existing CLI flags as convenient overrides.
func resolveSettings(args []string, stderr io.Writer) (stationconfig.StoreConfig, error) {
	flags := flag.NewFlagSet("booth-player", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "path to versioned local station config JSON")
	frameFlag := flags.String("frame", "", "path to a schema v1 frame JSON")
	cameraFlag := flags.String("camera", "fake", "camera provider: fake or windowsmf (Windows only)")
	deviceFlag := flags.String("device", "", "camera friendly name or exact symbolic link (windowsmf only)")
	fpsFlag := flags.Float64("fps", 30, "requested camera stream frame rate (windowsmf only)")
	workDir := flags.String("sessions-dir", filepath.Join("data", "sessions"), "local directory for temporary session captures and render")
	outputDir := flags.String("output-dir", filepath.Join("data", "output"), "local directory for finalized photos")
	shots := flags.Int("shots", 3, "number of captures (1-12; must match frame slots)")
	countdown := flags.Duration("countdown", 3*time.Second, "countdown duration per shot, e.g. 3s or 0s")
	interval := flags.Duration("interval", time.Second, "delay between shots, e.g. 1s or 0s")
	width := flags.Int("width", 1280, "camera capture width")
	height := flags.Int("height", 720, "camera capture height")
	if err := flags.Parse(args); err != nil {
		return stationconfig.StoreConfig{}, err
	}
	if flags.NArg() != 0 {
		return stationconfig.StoreConfig{}, fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}

	provided := make(map[string]bool)
	flags.Visit(func(f *flag.Flag) { provided[f.Name] = true })
	cfg := stationconfig.Defaults()
	configLoaded := provided["config"]
	if configLoaded {
		if strings.TrimSpace(*configPath) == "" {
			return stationconfig.StoreConfig{}, errors.New("--config requires a non-empty file path")
		}
		loaded, err := stationconfig.Load(*configPath)
		if err != nil {
			return stationconfig.StoreConfig{}, err
		}
		cfg = loaded
	}

	if provided["camera"] {
		cfg.Camera.Provider = *cameraFlag
	}
	if provided["device"] {
		cfg.Camera.Device = *deviceFlag
	}
	if provided["fps"] {
		cfg.Camera.FPS = *fpsFlag
	}
	if provided["sessions-dir"] {
		cfg.Storage.SessionsDir = *workDir
	}
	if provided["output-dir"] {
		cfg.Storage.OutputDir = *outputDir
	}
	if provided["shots"] {
		cfg.Capture.Shots = *shots
	}
	if provided["countdown"] {
		cfg.Capture.Countdown = countdown.String()
	}
	if provided["interval"] {
		cfg.Capture.Interval = interval.String()
	}
	if provided["width"] {
		cfg.Camera.Width = *width
	}
	if provided["height"] {
		cfg.Camera.Height = *height
	}
	if provided["frame"] {
		cfg.Frame.ConfigPath = *frameFlag
	}

	if err := cfg.Validate(); err != nil {
		return stationconfig.StoreConfig{}, err
	}
	var frameErr error
	if configLoaded || provided["frame"] {
		cfg.Frame.ConfigPath, frameErr = resolveFrameConfig(cfg.Frame.ConfigPath)
	} else {
		cfg.Frame.ConfigPath, frameErr = resolveFrameConfig("")
	}
	if frameErr != nil {
		return stationconfig.StoreConfig{}, frameErr
	}
	return cfg, nil
}

func resolveFrameConfig(explicit string) (string, error) {
	if explicit != "" {
		info, err := os.Stat(explicit)
		if err != nil {
			return "", fmt.Errorf("frame config %q: %w", explicit, err)
		}
		if !info.Mode().IsRegular() {
			return "", errors.New("frame config must be a regular file")
		}
		return filepath.Abs(explicit)
	}
	candidates := []string{filepath.Join("examples", "frames", "photo-strip-v1.json")}
	if executable, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(executable)
		candidates = append(candidates,
			filepath.Join(exeDir, "examples", "frames", "photo-strip-v1.json"),
			filepath.Join(exeDir, "..", "..", "examples", "frames", "photo-strip-v1.json"),
		)
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && info.Mode().IsRegular() {
			absolute, err := filepath.Abs(candidate)
			if err != nil {
				return "", err
			}
			return absolute, nil
		}
	}
	return "", errors.New("sample frame not found; run from the repository root or pass --frame <path>")
}
