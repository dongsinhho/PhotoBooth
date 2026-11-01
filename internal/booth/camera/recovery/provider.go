// Package recovery adds bounded close-and-reopen recovery to a camera provider.
package recovery

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/dongsinhho/PhotoBooth/internal/booth/camera"
)

type Factory func() camera.Provider

type Event struct {
	Type    string
	Attempt int
	Total   int
	Err     error
}

// Options controls retries around camera stream loss. OnEvent is synchronous;
// handlers must return quickly and must not call back into this provider.
type Options struct {
	// OpenAttempts bounds the initial open and lazy open on a later session.
	OpenAttempts int
	// RecoveryAttempts bounds reopen attempts after an active stream is lost.
	// If unset, it defaults to OpenAttempts. A larger value gives an operator
	// time to reconnect a physical device without failing the active capture.
	RecoveryAttempts int
	RetryDelay       time.Duration
	ShouldRecover    func(error) bool
	Sleep            func(context.Context, time.Duration) error
	OnEvent          func(Event)
}

type Provider struct {
	mu      sync.Mutex
	factory Factory
	options Options
	active  camera.Provider
	info    camera.Info
}

var _ camera.Provider = (*Provider)(nil)

func New(factory Factory, options Options) *Provider {
	if options.OpenAttempts < 1 {
		options.OpenAttempts = 3
	}
	if options.OpenAttempts > 10 {
		options.OpenAttempts = 10
	}
	if options.RecoveryAttempts < 1 {
		options.RecoveryAttempts = options.OpenAttempts
	}
	if options.RecoveryAttempts > 120 {
		options.RecoveryAttempts = 120
	}
	if options.RetryDelay <= 0 {
		options.RetryDelay = 250 * time.Millisecond
	}
	if options.ShouldRecover == nil {
		options.ShouldRecover = func(err error) bool {
			return errors.Is(err, camera.ErrStreamUnavailable)
		}
	}
	return &Provider{factory: factory, options: options}
}

func (p *Provider) Info() camera.Info {
	if p == nil {
		return camera.Info{Name: "Recovering camera provider", Backend: "recovery"}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	info := p.info
	if p.active != nil {
		info = p.active.Info()
	}
	info.Capabilities.Resolution = append([]camera.Resolution(nil), info.Capabilities.Resolution...)
	return info
}

func (p *Provider) Open(ctx context.Context) error {
	if p == nil {
		return errors.New("camera recovery provider is nil")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.active != nil {
		return errors.New("camera recovery provider is already open")
	}
	return p.openFresh(ctx, false)
}

func (p *Provider) Close() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.active == nil {
		return nil
	}
	if err := p.active.Close(); err != nil {
		// Preserve the reference so a later Close can retry cleanup. In
		// particular, native providers may still own a worker/OS thread.
		return fmt.Errorf("close camera provider: %w", err)
	}
	p.active = nil
	return nil
}

func (p *Provider) Capture(ctx context.Context, req camera.CaptureRequest) (camera.CaptureResult, error) {
	if p == nil {
		return camera.CaptureResult{}, errors.New("camera recovery provider is nil")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return camera.CaptureResult{}, err
	}
	if p.active == nil {
		if err := p.openFresh(ctx, false); err != nil {
			return camera.CaptureResult{}, fmt.Errorf("open camera before capture: %w", err)
		}
	}

	result, err := p.active.Capture(ctx, req)
	if err == nil {
		return result, nil
	}
	if isContextError(ctx, err) || !p.options.ShouldRecover(err) {
		return camera.CaptureResult{}, err
	}

	originalErr := err
	p.emit(Event{Type: "recovery_started", Err: originalErr})
	old := p.active
	if closeErr := old.Close(); closeErr != nil {
		return camera.CaptureResult{}, errors.Join(
			fmt.Errorf("capture failed: %w", originalErr),
			fmt.Errorf("close failed provider before recovery: %w", closeErr),
		)
	}
	p.active = nil

	// Providers are expected to write atomically, but remove any artifact from
	// a failed attempt so a retry cannot leave a stale or partial capture.
	if req.OutputPath != "" {
		if removeErr := os.Remove(req.OutputPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return camera.CaptureResult{}, errors.Join(
				fmt.Errorf("capture failed: %w", originalErr),
				fmt.Errorf("remove failed capture artifact: %w", removeErr),
			)
		}
	}

	if reopenErr := p.openFresh(ctx, true); reopenErr != nil {
		recoveryErr := fmt.Errorf("reopen camera after stream failure: %w", reopenErr)
		p.emit(Event{Type: "recovery_failed", Err: recoveryErr})
		return camera.CaptureResult{}, errors.Join(
			fmt.Errorf("initial capture failed: %w", originalErr),
			recoveryErr,
		)
	}

	result, retryErr := p.active.Capture(ctx, req)
	if retryErr == nil {
		p.emit(Event{Type: "recovery_succeeded"})
		return result, nil
	}
	if errors.Is(retryErr, camera.ErrStreamUnavailable) {
		if closeErr := p.active.Close(); closeErr == nil {
			p.active = nil
		} else {
			retryErr = errors.Join(retryErr, fmt.Errorf("close failed provider after retry: %w", closeErr))
		}
	}
	recoveryErr := fmt.Errorf("capture retry after reopening failed: %w", retryErr)
	p.emit(Event{Type: "recovery_failed", Err: recoveryErr})
	return camera.CaptureResult{}, errors.Join(
		fmt.Errorf("initial capture failed: %w", originalErr),
		recoveryErr,
	)
}

// Preview reads from the active provider's existing stream. It uses the same
// bounded close/reopen recovery policy as Capture, without creating or deleting
// capture artifacts.
func (p *Provider) Preview(ctx context.Context) ([]byte, error) {
	if p == nil {
		return nil, errors.New("camera recovery provider is nil")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.active == nil {
		if err := p.openFresh(ctx, false); err != nil {
			return nil, fmt.Errorf("open camera before preview: %w", err)
		}
	}
	previewer, ok := p.active.(camera.PreviewProvider)
	if !ok {
		return nil, errors.New("active camera provider does not support preview")
	}
	frame, err := previewer.Preview(ctx)
	if err == nil {
		return frame, nil
	}
	if isContextError(ctx, err) || !p.options.ShouldRecover(err) {
		return nil, err
	}

	originalErr := err
	p.emit(Event{Type: "recovery_started", Err: originalErr})
	old := p.active
	if closeErr := old.Close(); closeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("preview failed: %w", originalErr),
			fmt.Errorf("close failed provider before preview recovery: %w", closeErr),
		)
	}
	p.active = nil
	if reopenErr := p.openFresh(ctx, true); reopenErr != nil {
		recoveryErr := fmt.Errorf("reopen camera after preview stream failure: %w", reopenErr)
		p.emit(Event{Type: "recovery_failed", Err: recoveryErr})
		return nil, errors.Join(fmt.Errorf("initial preview failed: %w", originalErr), recoveryErr)
	}
	previewer, ok = p.active.(camera.PreviewProvider)
	if !ok {
		return nil, errors.New("reopened camera provider does not support preview")
	}
	frame, retryErr := previewer.Preview(ctx)
	if retryErr == nil {
		p.emit(Event{Type: "recovery_succeeded"})
		return frame, nil
	}
	if errors.Is(retryErr, camera.ErrStreamUnavailable) {
		if closeErr := p.active.Close(); closeErr == nil {
			p.active = nil
		} else {
			retryErr = errors.Join(retryErr, fmt.Errorf("close failed provider after preview retry: %w", closeErr))
		}
	}
	recoveryErr := fmt.Errorf("preview retry after reopening failed: %w", retryErr)
	p.emit(Event{Type: "recovery_failed", Err: recoveryErr})
	return nil, errors.Join(fmt.Errorf("initial preview failed: %w", originalErr), recoveryErr)
}

