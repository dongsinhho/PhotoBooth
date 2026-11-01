package camera

import (
	"context"
	"errors"
)

// ErrStreamUnavailable indicates that a provider could not read from its active
// camera stream. A recovery wrapper may close and reopen the provider before
// retrying a capture. Argument, file-output and image-decoding errors should
// not be wrapped with this sentinel.
var ErrStreamUnavailable = errors.New("camera stream unavailable")

// Provider isolates camera-specific APIs from session orchestration.
type Provider interface {
	Info() Info
	Open(ctx context.Context) error
	Close() error
	Capture(ctx context.Context, req CaptureRequest) (CaptureResult, error)
}

// PreviewProvider is an optional extension for providers that can return one
// compressed frame from their active stream without writing a capture artifact.
// Preview and Capture must share the provider's stream/worker; implementations
// must serialize access so callers never create a second native reader.
type PreviewProvider interface {
	Preview(ctx context.Context) ([]byte, error)
}

type Info struct {
	ID           string
	Name         string
	OS           string
	Backend      string
	Capabilities Capabilities
}

type Capabilities struct {
	Preview                bool
	FrameCaptureFromStream bool
	StillCapture           bool
	RemoteTrigger          bool
	Resolution             []Resolution
}

type Resolution struct{ Width, Height int }

type CaptureRequest struct {
	Sequence   int
	Width      int
	Height     int
	OutputPath string
}

type CaptureResult struct {
	Sequence int
	Path     string
	Width    int
	Height   int
}
