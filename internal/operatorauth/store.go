package operatorauth

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	storeVersion           = 1
	storeFileName          = "operator-auth-v1.json"
	maxStoreBytes          = 4 * 1024
	argonMemoryKiB  uint32 = 64 * 1024
	argonIterations uint32 = 3
	argonThreads    uint8  = 2
	argonKeyBytes   uint32 = 32
	argonSaltBytes         = 16
)

var (
	ErrNotConfigured        = errors.New("operator access is not configured")
	ErrAlreadyConfigured    = errors.New("operator access is already configured")
	ErrInvalidPIN           = errors.New("operator PIN must contain exactly eight digits")
	ErrAuthenticationFailed = errors.New("operator authentication failed")
	ErrRateLimited          = errors.New("operator authentication is temporarily rate-limited")
	ErrUnauthorized         = errors.New("operator authentication is required")
	ErrInvalidStore         = errors.New("operator credential store is invalid")
)

type credentialStore struct {
	Version        int       `json:"version"`
	KDF            string    `json:"kdf"`
	MemoryKiB      uint32    `json:"memory_kib"`
	Iterations     uint32    `json:"iterations"`
	Threads        uint8     `json:"threads"`
	Salt           string    `json:"salt"`
	Verifier       string    `json:"verifier"`
	FailedAttempts uint32    `json:"failed_attempts"`
	NextAllowedAt  time.Time `json:"next_allowed_at,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func StorePath(dataDir string) string { return filepath.Join(dataDir, storeFileName) }

func readStore(path string) (*credentialStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("operator credential path is not configured")
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect operator credential store: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxStoreBytes {
		return nil, ErrInvalidStore
	}
	if err := validateFilePermissions(info); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read operator credential store: %w", err)
	}
	var store credentialStore
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&store); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidStore, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, ErrInvalidStore
	}
	if err := validateStore(store); err != nil {
		return nil, err
	}
	return &store, nil
}

func validateStore(store credentialStore) error {
	if store.Version != storeVersion || store.KDF != "argon2id" ||
		store.MemoryKiB != argonMemoryKiB || store.Iterations != argonIterations ||
		store.Threads != argonThreads || store.FailedAttempts > 1_000_000 || store.UpdatedAt.IsZero() {
		return ErrInvalidStore
	}
	salt, err := base64.RawStdEncoding.DecodeString(store.Salt)
	if err != nil || len(salt) != argonSaltBytes {
		return ErrInvalidStore
	}
	verifier, err := base64.RawStdEncoding.DecodeString(store.Verifier)
	if err != nil || len(verifier) != int(argonKeyBytes) {
		return ErrInvalidStore
	}
	return nil
}

func writeStore(path string, store credentialStore, replace bool) error {
	if err := validateStore(store); err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("encode operator credential store: %w", err)
	}
	if len(data) > maxStoreBytes {
		return ErrInvalidStore
	}
	if err := atomicWrite(path, data, 0o600, replace); err != nil {
		return fmt.Errorf("persist operator credential store: %w", err)
	}
	return nil
}

func validPIN(pin string) bool {
	if len(pin) != 8 {
		return false
	}
	for _, digit := range pin {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}