func (p *Provider) openFresh(ctx context.Context, recovering bool) error {
	if p.factory == nil {
		return errors.New("camera recovery provider has no factory")
	}
	attemptLimit := p.options.OpenAttempts
	if recovering {
		attemptLimit = p.options.RecoveryAttempts
	}
	var lastErr error
	for attempt := 1; attempt <= attemptLimit; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		candidate := p.factory()
		if candidate == nil {
			lastErr = errors.New("camera factory returned a nil provider")
		} else {
			info := candidate.Info()
			if info.Name != "" || info.Backend != "" {
				p.info = info
			}
			openErr := candidate.Open(ctx)
			if openErr == nil {
				p.active = candidate
				p.info = candidate.Info()
				if recovering {
					p.emit(Event{Type: "recovery_provider_opened", Attempt: attempt, Total: attemptLimit})
				}
				return nil
			}
			closeErr := candidate.Close()
			lastErr = errors.Join(openErr, closeErr)
			if closeErr != nil {
				// Do not create another provider while this candidate may still
				// own native resources.
				p.active = candidate
				return fmt.Errorf("camera open attempt %d failed and cleanup failed: %w", attempt, lastErr)
			}
		}
		if recovering {
			p.emit(Event{Type: "recovery_open_attempt_failed", Attempt: attempt, Total: attemptLimit, Err: lastErr})
		}
		if attempt == attemptLimit {
			break
		}
		// Linear backoff capped at 2s prevents a fast spin without making the
		// operator wait increasingly long between device re-enumeration checks.
		delay := p.options.RetryDelay * time.Duration(min(attempt, 4))
		if delay > 2*time.Second {
			delay = 2 * time.Second
		}
		if err := p.sleep(ctx, delay); err != nil {
			return errors.Join(lastErr, err)
		}
	}
	return fmt.Errorf("camera open failed after %d attempt(s): %w", attemptLimit, lastErr)
}

func (p *Provider) sleep(ctx context.Context, delay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if p.options.Sleep != nil {
		return p.options.Sleep(ctx, delay)
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (p *Provider) emit(event Event) {
	if p.options.OnEvent != nil {
		p.options.OnEvent(event)
	}
}

func isContextError(ctx context.Context, err error) bool {
	return ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
