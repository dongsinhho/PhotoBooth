package main

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
	"github.com/dongsinhho/PhotoBooth/internal/booth/output"
	"github.com/dongsinhho/PhotoBooth/internal/booth/session"
	"github.com/dongsinhho/PhotoBooth/internal/booth/workflow"
)

func TestPlayerOutputDispatcherKeepsPrintingDisabledByDefault(t *testing.T) {
	cfg := stationconfig.Defaults()
	root := t.TempDir()
	dispatcher, err := newPlayerOutputDispatcher(cfg, filepath.Join(root, "sessions"), filepath.Join(root, "photos"), false)
	if err != nil {
		t.Fatalf("newPlayerOutputDispatcher: %v", err)
	}
	local, ok := dispatcher.(*output.LocalProvider)
	if !ok || local.Name() != output.TargetLocalStorage {
		t.Fatalf("default dispatcher=%T, want local-only provider", dispatcher)
	}
}

func TestPlayerOutputDispatcherNeverPrintsDemoSessions(t *testing.T) {
	cfg := stationconfig.Defaults()
	cfg.Printer = stationconfig.PrinterSettings{
		Enabled: true, QueueName: "DS-RX1", MediaForm: "6x4",
		Orientation: stationconfig.PrinterOrientationLandscape,
		Transform:   stationconfig.PrinterTransformSettings{FlipVertical: true},
	}
	root := t.TempDir()
	dispatcher, err := newPlayerOutputDispatcher(cfg, filepath.Join(root, "sessions"), filepath.Join(root, "photos"), true)
	if err != nil {
		t.Fatalf("demo dispatcher: %v", err)
	}
	if _, ok := dispatcher.(*output.LocalProvider); !ok {
		t.Fatalf("demo dispatcher=%T; demo must never include a printer", dispatcher)
	}
}

func TestPlayerOutputDispatcherUsesDurableWindowsPrinterOnlyWhenEnabled(t *testing.T) {
	cfg := stationconfig.Defaults()
	cfg.Printer = stationconfig.PrinterSettings{
		Enabled: true, QueueName: "DS-RX1", MediaForm: "6x4",
		Orientation: stationconfig.PrinterOrientationLandscape,
		Transform:   stationconfig.PrinterTransformSettings{FlipVertical: true},
	}
	root := t.TempDir()
	dispatcher, err := newPlayerOutputDispatcher(cfg, filepath.Join(root, "sessions"), filepath.Join(root, "photos"), false)
	if runtime.GOOS != "windows" {
		if err == nil {
			t.Fatal("enabled printer must fail closed outside Windows Preview")
		}
		return
	}
	if err != nil {
		t.Fatalf("enabled printer dispatcher: %v", err)
	}
	coordinator, ok := dispatcher.(*output.Coordinator)
	if !ok || len(coordinator.Targets) != 2 {
		t.Fatalf("enabled dispatcher=%T; want coordinator with local and printer targets", dispatcher)
	}
	if coordinator.Targets[0].Name() != output.TargetLocalStorage {
		t.Fatalf("first target=%q, want independent local save", coordinator.Targets[0].Name())
	}
	durable, ok := coordinator.Targets[1].(*output.DurablePrinter)
	if !ok {
		t.Fatalf("second target=%T, want DurablePrinter", coordinator.Targets[1])
	}
	if durable.QueueName != "DS-RX1" || durable.SessionsRoot != filepath.Join(root, "sessions") {
		t.Fatalf("durable printer config=%+v", durable)
	}
	printer, ok := durable.Target.(*output.WindowsPrinter)
	if !ok {
		t.Fatalf("durable target=%T, want WindowsPrinter", durable.Target)
	}
	if printer.PrinterName != "DS-RX1" || printer.Options.MediaForm != "6x4" ||
		!printer.Options.Landscape || !printer.Options.FlipVertical {
		t.Fatalf("Windows printer options=%+v", printer)
	}
}

func TestPlayerOutputTargetViewsPreserveQueueEvidenceWithoutOverclaiming(t *testing.T) {
	views := playerOutputTargetViews([]output.TargetResult{
		{Name: output.TargetLocalStorage, Status: output.TargetSucceeded, OutputPath: "photos/photo.jpg"},
		{Name: output.TargetPrinter, Status: output.TargetQueued, JobID: "91", Duration: 1500 * 1000 * 1000},
	})
	if len(views) != 2 {
		t.Fatalf("views=%+v; want local and printer result", views)
	}
	printer := views[1]
	if printer.Name != output.TargetPrinter || printer.Status != string(output.TargetQueued) ||
		printer.JobID != "91" || printer.DurationMS != 1500 {
		t.Fatalf("printer output view=%+v", printer)
	}
}

