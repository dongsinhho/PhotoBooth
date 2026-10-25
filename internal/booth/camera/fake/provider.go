// Package fake contains deterministic synthetic providers for tests and demos.
// It never accesses a physical camera.
package fake

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"sync"

	"github.com/dongsinhho/PhotoBooth/internal/booth/camera"
)

type Provider struct {
	mu     sync.Mutex
	opened bool
	FailAt int
}

func New() *Provider { return &Provider{} }

func (p *Provider) Info() camera.Info {
	return camera.Info{
		ID: "fake://synthetic-camera", Name: "Synthetic demo camera", OS: "any", Backend: "fake",
		Capabilities: camera.Capabilities{StillCapture: true, Resolution: []camera.Resolution{{Width: 640, Height: 480}}},
	}
}

func (p *Provider) Open(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.opened {
		return errors.New("fake camera is already open")
	}
	p.opened = true
	return nil
}

func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.opened = false
	return nil
}

func (p *Provider) Capture(ctx context.Context, req camera.CaptureRequest) (camera.CaptureResult, error) {
	if err := ctx.Err(); err != nil {
		return camera.CaptureResult{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.opened {
		return camera.CaptureResult{}, errors.New("fake camera is not open")
	}
	if p.FailAt > 0 && req.Sequence == p.FailAt {
		return camera.CaptureResult{}, fmt.Errorf("synthetic camera failure on shot %d", req.Sequence)
	}
	if req.Sequence < 1 || req.OutputPath == "" {
		return camera.CaptureResult{}, errors.New("sequence and output path are required")
	}
	w, h := req.Width, req.Height
	if w == 0 {
		w = 640
	}
	if h == 0 {
		h = 480
	}
	if w < 64 || h < 64 || w > 4000 || h > 4000 {
		return camera.CaptureResult{}, errors.New("requested dimensions must be between 64 and 4000 pixels")
	}

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	shift := uint8((req.Sequence * 53) % 180)
	for y := 0; y < h; y++ {
		if y%32 == 0 {
			if err := ctx.Err(); err != nil {
				return camera.CaptureResult{}, err
			}
		}
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x * 200 / w), G: uint8(y * 200 / h), B: 60 + shift, A: 255})
		}
	}
	for y := h / 4; y < 3*h/4; y++ {
		for x := w / 4; x < 3*w/4; x++ {
			if (x/(max(1, w/4))+y/(max(1, h/4))+req.Sequence)%3 == 0 {
				img.SetRGBA(x, y, color.RGBA{R: 245, G: 245, B: 245, A: 255})
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(req.OutputPath), 0o700); err != nil {
		return camera.CaptureResult{}, err
	}
	temp, err := os.CreateTemp(filepath.Dir(req.OutputPath), ".fake-capture-*.tmp")
	if err != nil {
		return camera.CaptureResult{}, err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := jpeg.Encode(temp, img, &jpeg.Options{Quality: 90}); err != nil {
		_ = temp.Close()
		return camera.CaptureResult{}, err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return camera.CaptureResult{}, err
	}
	if err := temp.Close(); err != nil {
		return camera.CaptureResult{}, err
	}
	if err := os.Rename(tempPath, req.OutputPath); err != nil {
		return camera.CaptureResult{}, err
	}
	return camera.CaptureResult{Sequence: req.Sequence, Path: req.OutputPath, Width: w, Height: h}, nil
}
