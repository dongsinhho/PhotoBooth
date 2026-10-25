package main

import "testing"

func TestDemoSessionLifecycle(t *testing.T) {
	app := NewApp()
	started := app.StartDemoSession()
	if started.ID == "" || started.State != "capturing" || started.Total != 3 || started.Captured != 0 {
		t.Fatalf("unexpected start state: %+v", started)
	}

	for shot := 1; shot <= 3; shot++ {
		got, err := app.RecordDemoShot(started.ID)
		if err != nil {
			t.Fatalf("RecordDemoShot %d: %v", shot, err)
		}
		if got.Captured != shot {
			t.Fatalf("captured=%d, want %d", got.Captured, shot)
		}
	}
	if app.session.State != "ready_to_finish" {
		t.Fatalf("state=%q, want ready_to_finish", app.session.State)
	}
	if _, err := app.RecordDemoShot(started.ID); err == nil {
		t.Fatal("expected fourth shot to be rejected")
	}
	finished, err := app.FinishDemoSession(started.ID)
	if err != nil {
		t.Fatalf("FinishDemoSession: %v", err)
	}
	if finished.State != "completed" || finished.Captured != 3 {
		t.Fatalf("unexpected finished state: %+v", finished)
	}
	reset := app.ResetDemoSession()
	if reset.State != "idle" || reset.Captured != 0 || reset.ID != "" {
		t.Fatalf("unexpected reset state: %+v", reset)
	}
}

func TestDemoSessionRejectsWrongSessionID(t *testing.T) {
	app := NewApp()
	app.StartDemoSession()
	if _, err := app.RecordDemoShot("not-the-session"); err == nil {
		t.Fatal("expected wrong session ID to be rejected")
	}
	if _, err := app.FinishDemoSession("not-the-session"); err == nil {
		t.Fatal("expected wrong session ID to be rejected for finish")
	}
}

func TestPingEchoesSequenceAndBenchmarkModeDefaultsOff(t *testing.T) {
	app := NewApp()
	if got := app.Ping(42); got != 42 {
		t.Fatalf("Ping returned %d, want 42", got)
	}
	if app.IsBenchmarkMode() {
		t.Fatal("benchmark mode must be disabled unless explicitly requested")
	}
}
