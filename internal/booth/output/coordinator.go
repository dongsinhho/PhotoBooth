package output

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	TargetLocalStorage = "local-storage"
	TargetPrinter      = "printer"
	maxTargets         = 4
)

type TargetStatus string

const (
	TargetSucceeded TargetStatus = "succeeded"
	TargetQueued    TargetStatus = "queued"
	TargetFailed    TargetStatus = "failed"
	TargetCancelled TargetStatus = "cancelled"
	TargetTimedOut  TargetStatus = "timed_out"
)

// Job describes the already-rendered image that all output targets receive.
// Targets must treat the source as read-only.
type Job struct {
	SessionID  string
	SourcePath string
}

// TargetOutput contains target-specific details. OutputPath is typically used
// by local storage; JobID can identify a printer spool/queue job.
type TargetOutput struct {
	// Status can be TargetQueued when a target accepted an asynchronous job
	// (for example, a printer spool request). Empty means TargetSucceeded.
	Status     TargetStatus
	OutputPath string
	JobID      string
}

// Target runs one independent output operation. Slow hardware targets such as
// printers should enqueue/spool the job and return a JobID instead of blocking
// the session until the physical device finishes printing.
type Target interface {
	Name() string
	Execute(ctx context.Context, job Job) (TargetOutput, error)
}

type TargetResult struct {
	Name       string
	Status     TargetStatus
	OutputPath string
	JobID      string
	Duration   time.Duration
	Error      string
}

type DispatchResult struct {
	Results   []TargetResult
	Succeeded int
	Queued    int
	Failed    int
	Cancelled int
	TimedOut  int
}

func (r DispatchResult) HasSuccess() bool { return r.Succeeded+r.Queued > 0 }

// Dispatcher attempts all configured output targets independently. A target
// failure does not cancel another target. The coordinator runs a bounded number
// of targets concurrently and returns results in configuration order.
type Dispatcher interface {
	Dispatch(ctx context.Context, job Job) (DispatchResult, error)
}

type Coordinator struct {
	Targets []Target
}

func NewCoordinator(targets ...Target) *Coordinator {
	return &Coordinator{Targets: append([]Target(nil), targets...)}
}

type indexedResult struct {
	index  int
	result TargetResult
}

func (c *Coordinator) Dispatch(ctx context.Context, job Job) (DispatchResult, error) {
	var dispatch DispatchResult
	if err := ctx.Err(); err != nil {
		return dispatch, err
	}
	if c == nil {
		return dispatch, errors.New("output coordinator is nil")
	}
	if len(c.Targets) == 0 {
		return dispatch, errors.New("at least one output target is required")
	}
	if len(c.Targets) > maxTargets {
		return dispatch, fmt.Errorf("at most %d output targets may be configured", maxTargets)
	}
	if !validSessionID(job.SessionID) {
		return dispatch, errors.New("session ID must be a 32-character lowercase hexadecimal identifier")
	}
	if strings.TrimSpace(job.SourcePath) == "" {
		return dispatch, errors.New("output source path is required")
	}
	info, err := os.Stat(job.SourcePath)
	if err != nil {
		return dispatch, fmt.Errorf("stat output source: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return dispatch, errors.New("output source must be a non-empty regular file")
	}

	names := make(map[string]struct{}, len(c.Targets))
	for _, target := range c.Targets {
		if target == nil {
			return dispatch, errors.New("output target cannot be nil")
		}
		name := strings.TrimSpace(target.Name())
		if name == "" {
			return dispatch, errors.New("output target name cannot be empty")
		}
		if _, exists := names[name]; exists {
			return dispatch, fmt.Errorf("duplicate output target name %q", name)
		}
		names[name] = struct{}{}
	}

	dispatch.Results = make([]TargetResult, len(c.Targets))
	completed := make(chan indexedResult, len(c.Targets))
	for index, target := range c.Targets {
		go func(index int, target Target) {
			started := time.Now()
			outputData, runErr := target.Execute(ctx, job)
			result := TargetResult{
				Name:       target.Name(),
				OutputPath: outputData.OutputPath,
				JobID:      outputData.JobID,
				Duration:   time.Since(started),
			}
			switch {
			case runErr == nil && (outputData.Status == "" || outputData.Status == TargetSucceeded):
				result.Status = TargetSucceeded
			case runErr == nil && outputData.Status == TargetQueued:
				result.Status = TargetQueued
			case runErr == nil:
				result.Status = TargetFailed
				result.Error = fmt.Sprintf("target returned unsupported success status %q", outputData.Status)
			case errors.Is(runErr, context.DeadlineExceeded):
				result.Status = TargetTimedOut
				result.Error = runErr.Error()
			case errors.Is(runErr, context.Canceled):
				result.Status = TargetCancelled
				result.Error = runErr.Error()
			default:
				result.Status = TargetFailed
				result.Error = runErr.Error()
			}
			completed <- indexedResult{index: index, result: result}
		}(index, target)
	}

	for range c.Targets {
		item := <-completed
		dispatch.Results[item.index] = item.result
		switch item.result.Status {
		case TargetSucceeded:
			dispatch.Succeeded++
		case TargetQueued:
			dispatch.Queued++
		case TargetFailed:
			dispatch.Failed++
		case TargetCancelled:
			dispatch.Cancelled++
		case TargetTimedOut:
			dispatch.TimedOut++
		}
	}
	return dispatch, nil
}
