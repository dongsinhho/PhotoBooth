// Package v4l2 provides Linux Video4Linux2 camera inventory and an
// experimental V4L2 camera.Provider implementation.
package v4l2

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/dongsinhho/PhotoBooth/internal/booth/camera"
)

const (
	defaultDevicePath = "/dev/video0"
	defaultWidth      = 640
	defaultHeight     = 480
	defaultFPS        = 15.0
)

// Options configures the experimental V4L2 provider. The defaults deliberately
// match the only B525 mode currently validated on the WSL2 reference host.
type Options struct {
	DevicePath     string
	Width          int
	Height         int
	FPS            float64
	BufferCount    int
	FrameTimeout   time.Duration
	WarmupDuration time.Duration
	// SkipWarmup is intended only for controlled diagnostics and tests.
	SkipWarmup bool
}

type backend interface {
	Info() camera.Info
	Open(context.Context) error
	Close() error
	Capture(context.Context, camera.CaptureRequest) (camera.CaptureResult, error)
	Preview(context.Context) ([]byte, error)
}

// Provider adapts a single-planar Linux V4L2 MJPEG stream to camera.Provider.
// It remains experimental and is not selected by station configuration.
type Provider struct {
	backend backend
}

var _ camera.Provider = (*Provider)(nil)
var _ camera.PreviewProvider = (*Provider)(nil)

// New creates an experimental V4L2 provider. Open reports all device,
// capability, format-negotiation, permission and streaming errors.
func New(options Options) *Provider {
	if strings.TrimSpace(options.DevicePath) == "" {
		options.DevicePath = defaultDevicePath
	}
	options.DevicePath = strings.TrimSpace(options.DevicePath)
	if options.Width == 0 {
		options.Width = defaultWidth
	}
	if options.Height == 0 {
		options.Height = defaultHeight
	}
	if options.FPS <= 0 {
		options.FPS = defaultFPS
	}
	if options.BufferCount == 0 {
		options.BufferCount = 4
	}
	if options.FrameTimeout <= 0 {
		options.FrameTimeout = 5 * time.Second
	}
	if options.SkipWarmup {
		options.WarmupDuration = 0
	} else {
		if options.WarmupDuration <= 0 {
			options.WarmupDuration = time.Second
		}
		if options.WarmupDuration > 3*time.Second {
			options.WarmupDuration = 3 * time.Second
		}
	}
	return &Provider{backend: newBackend(options)}
}

func (p *Provider) Info() camera.Info {
	if p == nil || p.backend == nil {
		return camera.Info{Name: "Linux V4L2 camera", Backend: "v4l2"}
	}
	return p.backend.Info()
}

func (p *Provider) Open(ctx context.Context) error {
	if p == nil || p.backend == nil {
		return errors.New("Linux V4L2 provider is not initialized")
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
		return camera.CaptureResult{}, errors.New("Linux V4L2 provider is not initialized")
	}
	return p.backend.Capture(ctx, req)
}

// Preview reads one validated frame from the already-open V4L2 stream.
// Preview and Capture share the provider mutex and never create another reader.
func (p *Provider) Preview(ctx context.Context) ([]byte, error) {
	if p == nil || p.backend == nil {
		return nil, errors.New("Linux V4L2 provider is not initialized")
	}
	return p.backend.Preview(ctx)
}
