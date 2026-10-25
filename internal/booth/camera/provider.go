package camera

import "context"

// Provider is the stable boundary between booth orchestration and camera-specific APIs.
// Implementations may wrap UVC/OpenCV, vendor SDKs, or another capture mechanism.
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

type Resolution struct {
	Width  int
	Height int
}

type CaptureRequest struct {
	Width  int
	Height int
}

type CaptureResult struct {
	Sequence int
	Path     string
	Width    int
	Height   int
}
