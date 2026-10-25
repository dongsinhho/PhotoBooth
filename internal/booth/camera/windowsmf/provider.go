// Package windowsmf adapts Windows Media Foundation UVC capture to the
// platform-independent camera.Provider interface.
package windowsmf

import (
	"context"
	"errors"
	"strings"

	"github.com/dongsinhho/PhotoBooth/internal/booth/camera"
)

type Options struct {
	// DeviceID can be a device friendly name or exact Windows symbolic link.
	// Empty selects the first enumerated camera.
	DeviceID string
	Width    int
	Height   int
	FPS      float64
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
