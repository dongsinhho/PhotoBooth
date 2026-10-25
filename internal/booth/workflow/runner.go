package workflow

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dongsinhho/PhotoBooth/internal/booth/camera"
	"github.com/dongsinhho/PhotoBooth/internal/booth/output"
	"github.com/dongsinhho/PhotoBooth/internal/booth/session"
)

type Renderer interface {
	SlotCount(ctx context.Context, configPath string) (int, error)
	Render(ctx context.Context, configPath string, inputPaths []string, outputPath string) error
}

type Config struct {
	FrameConfigPath string
	WorkDir         string
	CaptureCount    int
	Countdown       time.Duration
	Interval        time.Duration
	CaptureWidth    int
	CaptureHeight   int
	SessionID       string // Optional. Generated automatically when empty.
}

type Event struct {
	Type      string
	State     session.State
	SessionID string
	Shot      int
	Total     int
	Seconds   int
	Path      string
	Message   string
}

type Result struct {
	SessionID    string
	State        session.State
	CapturePaths []string
	RenderedPath string
	OutputPath   string
	StartedAt    time.Time
	FinishedAt   time.Time
}

type Runner struct {
	Camera   camera.Provider
	Renderer Renderer
	Output   output.Provider
	OnEvent  func(Event)
	// Sleep can be injected by tests to exercise countdown/interval paths instantly.
	Sleep func(context.Context, time.Duration) error
}

func (r *Runner) Run(ctx context.Context, cfg Config) (result Result, runErr error) {
	result.StartedAt = time.Now()
	result.State = session.Idle
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			result.State = session.TimedOut
		} else {
			result.State = session.Cancelled
		}
		result.FinishedAt = time.Now()
		return result, err
	}
	if r == nil || r.Camera == nil || r.Renderer == nil || r.Output == nil {
		return result, errors.New("camera, renderer, and output providers are required")
	}
	if cfg.CaptureCount < 1 || cfg.CaptureCount > 12 {
		return result, errors.New("capture count must be between 1 and 12")
	}
	if strings.TrimSpace(cfg.WorkDir) == "" {
		return result, errors.New("work directory is required")
	}
	if strings.TrimSpace(cfg.FrameConfigPath) == "" {
		return result, errors.New("frame config path is required")
	}
	if cfg.Countdown < 0 || cfg.Interval < 0 {
		return result, errors.New("countdown and interval cannot be negative")
	}
	if cfg.SessionID == "" {
		var err error
		cfg.SessionID, err = newSessionID()
		if err != nil {
			return result, fmt.Errorf("generate session ID: %w", err)
		}
	}
	if !validSessionID(cfg.SessionID) {
		return result, errors.New("session ID must be 32 lowercase hexadecimal characters")
	}
	result.SessionID = cfg.SessionID

	if err := r.transition(&result, session.Preflight, "validate frame configuration and prepare local session"); err != nil {
		return result, err
	}
	slots, err := r.Renderer.SlotCount(ctx, cfg.FrameConfigPath)
	if err != nil {
		return r.fail(result, fmt.Errorf("validate frame config: %w", err))
	}
	if slots != cfg.CaptureCount {
		return r.fail(result, fmt.Errorf("frame defines %d photo slots, but session is configured for %d captures", slots, cfg.CaptureCount))
	}
	if err := ctx.Err(); err != nil {
		return r.fail(result, err)
	}
	sessionDir := filepath.Join(cfg.WorkDir, cfg.SessionID)
	captureDir := filepath.Join(sessionDir, "captures")
	if err := os.MkdirAll(captureDir, 0o700); err != nil {
		return r.fail(result, fmt.Errorf("create session workspace: %w", err))
	}

	if err := r.Camera.Open(ctx); err != nil {
		return r.fail(result, fmt.Errorf("open camera %q: %w", r.Camera.Info().Name, err))
	}
	cameraOpen := true
	defer func() {
		if cameraOpen {
			if closeErr := r.Camera.Close(); closeErr != nil && runErr == nil {
				runErr = fmt.Errorf("close camera: %w", closeErr)
				if session.CanTransition(result.State, session.Failed) {
					result.State = session.Failed
				}
				result.FinishedAt = time.Now()
				r.emit(Event{Type: "failed", State: result.State, SessionID: result.SessionID, Message: runErr.Error()})
			}
		}
	}()

	if err := r.transition(&result, session.Capturing, "camera opened; begin countdown and capture"); err != nil {
		return r.fail(result, err)
	}
	for shot := 1; shot <= cfg.CaptureCount; shot++ {
		if err := ctx.Err(); err != nil {
			return r.fail(result, err)
		}
		if err := r.countdown(ctx, &result, cfg.Countdown, shot, cfg.CaptureCount); err != nil {
			return r.fail(result, err)
		}
		path := filepath.Join(captureDir, fmt.Sprintf("capture-%02d.jpg", shot))
		captured, err := r.Camera.Capture(ctx, camera.CaptureRequest{
			Sequence: shot, Width: cfg.CaptureWidth, Height: cfg.CaptureHeight, OutputPath: path,
		})
		if err != nil {
			return r.fail(result, fmt.Errorf("capture shot %d/%d: %w", shot, cfg.CaptureCount, err))
		}
		if captured.Sequence != shot {
			return r.fail(result, fmt.Errorf("camera returned sequence %d for requested shot %d", captured.Sequence, shot))
		}
		if strings.TrimSpace(captured.Path) == "" {
			return r.fail(result, fmt.Errorf("camera returned an empty path for shot %d", shot))
		}
		if filepath.Clean(captured.Path) != filepath.Clean(path) {
			return r.fail(result, fmt.Errorf("camera returned an unexpected path for shot %d: got %q, requested %q", shot, captured.Path, path))
		}
		info, err := os.Stat(captured.Path)
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			if err == nil {
				err = errors.New("capture output is not a non-empty regular file")
			}
			return r.fail(result, fmt.Errorf("validate capture shot %d output: %w", shot, err))
		}
		result.CapturePaths = append(result.CapturePaths, captured.Path)
		r.emit(Event{Type: "captured", State: result.State, SessionID: result.SessionID, Shot: shot, Total: cfg.CaptureCount, Path: captured.Path})
		if shot < cfg.CaptureCount && cfg.Interval > 0 {
			if err := r.sleep(ctx, cfg.Interval); err != nil {
				return r.fail(result, err)
			}
		}
	}

	// Close the device before rendering; hardware is no longer needed.
	if err := r.Camera.Close(); err != nil {
		cameraOpen = false
		return r.fail(result, fmt.Errorf("close camera after capture: %w", err))
	}
	cameraOpen = false

	if err := r.transition(&result, session.Rendering, "compose captured images with selected frame"); err != nil {
		return r.fail(result, err)
	}
	renderedPath := filepath.Join(sessionDir, "rendered.jpg")
	if err := r.Renderer.Render(ctx, cfg.FrameConfigPath, result.CapturePaths, renderedPath); err != nil {
		return r.fail(result, fmt.Errorf("render session output: %w", err))
	}
	info, err := os.Stat(renderedPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		if err == nil {
			err = errors.New("rendered output is not a non-empty regular file")
		}
		return r.fail(result, fmt.Errorf("validate rendered output: %w", err))
	}
	result.RenderedPath = renderedPath

	if err := r.transition(&result, session.Output, "save finalized photo locally"); err != nil {
		return r.fail(result, err)
	}
	outputPath, err := r.Output.Save(ctx, result.SessionID, renderedPath)
	if err != nil {
		return r.fail(result, fmt.Errorf("save local output: %w", err))
	}
	result.OutputPath = outputPath
	if err := r.transition(&result, session.Completed, "session complete"); err != nil {
		return r.fail(result, err)
	}
	result.FinishedAt = time.Now()
	r.emit(Event{Type: "completed", State: result.State, SessionID: result.SessionID, Path: result.OutputPath})
	return result, nil
}

