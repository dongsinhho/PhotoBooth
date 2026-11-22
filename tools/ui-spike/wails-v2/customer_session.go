package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
)

type customerPlayerSession struct {
	Token                string   `json:"token,omitempty"`
	State                string   `json:"state"`
	Captured             int      `json:"captured"`
	Total                int      `json:"total"`
	CurrentShot          int      `json:"current_shot"`
	CountdownSeconds     int      `json:"countdown_seconds"`
	Message              string   `json:"message"`
	CameraMode           string   `json:"camera_mode"`
	OutputSaved          bool     `json:"output_saved"`
	OutputRetrying       bool     `json:"output_retrying,omitempty"`
	OutputRetryAvailable bool     `json:"output_retry_available"`
	PreviewDataURL       string   `json:"preview_data_url,omitempty"`
	CapturePreviews      []string `json:"capture_previews,omitempty"`
	DemoMode             bool     `json:"demo_mode"`
}

// MarshalJSON is the sole WebView projection for the internal PlayerSession.
// It intentionally omits real session IDs, filesystem paths, raw errors and
// printer queue/job data. Those stay inside Go for recovery and operator work.
func (s PlayerSession) MarshalJSON() ([]byte, error) {
	outputRetryAvailable := s.RenderedPath != "" && s.OutputPath == "" &&
		s.Total > 0 && s.Captured >= s.Total &&
		(s.State == "failed" || s.State == "partial_success" || s.State == "timed_out")
	return json.Marshal(customerPlayerSession{
		Token:                s.CustomerToken,
		State:                s.State,
		Captured:             s.Captured,
		Total:                s.Total,
		CurrentShot:          s.CurrentShot,
		CountdownSeconds:     s.CountdownSeconds,
		Message:              customerSafeMessage(s),
		CameraMode:           s.CameraMode,
		OutputSaved:          s.OutputPath != "",
		OutputRetrying:       s.OutputRetrying,
		OutputRetryAvailable: outputRetryAvailable,
		PreviewDataURL:       s.PreviewDataURL,
		CapturePreviews:      s.CapturePreviews,
		DemoMode:             s.DemoMode,
	})
}

func customerSafeMessage(s PlayerSession) string {
	switch s.State {
	case "idle":
		return "Ready to take photos."
	case "preflight":
		return "Preparing your photo session."
	case "capturing":
		if s.CountdownSeconds > 0 {
			return "Get ready for your next photo."
		}
		return "Taking your photos."
	case "reconnecting":
		return "The camera disconnected. Reconnecting to continue your photos."
	case "rendered", "rendering":
		return "Preparing your photo strip."
	case "cancelling":
		return "Cancelling this photo session."
	case "completed":
		return "Your photo strip is ready."
	case "partial_success":
		return "Your photo is saved, but another output may need operator attention."
	case "failed", "timed_out":
		return "This photo session could not be completed. Please ask the booth operator for help."
	case "cancelled":
		return "Photo session cancelled."
	default:
		return "Your photo session is in progress."
	}
}

func newCustomerSessionToken() (string, error) {
	var token [24]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(token[:]), nil
}

func (a *App) resolveCustomerSessionToken(token string) (string, error) {
	if a.ctx == nil {
		// Direct Go tests use internal IDs. A running Wails host must present
		// the in-memory random token exposed by the current customer projection.
		return token, nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.player.ID == "" && token == "" {
		return "", nil
	}
	if token == "" || token != a.player.CustomerToken || a.player.ID == "" {
		return "", errors.New("photo session token is invalid or expired")
	}
	return a.player.ID, nil
}