func TestClonePlayerSessionCopiesOutputTargetHistory(t *testing.T) {
	source := PlayerSession{
		OutputTargets: []PlayerOutputTargetView{{Name: output.TargetPrinter, Status: string(output.TargetQueued), JobID: "92"}},
	}
	cloned := clonePlayerSession(source)
	cloned.OutputTargets[0].JobID = "changed"
	if source.OutputTargets[0].JobID != "92" {
		t.Fatal("mutating a UI snapshot changed the source output target history")
	}
}

func TestHandleWorkflowEventTracksEachOutputTargetOnce(t *testing.T) {
	app := NewApp()
	app.player = PlayerSession{ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", State: "output"}
	app.playerRunning = true
	app.handleWorkflowEvent(app.player.ID, workflow.Event{
		Type: "output_target", State: session.Output, Target: output.TargetPrinter,
		Status: string(output.TargetQueued), JobID: "93", Duration: 2 * time.Second,
	})
	app.handleWorkflowEvent(app.player.ID, workflow.Event{
		Type: "output_target", State: session.Output, Target: output.TargetLocalStorage,
		Status: string(output.TargetSucceeded), Path: "photos/photo.jpg",
	})
	app.handleWorkflowEvent(app.player.ID, workflow.Event{
		Type: "output_target", State: session.Output, Target: output.TargetPrinter,
		Status: string(output.TargetQueued), JobID: "94", Duration: 3 * time.Second,
	})
	if len(app.player.OutputTargets) != 2 {
		t.Fatalf("output targets=%+v, want exactly one row per target", app.player.OutputTargets)
	}
	var printer *PlayerOutputTargetView
	for index := range app.player.OutputTargets {
		if app.player.OutputTargets[index].Name == output.TargetPrinter {
			printer = &app.player.OutputTargets[index]
			break
		}
	}
	if printer == nil || printer.JobID != "94" || printer.DurationMS != 3000 {
		t.Fatalf("printer status not updated in place: %+v", app.player.OutputTargets)
	}
}

func TestPlayerCompletionMessageDistinguishesLocalSaveAndPrinterQueue(t *testing.T) {
	t.Run("local-saved-and-printer-queued", func(t *testing.T) {
		message := playerCompletionMessage(workflow.Result{
			CapturePaths: []string{"a", "b", "c"}, OutputPath: "photos/photo.jpg",
			OutputResults: []output.TargetResult{{Name: output.TargetPrinter, Status: output.TargetQueued, JobID: "99"}},
		})
		if !strings.Contains(message, "saved locally") || !strings.Contains(message, "accepted printer job 99") ||
			!strings.Contains(message, "not physical paper output") {
			t.Fatalf("message over/understates queued output: %q", message)
		}
	})
	t.Run("printer-queued-local-save-missing", func(t *testing.T) {
		message := playerCompletionMessage(workflow.Result{
			CapturePaths:  []string{"a", "b", "c"},
			OutputResults: []output.TargetResult{{Name: output.TargetPrinter, Status: output.TargetQueued, JobID: "100"}},
		})
		if !strings.Contains(message, "local save did not complete") || !strings.Contains(message, "not proof") {
			t.Fatalf("message overstates local/physical output: %q", message)
		}
	})
	t.Run("local-saved-printer-failed", func(t *testing.T) {
		message := playerCompletionMessage(workflow.Result{
			CapturePaths: []string{"a", "b", "c"}, OutputPath: "photos/photo.jpg",
			OutputResults: []output.TargetResult{{Name: output.TargetPrinter, Status: output.TargetFailed, Error: "queue offline"}},
		})
		if !strings.Contains(message, "saved locally") || !strings.Contains(message, "queue offline") {
			t.Fatalf("message lost independent local result or printer failure: %q", message)
		}
	})
}

func TestPlayerFailureMessagePreservesPrinterUncertainty(t *testing.T) {
	message := playerFailureMessage(errors.New("no output target succeeded"), false, workflow.Result{
		OutputResults: []output.TargetResult{{
			Name: output.TargetPrinter, Status: output.TargetFailed,
			Cause: &output.PrinterSubmissionError{Uncertain: true, JobID: "101", Err: errors.New("EndDoc result uncertain")},
			Error: "printer submission outcome is uncertain (job 101)",
		}},
	})
	if !strings.Contains(message, "outcome is uncertain") || !strings.Contains(message, "job 101") ||
		!strings.Contains(message, "do not resubmit automatically") {
		t.Fatalf("printer uncertainty missing from failure message: %q", message)
	}
}
