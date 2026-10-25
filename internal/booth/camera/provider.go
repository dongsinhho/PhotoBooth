package camera

import "context"

// Provider isolates camera-specific APIs from session orchestration.
type Provider interface {
	Info() Info
	Open(ctx context.Context) error
	Close() error
	Capture(ctx context.Context, req CaptureRequest) (CaptureResult, error)
}

type Info struct {
	ID           string
	Name         string
	OS           string
	Backend      string
	Capabilities Capabilities
}

type Capabilities struct {
	Preview       bool
	StillCapture  bool
	RemoteTrigger bool
	Resolution    []Resolution
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
