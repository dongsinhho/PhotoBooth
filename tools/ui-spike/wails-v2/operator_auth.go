package main

import (
	"errors"
	"fmt"

	"github.com/dongsinhho/PhotoBooth/internal/operatorauth"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	ErrOperatorTransitionBusy       = errors.New("finish the active photo/output operation before entering Operator Mode")
	ErrOperatorAuthenticationFailed = errors.New("operator authentication failed")
)

func (a *App) operatorController() *operatorauth.Controller {
	a.operatorMu.RLock()
	defer a.operatorMu.RUnlock()
	return a.operatorAccess
}

func (a *App) requireCustomer() error {
	controller := a.operatorController()
	if controller == nil {
		if a.ctx == nil {
			return nil
		}
		if a.operatorInitErr != nil {
			return fmt.Errorf("station startup is not ready: %w", a.operatorInitErr)
		}
		return nil
	}
	state := controller.Status()
	if state.Mode == "operator" {
		return errors.New("return to Customer Mode before starting or controlling a guest session")
	}
	if state.Mode == "startup_blocked" {
		return errors.New("PhotoBooth requires operator service before accepting customers")
	}
	return nil
}

func (a *App) requireOperator() error {
	controller := a.operatorController()
	if controller == nil {
		// Direct App unit tests do not have a Wails host or external bindings.
		// A running host must fail closed if startup could not initialize auth.
		if a.ctx == nil {
			return nil
		}
		if a.operatorInitErr != nil {
			return fmt.Errorf("operator authorization is unavailable: %w", a.operatorInitErr)
		}
		return operatorauth.ErrUnauthorized
	}
	return controller.Authorize()
}

func (a *App) AuthenticateOperator(pin string) error {
	controller := a.operatorController()
	if controller == nil {
		return ErrOperatorAuthenticationFailed
	}

	// Serialize the entire transition with camera/session startup. Without
	// this lock, a guest could start capture between the idle check and PIN
	// verification, causing Operator Mode to open during a live session.
	a.cameraLifecycleMu.Lock()
	defer a.cameraLifecycleMu.Unlock()
	if err := a.operatorTransitionReadyLocked(); err != nil {
		return err
	}
	if _, err := controller.Authenticate(pin); err != nil {
		// Wrong PIN, unconfigured store, corrupt store and persisted backoff
		// share one binding error so Customer Mode cannot infer credential state.
		return ErrOperatorAuthenticationFailed
	}
	if err := a.stopCameraPreviewLocked(); err != nil {
		controller.Lock()
		return fmt.Errorf("camera preview could not be released; Operator Mode was not opened: %w", err)
	}
	return nil
}

func (a *App) LockOperatorMode() error {
	a.cameraLifecycleMu.Lock()
	defer a.cameraLifecycleMu.Unlock()
	controller := a.operatorController()
	if controller != nil {
		controller.Lock()
	}
	return nil
}

func (a *App) TouchOperatorActivity() error {
	return a.requireOperator()
}

func (a *App) ChangeOperatorPIN(currentPIN, newPIN string) error {
	if err := a.requireOperator(); err != nil {
		return err
	}
	controller := a.operatorController()
	if controller == nil {
		return operatorauth.ErrUnauthorized
	}
	if err := controller.Reset(currentPIN, newPIN); err != nil {
		return err
	}
	return nil
}

func (a *App) ExitPhotoBooth() error {
	if err := a.requireOperator(); err != nil {
		return err
	}
	if a.ctx == nil {
		return errors.New("application runtime is not ready")
	}

	a.cameraLifecycleMu.Lock()
	if err := a.requireOperator(); err != nil {
		a.cameraLifecycleMu.Unlock()
		return err
	}
	a.mu.Lock()
	busy := a.playerRunning || a.outputRetrying || a.settingsSaving || a.retentionBusy
	a.mu.Unlock()
	if busy {
		a.cameraLifecycleMu.Unlock()
		return ErrOperatorTransitionBusy
	}
	if err := a.stopCameraPreviewLocked(); err != nil {
		a.cameraLifecycleMu.Unlock()
		return fmt.Errorf("cannot exit while camera preview is not fully released: %w", err)
	}
	a.cameraLifecycleMu.Unlock()

	if controller := a.operatorController(); controller != nil {
		controller.Lock()
	}
	wailsruntime.Quit(a.ctx)
	return nil
}

// operatorTransitionReadyLocked must be called while cameraLifecycleMu is held.
func (a *App) operatorTransitionReadyLocked() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.playerRunning || a.outputRetrying || a.settingsSaving || a.retentionBusy {
		return ErrOperatorTransitionBusy
	}
	switch a.player.State {
	case "", "idle", "completed", "partial_success", "failed", "timed_out", "cancelled":
		return nil
	default:
		return ErrOperatorTransitionBusy
	}
}

func (a *App) operatorModeLocked() {
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "operator-mode-locked")
	}
}
