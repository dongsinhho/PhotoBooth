package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestPlayerSessionJSONUsesCustomerSafeProjection(t *testing.T) {
	session := PlayerSession{
		ID:              "internal-session-secret",
		CustomerToken:   "customer-token-capability",
		State:           "failed",
		Captured:        3,
		Total:           3,
		Message:         "contains private path C:\\private\\photos",
		ErrorMessage:    "printer queue secret",
		CameraMode:      "Windows webcam",
		OutputPath:      "C:\\private\\photos\\final.jpg",
		RenderedPath:    "C:\\private\\sessions\\rendered.jpg",
		OutputTargets:   []PlayerOutputTargetView{{Name: "printer", JobID: "spooler-job-secret"}},
		PreviewDataURL:  "data:image/jpeg;base64,AA==",
		CapturePreviews: []string{"data:image/jpeg;base64,AQ=="},
	}
	data, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	raw := string(data)
	for _, secret := range []string{
		"internal-session-secret",
		"C:\\\\private\\\\photos",
		"printer queue secret",
		"spooler-job-secret",
		"output_path",
		"rendered_path",
		"output_targets",
		"error_message",
	} {
		if strings.Contains(raw, secret) {
			t.Fatalf("customer JSON contains private field %q: %s", secret, raw)
		}
	}
	for _, expected := range []string{
		"customer-token-capability",
		"output_saved",
		"output_retry_available",
		"capture_previews",
	} {
		if !strings.Contains(raw, expected) {
			t.Fatalf("customer JSON is missing %q: %s", expected, raw)
		}
	}
	if strings.Contains(raw, "contains private path") {
		t.Fatalf("customer JSON exposed raw message: %s", raw)
	}
}

func TestCustomerTokenResolvesOnlyInsideWailsRuntime(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()
	app.player = PlayerSession{
		ID:            "actual-internal-session-id",
		CustomerToken: "c0ffee-random-capability",
	}
	if _, err := app.resolveCustomerSessionToken("actual-internal-session-id"); err == nil {
		t.Fatal("internal session ID was accepted as a customer token")
	}
	got, err := app.resolveCustomerSessionToken("c0ffee-random-capability")
	if err != nil {
		t.Fatal(err)
	}
	if got != "actual-internal-session-id" {
		t.Fatalf("resolved ID = %q, want internal session ID", got)
	}
	if _, err := app.resolveCustomerSessionToken("stale-token"); err == nil {
		t.Fatal("stale customer token was accepted")
	}
}
