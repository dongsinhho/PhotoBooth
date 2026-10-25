package output

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

const testSessionID = "0123456789abcdef0123456789abcdef"

func writeOutputSource(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rendered.jpg")
	if err := os.WriteFile(path, []byte("test-rendered-image"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

type targetFunc struct {
	name string
	run  func(context.Context, Job) (TargetOutput, error)
}

func (t targetFunc) Name() string { return t.name }
func (t targetFunc) Execute(ctx context.Context, job Job) (TargetOutput, error) {
	return t.run(ctx, job)
}

func TestCoordinatorKeepsLocalSaveWhenPrinterFails(t *testing.T) {
	source := writeOutputSource(t)
	localRoot := filepath.Join(t.TempDir(), "photos")
	printer := targetFunc{name: TargetPrinter, run: func(context.Context, Job) (TargetOutput, error) {
		return TargetOutput{}, errors.New("paper out")
	}}

	result, err := NewCoordinator(NewLocal(localRoot), printer).Dispatch(context.Background(), Job{
		SessionID: testSessionID, SourcePath: source,
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if result.Succeeded != 1 || result.Failed != 1 {
		t.Fatalf("counts = succeeded:%d failed:%d; want 1 success and 1 failure", result.Succeeded, result.Failed)
	}
	if len(result.Results) != 2 || result.Results[0].Name != TargetLocalStorage || result.Results[0].Status != TargetSucceeded {
		t.Fatalf("local target result = %+v", result.Results)
	}
	if result.Results[1].Name != TargetPrinter || result.Results[1].Status != TargetFailed || result.Results[1].Error != "paper out" {
		t.Fatalf("printer target result = %+v", result.Results[1])
	}
	if _, err := os.Stat(result.Results[0].OutputPath); err != nil {
		t.Fatalf("local copy was not saved: %v", err)
	}
}

func TestCoordinatorRunsTargetsConcurrentlyAndReturnsConfiguredOrder(t *testing.T) {
	source := writeOutputSource(t)
	started := make(chan string, 2)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseAll := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseAll()

	makeBlockingTarget := func(name string) Target {
		return targetFunc{name: name, run: func(ctx context.Context, _ Job) (TargetOutput, error) {
			started <- name
			select {
			case <-release:
				return TargetOutput{JobID: "job-" + name}, nil
			case <-ctx.Done():
				return TargetOutput{}, ctx.Err()
			}
		}}
	}

	type dispatchResponse struct {
		result DispatchResult
		err    error
	}
	done := make(chan dispatchResponse, 1)
	coordinator := NewCoordinator(makeBlockingTarget("first"), makeBlockingTarget("second"))
	go func() {
		result, err := coordinator.Dispatch(context.Background(), Job{SessionID: testSessionID, SourcePath: source})
		done <- dispatchResponse{result: result, err: err}
	}()

	seen := map[string]bool{}
	timeout := time.After(2 * time.Second)
	for len(seen) < 2 {
		select {
		case name := <-started:
			seen[name] = true
		case <-timeout:
			t.Fatalf("both targets were not started concurrently; observed %v", seen)
		}
	}
	releaseAll()

	select {
	case response := <-done:
		if response.err != nil {
			t.Fatalf("Dispatch: %v", response.err)
		}
		if response.result.Succeeded != 2 || len(response.result.Results) != 2 {
			t.Fatalf("unexpected dispatch summary: %+v", response.result)
		}
		if response.result.Results[0].Name != "first" || response.result.Results[1].Name != "second" {
			t.Fatalf("results should retain configured order: %+v", response.result.Results)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Dispatch did not finish after releasing both targets")
	}
}

func TestCoordinatorRejectsDuplicateTargetNames(t *testing.T) {
	source := writeOutputSource(t)
	target := targetFunc{name: TargetPrinter, run: func(context.Context, Job) (TargetOutput, error) {
		return TargetOutput{}, nil
	}}
	_, err := NewCoordinator(target, target).Dispatch(context.Background(), Job{SessionID: testSessionID, SourcePath: source})
	if err == nil {
		t.Fatal("expected duplicate target names to be rejected")
	}
}

func TestCoordinatorRejectsEmptyOrInvalidJobs(t *testing.T) {
	target := targetFunc{name: "dummy", run: func(context.Context, Job) (TargetOutput, error) {
		return TargetOutput{}, nil
	}}
	if _, err := NewCoordinator().Dispatch(context.Background(), Job{}); err == nil {
		t.Fatal("expected empty target configuration to fail")
	}
	if _, err := NewCoordinator(target).Dispatch(context.Background(), Job{SessionID: "bad", SourcePath: "missing"}); err == nil {
		t.Fatal("expected invalid session ID to fail before dispatch")
	}
}
