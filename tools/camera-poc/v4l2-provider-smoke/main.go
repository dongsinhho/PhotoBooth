package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	"os"
	"path/filepath"
	"time"

	"github.com/dongsinhho/PhotoBooth/internal/booth/camera"
	"github.com/dongsinhho/PhotoBooth/internal/booth/camera/v4l2"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL %v\n", err)
		os.Exit(1)
	}
}

func run() (runErr error) {
	device := flag.String("device", "/dev/video0", "V4L2 capture device node")
	width := flag.Int("width", 640, "requested frame width")
	height := flag.Int("height", 480, "requested frame height")
	fps := flag.Float64("fps", 15, "requested integer frames per second")
	frames := flag.Int("frames", 3, "number of captures in each open cycle")
	iterations := flag.Int("iterations", 1, "number of open/capture/close cycles using the same provider instance")
	streamSoak := flag.Duration("stream-soak", 0, "keep one provider stream open and continuously preview/capture for this duration (max 30m)")
	streamSoakInterval := flag.Duration("stream-soak-interval", 500*time.Millisecond, "minimum interval between continuous soak preview/capture pairs")
	skipWarmup := flag.Bool("skip-warmup", false, "skip the default one-second warm-up (diagnostics only)")
	flag.Parse()

	if *frames < 1 || *frames > 12 {
		return fmt.Errorf("-frames must be between 1 and 12")
	}
	if *iterations < 1 || *iterations > 100 {
		return fmt.Errorf("-iterations must be between 1 and 100")
	}
	if *streamSoak < 0 || *streamSoak > 30*time.Minute {
		return fmt.Errorf("-stream-soak must be between 0 and 30m")
	}
	if *streamSoak > 0 {
		if *iterations != 1 {
			return fmt.Errorf("-stream-soak cannot be combined with -iterations greater than 1")
		}
		if *streamSoakInterval < 100*time.Millisecond || *streamSoakInterval > 5*time.Second {
			return fmt.Errorf("-stream-soak-interval must be between 100ms and 5s")
		}
	}

	outDir, err := os.MkdirTemp("", "photobooth-v4l2-provider-smoke-*")
	if err != nil {
		return fmt.Errorf("create temporary output: %w", err)
	}

	provider := v4l2.New(v4l2.Options{
		DevicePath:   *device,
		Width:        *width,
		Height:       *height,
		FPS:          *fps,
		BufferCount:  4,
		FrameTimeout: 5 * time.Second,
		SkipWarmup:   *skipWarmup,
	})
	defer func() {
		if closeErr := provider.Close(); closeErr != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close provider during cleanup: %w", closeErr))
		}
		if removeErr := os.RemoveAll(outDir); removeErr != nil {
			runErr = errors.Join(runErr, fmt.Errorf("remove temporary output: %w", removeErr))
		}
	}()

	contextLimit := 6 * time.Minute
	if *streamSoak > 0 {
		contextLimit = *streamSoak + time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), contextLimit)
	defer cancel()

	if *streamSoak > 0 {
		if err := runContinuousStreamSoak(ctx, provider, outDir, *width, *height, *fps, !*skipWarmup, *streamSoak, *streamSoakInterval); err != nil {
			return fmt.Errorf("continuous stream soak: %w", err)
		}
		return nil
	}

	started := time.Now()
	totalCaptures := 0
	for iteration := 1; iteration <= *iterations; iteration++ {
		iterationStarted := time.Now()
		if err := provider.Open(ctx); err != nil {
			return fmt.Errorf("open provider cycle %d/%d: %w", iteration, *iterations, err)
		}
		info := provider.Info()
		fmt.Printf("OPEN_PASS iteration=%d/%d device=%s name=%q backend=%s resolution=%dx%d requested_fps=%.2f warmup=%t\n",
			iteration, *iterations, info.ID, info.Name, info.Backend, *width, *height, *fps, !*skipWarmup)

		preview, err := provider.Preview(ctx)
		if err != nil {
			return fmt.Errorf("preview cycle %d/%d: %w", iteration, *iterations, err)
		}
		previewImage, _, err := image.Decode(bytes.NewReader(preview))
		if err != nil {
			return fmt.Errorf("decode preview cycle %d/%d: %w", iteration, *iterations, err)
		}
		if previewImage.Bounds().Dx() != *width || previewImage.Bounds().Dy() != *height {
			return fmt.Errorf("preview dimensions cycle %d/%d = %v, want %dx%d", iteration, *iterations, previewImage.Bounds(), *width, *height)
		}
		fmt.Printf("PREVIEW_PASS iteration=%d bytes=%d dimensions=%dx%d\n",
			iteration, len(preview), previewImage.Bounds().Dx(), previewImage.Bounds().Dy())

		for sequence := 1; sequence <= *frames; sequence++ {
			path := filepath.Join(outDir, fmt.Sprintf("cycle-%02d-capture-%02d.jpg", iteration, sequence))
			captureStarted := time.Now()
			result, err := provider.Capture(ctx, camera.CaptureRequest{
				Sequence: sequence, Width: *width, Height: *height, OutputPath: path,
			})
			if err != nil {
				return fmt.Errorf("capture cycle %d/%d frame %d/%d: %w", iteration, *iterations, sequence, *frames, err)
			}
			if err := verifyJPEG(result.Path, *width, *height); err != nil {
				return fmt.Errorf("validate capture cycle %d/%d frame %d/%d: %w", iteration, *iterations, sequence, *frames, err)
			}
			stat, err := os.Stat(result.Path)
			if err != nil {
				return fmt.Errorf("stat captured JPEG: %w", err)
			}
			fmt.Printf("CAPTURE_PASS iteration=%d sequence=%d bytes=%d dimensions=%dx%d elapsed=%s\n",
				iteration, sequence, stat.Size(), *width, *height, time.Since(captureStarted))
			totalCaptures++
		}

		if err := provider.Close(); err != nil {
			return fmt.Errorf("close provider cycle %d/%d: %w", iteration, *iterations, err)
		}
		fmt.Printf("CLOSE_PASS iteration=%d/%d cycle_elapsed=%s\n",
			iteration, *iterations, time.Since(iterationStarted))
	}

	fmt.Printf("SMOKE_PASS open_close_cycles=%d captures=%d total_elapsed=%s\n",
		*iterations, totalCaptures, time.Since(started))
	return nil
}

