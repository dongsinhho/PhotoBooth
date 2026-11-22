package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/dongsinhho/PhotoBooth/internal/operatorauth"
)

func TestOperatorBindingsRejectCallsBeforeAuthentication(t *testing.T) {
	app := NewApp()
	app.playerDataDir = t.TempDir()

	controller := operatorauth.New(operatorauth.StorePath(app.playerDataDir))
	if _, err := controller.Load(); err != nil {
		t.Fatal(err)
	}
	if err := controller.Setup("12345678"); err != nil {
		t.Fatal(err)
	}
	app.operatorAccess = controller

	if _, err := app.GetPlayerSettings(); !errors.Is(err, operatorauth.ErrUnauthorized) {
		t.Fatalf("GetPlayerSettings before login error = %v, want ErrUnauthorized", err)
	}

	if err := app.AuthenticateOperator("12345678"); err != nil {
		t.Fatal(err)
	}
	if state := controller.Status(); state.Mode != "operator" {
		t.Fatalf("authenticated mode = %q, want operator", state.Mode)
	}
	if _, err := app.GetPlayerSettings(); err != nil {
		t.Fatalf("GetPlayerSettings after login: %v", err)
	}

	if err := app.LockOperatorMode(); err != nil {
		t.Fatal(err)
	}
	if _, err := app.GetPlayerSettings(); !errors.Is(err, operatorauth.ErrUnauthorized) {
		t.Fatalf("GetPlayerSettings after lock error = %v, want ErrUnauthorized", err)
	}
}

func TestOperatorCredentialPathUsesStationDataDirectory(t *testing.T) {
	dataDir := t.TempDir()
	path := operatorauth.StorePath(dataDir)
	if filepath.Dir(path) != dataDir {
		t.Fatalf("credential path %q is not in station data dir %q", path, dataDir)
	}
}

func TestCustomerWorkflowCannotStartWhileOperatorModeIsActive(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()
	app.playerDataDir = t.TempDir()
	controller := operatorauth.New(operatorauth.StorePath(app.playerDataDir))
	if _, err := controller.Load(); err != nil {
		t.Fatal(err)
	}
	if err := controller.Setup("12345678"); err != nil {
		t.Fatal(err)
	}
	app.operatorAccess = controller
	if err := app.AuthenticateOperator("12345678"); err != nil {
		t.Fatal(err)
	}

	if _, err := app.StartDemoPhotoSession(); err == nil {
		t.Fatal("customer capture started while Operator Mode was active")
	}
	if _, err := app.StartCameraPreview(); err == nil {
		t.Fatal("customer camera preview started while Operator Mode was active")
	}
}

func TestOperatorAuthenticationWaitsForGuestWorkflowQuiescence(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()
	app.playerDataDir = t.TempDir()
	controller := operatorauth.New(operatorauth.StorePath(app.playerDataDir))
	if _, err := controller.Load(); err != nil {
		t.Fatal(err)
	}
	if err := controller.Setup("12345678"); err != nil {
		t.Fatal(err)
	}
	app.operatorAccess = controller
	app.playerRunning = true

	if err := app.AuthenticateOperator("12345678"); !errors.Is(err, ErrOperatorTransitionBusy) {
		t.Fatalf("authentication during guest session error = %v, want ErrOperatorTransitionBusy", err)
	}
	if state := controller.Status(); state.Mode == "operator" {
		t.Fatal("Operator Mode was granted while a guest session was running")
	}
}

func TestLegacyDemoBindingsAreOperatorOnly(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()
	app.playerDataDir = t.TempDir()
	controller := operatorauth.New(operatorauth.StorePath(app.playerDataDir))
	if _, err := controller.Load(); err != nil {
		t.Fatal(err)
	}
	if err := controller.Setup("12345678"); err != nil {
		t.Fatal(err)
	}
	app.operatorAccess = controller

	if got := app.StartDemoSession(); got.ID != "" {
		t.Fatalf("StartDemoSession exposed an unauthenticated demo session: %+v", got)
	}
	if _, err := app.SimulateCameraDisconnect("anything"); !errors.Is(err, operatorauth.ErrUnauthorized) {
		t.Fatalf("SimulateCameraDisconnect error = %v, want ErrUnauthorized", err)
	}
	if _, err := app.ContinueDemoSession("anything"); !errors.Is(err, operatorauth.ErrUnauthorized) {
		t.Fatalf("ContinueDemoSession error = %v, want ErrUnauthorized", err)
	}
	if _, err := app.RestartDemoSession("anything"); !errors.Is(err, operatorauth.ErrUnauthorized) {
		t.Fatalf("RestartDemoSession error = %v, want ErrUnauthorized", err)
	}
	if _, err := app.RecordDemoShot("anything"); !errors.Is(err, operatorauth.ErrUnauthorized) {
		t.Fatalf("RecordDemoShot error = %v, want ErrUnauthorized", err)
	}
	if _, err := app.FinishDemoSession("anything"); !errors.Is(err, operatorauth.ErrUnauthorized) {
		t.Fatalf("FinishDemoSession error = %v, want ErrUnauthorized", err)
	}
	if got := app.ResetDemoSession(); got.ID != "" {
		t.Fatalf("ResetDemoSession exposed demo state without authentication: %+v", got)
	}
}

func TestOperatorAuthenticationDoesNotDiscloseCredentialPresence(t *testing.T) {
	unconfiguredApp := NewApp()
	unconfiguredApp.operatorAccess = operatorauth.New(operatorauth.StorePath(t.TempDir()))
	if _, err := unconfiguredApp.operatorAccess.Load(); err != nil {
		t.Fatal(err)
	}
	unconfiguredErr := unconfiguredApp.AuthenticateOperator("00000000")
	if !errors.Is(unconfiguredErr, ErrOperatorAuthenticationFailed) {
		t.Fatalf("unconfigured auth error = %v, want generic auth failure", unconfiguredErr)
	}

	configuredApp := NewApp()
	configuredApp.operatorAccess = operatorauth.New(operatorauth.StorePath(t.TempDir()))
	if _, err := configuredApp.operatorAccess.Load(); err != nil {
		t.Fatal(err)
	}
	if err := configuredApp.operatorAccess.Setup("12345678"); err != nil {
		t.Fatal(err)
	}
	configuredErr := configuredApp.AuthenticateOperator("00000000")
	if !errors.Is(configuredErr, ErrOperatorAuthenticationFailed) {
		t.Fatalf("wrong-PIN auth error = %v, want generic auth failure", configuredErr)
	}
	if unconfiguredErr.Error() != configuredErr.Error() {
		t.Fatalf("credential state leaked through errors: unconfigured=%q configured=%q", unconfiguredErr, configuredErr)
	}
}
