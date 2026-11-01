// Package windowsmf adapts Windows Media Foundation UVC capture to the
// platform-independent camera.Provider interface.
package windowsmf

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/dongsinhho/PhotoBooth/internal/booth/camera"
)

const (
	// defaultWarmupDuration lets UVC auto-exposure and white balance settle before
	// the first guest-facing photo is captured.
	defaultWarmupDuration = time.Second
	maxWarmupDuration     = 3 * time.Second
)

type Options struct {
	// DeviceID can be a device friendly name or exact Windows symbolic link.
	// Empty selects the first enumerated camera.
	DeviceID string
	Width    int
	Height   int
	FPS      float64

	// WarmupDuration is how long to consume and discard stream frames after
	// opening the camera so auto-exposure/white-balance can settle. Values <= 0
	// use the default; the duration is capped to keep startup bounded.
	WarmupDuration time.Duration

	// SkipWarmupForDiagnostics disables frame warm-up only for a controlled
	// diagnostic comparison. Normal booth/player paths must leave this false.
	SkipWarmupForDiagnostics bool
}

type backend interface {
	Info() camera.Info
	Open(context.Context) error
	Close() error
	Capture(context.Context, camera.CaptureRequest) (camera.CaptureResult, error)
}

type Provider struct{ backend backend }

var _ camera.Provider = (*Provider)(nil)

func New(options Options) *Provider {
	if options.Width == 0 {
		options.Width = 1280
	}
	if options.Height == 0 {
		options.Height = 720
	}
	if options.FPS <= 0 {
		options.FPS = 30
	}
	if options.SkipWarmupForDiagnostics {
		options.WarmupDuration = 0
	} else {
		if options.WarmupDuration <= 0 {
			options.WarmupDuration = defaultWarmupDuration
		}
		if options.WarmupDuration > maxWarmupDuration {
			options.WarmupDuration = maxWarmupDuration
		}
	}
	options.DeviceID = strings.TrimSpace(options.DeviceID)
	return &Provider{backend: newBackend(options)}
}

func (p *Provider) Info() camera.Info {
	if p == nil || p.backend == nil {
		return camera.Info{Name: "Windows Media Foundation camera", Backend: "media-foundation"}
	}
	return p.backend.Info()
}

func (p *Provider) Open(ctx context.Context) error {
	if p == nil || p.backend == nil {
		return errors.New("Windows Media Foundation provider is not initialized")
	}
	return p.backend.Open(ctx)
}

func (p *Provider) Close() error {
	if p == nil || p.backend == nil {
		return nil
	}
	return p.backend.Close()
}

func (p *Provider) Capture(ctx context.Context, req camera.CaptureRequest) (camera.CaptureResult, error) {
	if p == nil || p.backend == nil {
		return camera.CaptureResult{}, errors.New("Windows Media Foundation provider is not initialized")
	}
	return p.backend.Capture(ctx, req)
}

// Preview returns one validated JPEG frame from the existing Media Foundation
// worker. It never opens another camera reader or writes a capture file.
func (p *Provider) Preview(ctx context.Context) ([]byte, error) {
	if p == nil || p.backend == nil {
		return nil, errors.New("Windows Media Foundation provider is not initialized")
	}
	previewer, ok := p.backend.(camera.PreviewProvider)
	if !ok {
		return nil, errors.New("camera preview is not supported by this platform backend")
	}
	return previewer.Preview(ctx)
}