func runContinuousStreamSoak(ctx context.Context, provider *v4l2.Provider, outDir string, width, height int, fps float64, warmup bool, duration, interval time.Duration) error {
	if err := provider.Open(ctx); err != nil {
		return fmt.Errorf("open provider: %w", err)
	}
	info := provider.Info()
	fmt.Printf("STREAM_SOAK_START device=%s name=%q resolution=%dx%d requested_fps=%.2f duration=%s interval=%s warmup=%t\n",
		info.ID, info.Name, width, height, fps, duration, interval, warmup)

	started := time.Now()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	captures := 0
	for time.Since(started) < duration {
		preview, err := provider.Preview(ctx)
		if err != nil {
			return fmt.Errorf("preview sequence %d: %w", captures+1, err)
		}
		if err := verifyJPEGBytes(preview, width, height); err != nil {
			return fmt.Errorf("validate preview sequence %d: %w", captures+1, err)
		}

		sequence := captures + 1
		path := filepath.Join(outDir, fmt.Sprintf("stream-capture-%06d.jpg", sequence))
		captureStarted := time.Now()
		result, err := provider.Capture(ctx, camera.CaptureRequest{
			Sequence: sequence, Width: width, Height: height, OutputPath: path,
		})
		if err != nil {
			return fmt.Errorf("capture sequence %d: %w", sequence, err)
		}
		if err := verifyJPEG(result.Path, width, height); err != nil {
			return fmt.Errorf("validate capture sequence %d: %w", sequence, err)
		}
		stat, err := os.Stat(result.Path)
		if err != nil {
			return fmt.Errorf("stat capture sequence %d: %w", sequence, err)
		}
		captures++
		elapsed := time.Since(started)
		if captures%20 == 0 || elapsed >= duration {
			fmt.Printf("STREAM_SOAK_PROGRESS captures=%d previews=%d last_capture_bytes=%d last_capture_elapsed=%s elapsed=%s\n",
				captures, captures, stat.Size(), time.Since(captureStarted), elapsed)
		}
		if elapsed >= duration {
			break
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("context ended after %d captures: %w", captures, ctx.Err())
		case <-ticker.C:
		}
	}

	if err := provider.Close(); err != nil {
		return fmt.Errorf("close provider after soak: %w", err)
	}
	fmt.Printf("STREAM_SOAK_PASS target_duration=%s actual_elapsed=%s open_close_cycles=1 captures=%d previews=%d\n",
		duration, time.Since(started), captures, captures)
	return nil
}

func verifyJPEGBytes(data []byte, width, height int) error {
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}
	if decoded.Bounds().Dx() != width || decoded.Bounds().Dy() != height {
		return fmt.Errorf("dimensions=%dx%d, want %dx%d", decoded.Bounds().Dx(), decoded.Bounds().Dy(), width, height)
	}
	return nil
}

func verifyJPEG(path string, width, height int) error {
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
	if decoded.Bounds().Dx() != width || decoded.Bounds().Dy() != height {
		return fmt.Errorf("dimensions=%dx%d, want %dx%d", decoded.Bounds().Dx(), decoded.Bounds().Dy(), width, height)
	}
	return nil
}
