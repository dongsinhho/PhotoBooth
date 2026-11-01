//go:build windows

// This manual hardware test keeps the app process alive and exercises the same
// recovery decorator used by the Windows booth-player CLI.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dongsinhho/PhotoBooth/internal/booth/camera"
	"github.com/dongsinhho/PhotoBooth/internal/booth/camera/recovery"
	"github.com/dongsinhho/PhotoBooth/internal/booth/camera/windowsmf"
)

func main() {
	out := filepath.Join(os.TempDir(), "photobooth-hotplug-recovery")
	if err := os.RemoveAll(out); err != nil {
		fmt.Printf("CLEAN_TEMP_ERROR %v\n", err)
		return
	}
	if err := os.MkdirAll(out, 0700); err != nil {
		fmt.Printf("OUTPUT_DIR_ERROR %v\n", err)
		return
	}
	defer func() { _ = os.RemoveAll(out) }()
	options := windowsmf.Options{DeviceID: "B525 HD Webcam", Width: 1280, Height: 720, FPS: 30}
	recoveryStarted := false
	p := recovery.New(func() camera.Provider {
		return windowsmf.New(options)
	}, recovery.Options{
		OpenAttempts:     3,
		RecoveryAttempts: 30,
		RetryDelay:       250 * time.Millisecond,
		OnEvent: func(e recovery.Event) {
			switch e.Type {
			case "recovery_started":
				recoveryStarted = true
				fmt.Printf("RECOVERY_STARTED err=%v\n", e.Err)
			case "recovery_open_attempt_failed":
				fmt.Printf("REOPEN_ATTEMPT_FAILED attempt=%d/%d err=%v\n", e.Attempt, e.Total, e.Err)
			case "recovery_provider_opened":
				fmt.Printf("REOPEN_OK attempt=%d\n", e.Attempt)
			case "recovery_succeeded":
				fmt.Println("AUTO_RECOVERY_SUCCEEDED")
			case "recovery_failed":
				fmt.Printf("AUTO_RECOVERY_FAILED err=%v\n", e.Err)
			}
		},
	})
	ctx := context.Background()
	if err := p.Open(ctx); err != nil {
		fmt.Printf("INITIAL_OPEN_ERROR %v\n", err)
		return
	}
	defer func() {
		fmt.Println("CLOSING")
		fmt.Printf("CLOSE_RESULT %v\n", p.Close())
	}()
	fmt.Println("TEST_READY - integrated camera recovery wrapper opened B525")
	fmt.Println("ACTION: unplug the webcam now; tell me 'đã rút'.")
	ok, failed, afterRecovery := 0, 0, false
	path := filepath.Join(out, "latest.jpg")
	for i := 1; i <= 600; i++ {
		started := time.Now()
		result, err := p.Capture(ctx, camera.CaptureRequest{Sequence: i, OutputPath: path, Width: 1280, Height: 720})
		elapsed := time.Since(started).Round(time.Millisecond)
		if err != nil {
			failed++
			if failed == 1 || failed%10 == 0 {
				fmt.Printf("CAPTURE_ERROR attempt=%03d errors=%d elapsed=%s err=%v\n", i, failed, elapsed, err)
			}
		} else {
			ok++
			if recoveryStarted && !afterRecovery {
				afterRecovery = true
				fmt.Printf("CAPTURE_OK_AFTER_RECOVERY attempt=%03d successes=%d elapsed=%s path=%s\n", i, ok, elapsed, result.Path)
				fmt.Println("ACTION: auto-recovery verified; keep the process alive until the test loop finishes.")
			} else if ok%10 == 0 {
				fmt.Printf("CAPTURE_OK attempt=%03d successes=%d elapsed=%s\n", i, ok, elapsed)
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Printf("SUMMARY successes=%d failed_capture_calls=%d recovered=%v\n", ok, failed, afterRecovery)
}
