package payment

import (
	"context"
	"errors"
	"fmt"
	"strings"

	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
)

// Request is stable across retries. A future provider-backed gate must treat
// SessionID as an idempotency key and verify an existing permit on resume;
// it must never charge again merely because capture is being resumed.
type Request struct {
	SessionID string
	DemoMode  bool
	Resume    bool
}

// Gate is checked by the workflow before it creates a session workspace or
// opens the camera. Capture code has no dependency on a concrete provider.
type Gate interface {
	EnsureAuthorized(context.Context, Request) error
}

// DisabledGate implements the free/no-payment mode without network calls.
type DisabledGate struct{}

func (DisabledGate) EnsureAuthorized(ctx context.Context, request Request) error {
	if ctx == nil {
		return errors.New("payment authorization context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validSessionID(request.SessionID) {
		return errors.New("payment authorization requires a valid session ID")
	}
	return nil
}

// NewGate resolves the configured payment policy. Only disabled/free is
// implemented today; unknown modes fail closed.
func NewGate(settings stationconfig.PaymentSettings) (Gate, error) {
	mode := strings.ToLower(strings.TrimSpace(settings.Mode))
	switch mode {
	case stationconfig.PaymentModeDisabled:
		return DisabledGate{}, nil
	default:
		return nil, fmt.Errorf("unsupported payment mode %q", settings.Mode)
	}
}

func validSessionID(value string) bool {
	if len(value) != 32 {
		return false
	}
	for _, character := range value {
		if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}
