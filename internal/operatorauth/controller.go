package operatorauth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

const DefaultInactivityTimeout = 5 * time.Minute

type State struct {
	Mode              string `json:"mode"`
	Configured        bool   `json:"configured"`
	RetryAfterSeconds int    `json:"retry_after_seconds,omitempty"`
}

type Controller struct {
	mu                sync.Mutex
	path              string
	configured        bool
	mode              string
	lastActive        time.Time
	inactivityTimeout time.Duration
	expiryTimer       *time.Timer
	onLock            func()
	now               func() time.Time
}

func New(path string) *Controller {
	return &Controller{path: path, mode: "starting", inactivityTimeout: DefaultInactivityTimeout, now: time.Now}
}

func (c *Controller) SetOnLock(callback func()) {
	c.mu.Lock()
	c.onLock = callback
	c.mu.Unlock()
}

func (c *Controller) SetInactivityTimeout(timeout time.Duration) error {
	if timeout <= 0 {
		return errors.New("operator inactivity timeout must be positive")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.inactivityTimeout = timeout
	if c.mode == "operator" {
		c.armExpiryLocked()
	}
	return nil
}

func (c *Controller) Load() (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	store, err := readStore(c.path)
	if err != nil {
		c.mode = "startup_blocked"
		c.configured = false
		return State{Mode: c.mode}, err
	}
	c.configured = store != nil
	if c.configured {
		c.mode = "customer"
	} else {
		c.mode = "setup_required"
	}
	c.lastActive = time.Time{}
	return State{Mode: c.mode, Configured: c.configured}, nil
}

// Setup creates the first credential only. The store writer is exclusive and never overwrites.
func (c *Controller) Setup(pin string) error {
	if !validPIN(pin) {
		return ErrInvalidPIN
	}
	salt := make([]byte, argonSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("generate operator salt: %w", err)
	}
	verifier := argon2.IDKey([]byte(pin), salt, argonIterations, argonMemoryKiB, argonThreads, argonKeyBytes)
	now := c.now().UTC()
	store := credentialStore{
		Version: storeVersion, KDF: "argon2id",
		MemoryKiB: argonMemoryKiB, Iterations: argonIterations, Threads: argonThreads,
		Salt:      base64.RawStdEncoding.EncodeToString(salt),
		Verifier:  base64.RawStdEncoding.EncodeToString(verifier),
		UpdatedAt: now,
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	existing, err := readStore(c.path)
	if err != nil {
		return err
	}
	if existing != nil {
		return ErrAlreadyConfigured
	}
	if err := writeStore(c.path, store, false); err != nil {
		return err
	}
	c.configured = true
	c.revokeLocked("customer")
	return nil
}

// Reset changes the PIN only after checking the current PIN and persisted backoff.
func (c *Controller) Reset(currentPIN, newPIN string) error {
	if !validPIN(newPIN) {
		return ErrInvalidPIN
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	store, err := readStore(c.path)
	if err != nil {
		return err
	}
	if store == nil {
		return ErrNotConfigured
	}
	now := c.now()
	if now.Before(store.NextAllowedAt) {
		return ErrRateLimited
	}
	if !verifyPIN(currentPIN, *store) {
		if err := c.recordFailureLocked(store, now); err != nil {
			c.revokeLocked("startup_blocked")
			return err
		}
		c.revokeLocked("customer")
		return ErrAuthenticationFailed
	}
	salt := make([]byte, argonSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("generate new operator salt: %w", err)
	}
	verifier := argon2.IDKey([]byte(newPIN), salt, argonIterations, argonMemoryKiB, argonThreads, argonKeyBytes)
	replacement := credentialStore{
		Version: storeVersion, KDF: "argon2id",
		MemoryKiB: argonMemoryKiB, Iterations: argonIterations, Threads: argonThreads,
		Salt:      base64.RawStdEncoding.EncodeToString(salt),
		Verifier:  base64.RawStdEncoding.EncodeToString(verifier),
		UpdatedAt: now.UTC(),
	}
	if err := writeStore(c.path, replacement, true); err != nil {
		return err
	}
	c.configured = true
	c.revokeLocked("customer")
	return nil
}

// ResetFromOS replaces an existing credential for a caller that already
// controls the logged-in operator OS account. It is intentionally not exposed
// through Wails; the CLI recovery flow requires PhotoBooth to be closed.
func (c *Controller) ResetFromOS(newPIN string) error {
	if !validPIN(newPIN) {
		return ErrInvalidPIN
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	store, err := readStore(c.path)
	if err != nil {
		return err
	}
	if store == nil {
		return ErrNotConfigured
	}
	salt := make([]byte, argonSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("generate recovery salt: %w", err)
	}
	verifier := argon2.IDKey([]byte(newPIN), salt, argonIterations, argonMemoryKiB, argonThreads, argonKeyBytes)
	replacement := credentialStore{
		Version: storeVersion, KDF: "argon2id",
		MemoryKiB: argonMemoryKiB, Iterations: argonIterations, Threads: argonThreads,
		Salt:      base64.RawStdEncoding.EncodeToString(salt),
		Verifier:  base64.RawStdEncoding.EncodeToString(verifier),
		UpdatedAt: c.now().UTC(),
	}
	if err := writeStore(c.path, replacement, true); err != nil {
		return err
	}
	c.configured = true
	c.revokeLocked("customer")
	return nil
}

func (c *Controller) Authenticate(pin string) (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	store, err := readStore(c.path)
	if err != nil {
		c.revokeLocked("startup_blocked")
		c.configured = false
		return State{Mode: c.mode}, err
	}
	if store == nil {
		c.revokeLocked("setup_required")
		c.configured = false
		return State{Mode: c.mode}, ErrNotConfigured
	}
	c.configured = true
	now := c.now()
	if now.Before(store.NextAllowedAt) {
		return stateFor(store, "customer", now), ErrRateLimited
	}
	if !verifyPIN(pin, *store) {
		if err := c.recordFailureLocked(store, now); err != nil {
			c.revokeLocked("startup_blocked")
			return State{Mode: c.mode, Configured: true}, err
		}
		c.revokeLocked("customer")
		return stateFor(store, "customer", now), ErrAuthenticationFailed
	}
	store.FailedAttempts = 0
	store.NextAllowedAt = time.Time{}
	store.UpdatedAt = now.UTC()
	if err := writeStore(c.path, *store, true); err != nil {
		c.revokeLocked("startup_blocked")
		return State{Mode: c.mode, Configured: true}, fmt.Errorf("clear authentication backoff: %w", err)
	}
	c.configured = true
	c.mode = "operator"
	c.lastActive = now
	c.armExpiryLocked()
	return State{Mode: c.mode, Configured: true}, nil
}

func (c *Controller) Authorize() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if !c.configured || c.mode != "operator" {
		return ErrUnauthorized
	}
	if now.Sub(c.lastActive) >= c.inactivityTimeout {
		c.revokeLocked("customer")
		return ErrUnauthorized
	}
	c.lastActive = now
	c.armExpiryLocked()
	return nil
}

func (c *Controller) Status() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if c.mode == "operator" && now.Sub(c.lastActive) >= c.inactivityTimeout {
		c.revokeLocked("customer")
	}
	return State{Mode: c.mode, Configured: c.configured}
}

func (c *Controller) Lock() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.revokeLocked("customer")
	return State{Mode: c.mode, Configured: c.configured}
}

func (c *Controller) recordFailureLocked(store *credentialStore, now time.Time) error {
	if store.FailedAttempts < 1_000_000 {
		store.FailedAttempts++
	}
	store.NextAllowedAt = now.Add(backoff(store.FailedAttempts)).UTC()
	store.UpdatedAt = now.UTC()
	return writeStore(c.path, *store, true)
}

func (c *Controller) revokeLocked(nextMode string) {
	if c.expiryTimer != nil {
		c.expiryTimer.Stop()
		c.expiryTimer = nil
	}
	wasOperator := c.mode == "operator"
	c.mode = nextMode
	c.lastActive = time.Time{}
	if wasOperator && c.onLock != nil {
		callback := c.onLock
		go callback()
	}
}

func (c *Controller) armExpiryLocked() {
	if c.expiryTimer != nil {
		c.expiryTimer.Stop()
	}
	c.expiryTimer = time.AfterFunc(c.inactivityTimeout, c.expire)
}

func (c *Controller) expire() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.mode != "operator" {
		return
	}
	remaining := c.lastActive.Add(c.inactivityTimeout).Sub(c.now())
	if remaining > 0 {
		c.expiryTimer = time.AfterFunc(remaining, c.expire)
		return
	}
	c.revokeLocked("customer")
}