func (r *Runner) countdown(ctx context.Context, result *Result, duration time.Duration, shot, total int) error {
	if duration <= 0 {
		r.emit(Event{Type: "countdown", State: result.State, SessionID: result.SessionID, Shot: shot, Total: total, Seconds: 0})
		return ctx.Err()
	}
	seconds := int((duration + time.Second - 1) / time.Second)
	deadline := time.Now().Add(duration)
	for remaining := seconds; remaining > 0; remaining-- {
		r.emit(Event{Type: "countdown", State: result.State, SessionID: result.SessionID, Shot: shot, Total: total, Seconds: remaining})
		until := time.Until(deadline)
		if until <= 0 {
			break
		}
		if err := r.sleep(ctx, min(time.Second, until)); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func (r *Runner) transition(result *Result, next session.State, message string) error {
	if !session.CanTransition(result.State, next) {
		return fmt.Errorf("invalid session transition %s -> %s", result.State, next)
	}
	result.State = next
	r.emit(Event{Type: "state", State: next, SessionID: result.SessionID, Message: message})
	return nil
}

func (r *Runner) fail(result Result, err error) (Result, error) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		if session.CanTransition(result.State, session.TimedOut) {
			result.State = session.TimedOut
		}
	case errors.Is(err, context.Canceled):
		if session.CanTransition(result.State, session.Cancelled) {
			result.State = session.Cancelled
		}
	default:
		if session.CanTransition(result.State, session.Failed) {
			result.State = session.Failed
		}
	}
	result.FinishedAt = time.Now()
	r.emit(Event{Type: "failed", State: result.State, SessionID: result.SessionID, Message: err.Error()})
	return result, err
}

func (r *Runner) emit(event Event) {
	if r.OnEvent != nil {
		r.OnEvent(event)
	}
}

func (r *Runner) sleep(ctx context.Context, duration time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if duration <= 0 {
		return nil
	}
	if r.Sleep != nil {
		return r.Sleep(ctx, duration)
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func newSessionID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}

func validSessionID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, c := range id {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
