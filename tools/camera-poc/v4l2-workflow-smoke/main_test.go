package main

import (
	"testing"
	"time"
)

func TestRecoveryRetryBudget(t *testing.T) {
	tests := []struct {
		name       string
		attempts   int
		retryDelay time.Duration
		maxDelay   time.Duration
		want       time.Duration
	}{
		{name: "sixty attempts with capped linear backoff", attempts: 60, retryDelay: 500 * time.Millisecond, maxDelay: 2 * time.Second, want: 115 * time.Second},
		{name: "one attempt has no retry delay", attempts: 1, retryDelay: 500 * time.Millisecond, maxDelay: 2 * time.Second, want: 0},
		{name: "non-positive attempts", attempts: 0, retryDelay: 500 * time.Millisecond, maxDelay: 2 * time.Second, want: 0},
		{name: "non-positive retry delay", attempts: 60, retryDelay: 0, maxDelay: 2 * time.Second, want: 0},
		{name: "non-positive maximum delay", attempts: 60, retryDelay: 500 * time.Millisecond, maxDelay: 0, want: 0},
		{name: "maximum delay is applied", attempts: 5, retryDelay: time.Second, maxDelay: 2 * time.Second, want: 7 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := recoveryRetryBudget(tt.attempts, tt.retryDelay, tt.maxDelay); got != tt.want {
				t.Fatalf("recoveryRetryBudget() = %s, want %s", got, tt.want)
			}
		})
	}
}
