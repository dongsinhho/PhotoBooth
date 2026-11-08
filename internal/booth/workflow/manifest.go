package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
	"github.com/dongsinhho/PhotoBooth/internal/booth/session"
)

// NewSessionManifest fingerprints the frame definition and capture-affecting
// configuration. It does not access the camera or create session files.
func normalizedMediaDeliveryMode(mode string) string {
	if strings.TrimSpace(mode) == "" {
		return stationconfig.MediaDeliveryModeLocal
	}
	return strings.ToLower(strings.TrimSpace(mode))
}

func normalizedPaymentMode(mode string) string {
	if strings.TrimSpace(mode) == "" {
		return stationconfig.PaymentModeDisabled
	}
	return strings.ToLower(strings.TrimSpace(mode))
}

func NewSessionManifest(cfg Config) (session.Manifest, error) {
	frameBytes, err := readFrameConfigBytes(cfg.FrameConfigPath)
	if err != nil {
		return session.Manifest{}, fmt.Errorf("read frame config for session identity: %w", err)
	}
	return NewSessionManifestFromFrameBytes(cfg, frameBytes)
}

func readFrameConfigBytes(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 1<<20 {
		return nil, fmt.Errorf("frame config must be a regular file between 1 byte and %d bytes", 1<<20)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, fmt.Errorf("frame config identity changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > 1<<20 {
		return nil, fmt.Errorf("frame config exceeds %d bytes", 1<<20)
	}
	return data, nil
}

// NewSessionManifestFromFrameBytes fingerprints the exact frame bytes being
// used for validation/rendering while retaining the original configured path
// as part of the run identity. Recovery can therefore render from frame.json
// even when the original frame file has moved or been removed.
func NewSessionManifestFromFrameBytes(cfg Config, frameBytes []byte) (session.Manifest, error) {
	if !validSessionID(cfg.SessionID) {
		return session.Manifest{}, fmt.Errorf("session ID is invalid")
	}
	identityPath := cfg.FrameConfigIdentityPath
	if identityPath == "" {
		identityPath = cfg.FrameConfigPath
	}
	framePath, err := filepath.Abs(identityPath)
	if err != nil {
		return session.Manifest{}, fmt.Errorf("resolve frame config path: %w", err)
	}
	if len(frameBytes) == 0 || len(frameBytes) > 1<<20 {
		return session.Manifest{}, fmt.Errorf("frame config must be between 1 byte and %d bytes", 1<<20)
	}
	frameDigest := sha256.Sum256(frameBytes)
	frameHash := hex.EncodeToString(frameDigest[:])
	fingerprintInput := struct {
		ConfigSchemaVersion int
		FrameConfigPath     string
		FrameConfigSHA256   string
		CaptureCount        int
		CountdownNanos      int64
		IntervalNanos       int64
		CaptureWidth        int
		CaptureHeight       int
		CameraProvider      string
		CameraDevice        string
		CameraFPS           float64
		DemoMode            bool
		MediaDeliveryMode   string
		PaymentMode         string
	}{
		ConfigSchemaVersion: cfg.ConfigSchemaVersion,
		FrameConfigPath:     filepath.Clean(framePath),
		FrameConfigSHA256:   frameHash,
		CaptureCount:        cfg.CaptureCount,
		CountdownNanos:      int64(cfg.Countdown),
		IntervalNanos:       int64(cfg.Interval),
		CaptureWidth:        cfg.CaptureWidth,
		CaptureHeight:       cfg.CaptureHeight,
		CameraProvider:      cfg.CameraProvider,
		CameraDevice:        cfg.CameraDevice,
		CameraFPS:           cfg.CameraFPS,
		DemoMode:            cfg.DemoMode,
		MediaDeliveryMode:   normalizedMediaDeliveryMode(cfg.MediaDeliveryMode),
		PaymentMode:         normalizedPaymentMode(cfg.PaymentMode),
	}
	encoded, err := json.Marshal(fingerprintInput)
	if err != nil {
		return session.Manifest{}, fmt.Errorf("encode session config fingerprint: %w", err)
	}
	fingerprint := sha256.Sum256(encoded)
	manifest := session.NewManifest(
		cfg.SessionID,
		cfg.CaptureCount,
		filepath.Clean(framePath),
		frameHash,
		hex.EncodeToString(fingerprint[:]),
	)
	manifest.ConfigSchemaVersion = cfg.ConfigSchemaVersion
	manifest.CameraProvider = cfg.CameraProvider
	manifest.CameraDevice = cfg.CameraDevice
	manifest.CaptureWidth = cfg.CaptureWidth
	manifest.CaptureHeight = cfg.CaptureHeight
	manifest.CountdownMilliseconds = cfg.Countdown.Milliseconds()
	manifest.IntervalMilliseconds = cfg.Interval.Milliseconds()
	manifest.DemoMode = cfg.DemoMode
	manifest.MediaDeliveryMode = normalizedMediaDeliveryMode(cfg.MediaDeliveryMode)
	manifest.PaymentMode = normalizedPaymentMode(cfg.PaymentMode)
	if manifest.MediaDeliveryMode != stationconfig.MediaDeliveryModeLocal {
		return session.Manifest{}, fmt.Errorf("unsupported media delivery mode %q", cfg.MediaDeliveryMode)
	}
	if manifest.PaymentMode != stationconfig.PaymentModeDisabled {
		return session.Manifest{}, fmt.Errorf("unsupported payment mode %q", cfg.PaymentMode)
	}
	return manifest, nil
}
