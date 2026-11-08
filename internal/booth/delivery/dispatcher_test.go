package delivery

import (
	"testing"

	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
	"github.com/dongsinhho/PhotoBooth/internal/booth/output"
)

func TestNewDispatcherBuildsLocalDeliveryWithoutChangingWorkflowContract(t *testing.T) {
	dispatcher, err := NewDispatcher(
		stationconfig.MediaDeliverySettings{Mode: stationconfig.MediaDeliveryModeLocal},
		t.TempDir(),
	)
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}
	local, ok := dispatcher.(interface{ Name() string })
	if !ok || local.Name() != output.TargetLocalStorage {
		t.Fatalf("dispatcher=%T, want local storage dispatcher", dispatcher)
	}
}

func TestNewDispatcherRejectsMissingLocalRoot(t *testing.T) {
	if _, err := NewDispatcher(
		stationconfig.MediaDeliverySettings{Mode: stationconfig.MediaDeliveryModeLocal},
		"",
	); err == nil {
		t.Fatal("expected local delivery without an output directory to fail")
	}
}

func TestNewDispatcherRejectsUnsupportedMode(t *testing.T) {
	if _, err := NewDispatcher(stationconfig.MediaDeliverySettings{Mode: "lan"}, t.TempDir()); err == nil {
		t.Fatal("unsupported LAN mode must not silently fall back to local delivery")
	}
}
