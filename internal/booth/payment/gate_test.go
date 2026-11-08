package payment

import (
	"context"
	"testing"

	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
)

func TestNewGateBuildsExplicitDisabledPolicy(t *testing.T) {
	gate, err := NewGate(stationconfig.PaymentSettings{Mode: stationconfig.PaymentModeDisabled})
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}
	if _, ok := gate.(DisabledGate); !ok {
		t.Fatalf("gate=%T, want DisabledGate", gate)
	}
	if err := gate.EnsureAuthorized(context.Background(), Request{
		SessionID: "0123456789abcdef0123456789abcdef",
	}); err != nil {
		t.Fatalf("disabled gate: %v", err)
	}
}

func TestDisabledGateHonorsCancellationAndRejectsInvalidRequest(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := (DisabledGate{}).EnsureAuthorized(cancelled, Request{
		SessionID: "0123456789abcdef0123456789abcdef",
	}); err == nil {
		t.Fatal("cancelled context must not pass the payment gate")
	}
	if err := (DisabledGate{}).EnsureAuthorized(context.Background(), Request{}); err == nil {
		t.Fatal("invalid session identity must not pass the payment gate")
	}
}

func TestNewGateRejectsUnimplementedPaymentMode(t *testing.T) {
	if _, err := NewGate(stationconfig.PaymentSettings{Mode: "qr"}); err == nil {
		t.Fatal("QR payment must not silently fall back to disabled/free mode")
	}
}
