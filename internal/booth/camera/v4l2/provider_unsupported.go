//go:build !linux || !amd64

package v4l2

import (
	"context"
	"errors"

	"github.com/dongsinhho/PhotoBooth/internal/booth/camera"
)

type unsupportedBackend struct{ options Options }

func newBackend(options Options) backend { return &unsupportedBackend{options: options} }

func (b *unsupportedBackend) Info() camera.Info {
	return camera.Info{
		ID: b.options.DevicePath, Name: "Linux V4L2 camera",
		Backend: "v4l2", OS: "linux",
	}
}

func (b *unsupportedBackend) Open(context.Context) error {
	return errors.New("V4L2 camera provider is only supported on Linux")
}

func (b *unsupportedBackend) Close() error { return nil }

func (b *unsupportedBackend) Capture(context.Context, camera.CaptureRequest) (camera.CaptureResult, error) {
	return camera.CaptureResult{}, errors.New("V4L2 camera provider is only supported on Linux")
}

func (b *unsupportedBackend) Preview(context.Context) ([]byte, error) {
	return nil, errors.New("V4L2 camera provider is only supported on Linux")
}
