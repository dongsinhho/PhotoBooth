//go:build !windows

package windowsmf

import (
	"context"
	"errors"
	"runtime"

	"github.com/dongsinhho/PhotoBooth/internal/booth/camera"
)

type unsupportedBackend struct{ options Options }

func newBackend(options Options) backend { return &unsupportedBackend{options: options} }

func (b *unsupportedBackend) Info() camera.Info {
	return camera.Info{
		ID: b.options.DeviceID, Name: "Windows Media Foundation camera",
		OS: runtime.GOOS, Backend: "media-foundation",
	}
}

func (b *unsupportedBackend) Open(context.Context) error {
	return errors.New("Windows Media Foundation camera provider is only supported on Windows")
}
func (b *unsupportedBackend) Close() error { return nil }
func (b *unsupportedBackend) Capture(context.Context, camera.CaptureRequest) (camera.CaptureResult, error) {
	return camera.CaptureResult{}, errors.New("Windows Media Foundation camera provider is only supported on Windows")
}