func backoff(failed uint32) time.Duration {
	delays := [...]time.Duration{
		2 * time.Second, 5 * time.Second, 15 * time.Second, 30 * time.Second,
		time.Minute, 2 * time.Minute, 5 * time.Minute, 10 * time.Minute, 15 * time.Minute,
	}
	index := int(failed) - 1
	if index < 0 {
		return 0
	}
	if index >= len(delays) {
		index = len(delays) - 1
	}
	return delays[index]
}

func stateFor(store *credentialStore, mode string, now time.Time) State {
	state := State{Mode: mode, Configured: store != nil}
	if store != nil && now.Before(store.NextAllowedAt) {
		state.RetryAfterSeconds = int(store.NextAllowedAt.Sub(now).Round(time.Second).Seconds())
		if state.RetryAfterSeconds < 1 {
			state.RetryAfterSeconds = 1
		}
	}
	return state
}

func verifyPIN(pin string, store credentialStore) bool {
	salt, err := base64.RawStdEncoding.DecodeString(store.Salt)
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(store.Verifier)
	if err != nil {
		return false
	}
	candidate := pin
	validFormat := validPIN(pin)
	if !validFormat {
		candidate = "invalid-pin"
	}
	actual := argon2.IDKey([]byte(candidate), salt, store.Iterations, store.MemoryKiB, store.Threads, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1 && validFormat
}
