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
	"syscall"
	"time"

	fakecamera "github.com/dongsinhho/PhotoBooth/internal/booth/camera/fake"
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
	flags := flag.NewFlagSet("booth-player", flag.ContinueOnError)
	flags.SetOutput(stderr)
	frameFlag := flags.String("frame", "", "path to a schema v1 frame JSON")
	workDir := flags.String("sessions-dir", filepath.Join("data", "sessions"), "local directory for temporary session captures and render")
	outputDir := flags.String("output-dir", filepath.Join("data", "output"), "local directory for finalized photos")
	shots := flags.Int("shots", 3, "number of captures (1-12; must match frame slots)")
	countdown := flags.Duration("countdown", 3*time.Second, "countdown duration per shot, e.g. 3s or 0s")
	interval := flags.Duration("interval", time.Second, "delay between shots, e.g. 1s or 0s")
	width := flags.Int("width", 640, "synthetic capture width")
	height := flags.Int("height", 480, "synthetic capture height")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}
	framePath, err := resolveFrameConfig(*frameFlag)
	if err != nil {
		return err
	}

	fmt.Fprintln(stdout, "PhotoBooth — OFFLINE DEMO (synthetic camera; no physical camera is accessed)")
	fmt.Fprintf(stdout, "Frame: %s\n", framePath)
	fmt.Fprintf(stdout, "Settings: %d shot(s), countdown=%s, interval=%s\n", *shots, *countdown, *interval)

	runner := &workflow.Runner{
		Camera:   fakecamera.New(),
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
					fmt.Fprintf(stdout, "Shot %d/%d in %d…\n", event.Shot, event.Total, event.Seconds)
				}
			case "captured":
				fmt.Fprintf(stdout, "Captured shot %d/%d\n", event.Shot, event.Total)
			case "completed":
				fmt.Fprintf(stdout, "Session %s completed.\n", event.SessionID)
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
	fmt.Fprintf(stdout, "Locally saved output: %s\n", result.OutputPath)
	fmt.Fprintln(stdout, "This run used synthetic images. Connect a real camera provider before using it with guests.")
	return nil
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
