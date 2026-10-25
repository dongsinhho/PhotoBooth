package session

import "testing"

func TestValidSessionFlowWithoutPayment(t *testing.T) {
	flow := [7]State{Idle, Preflight, Capturing, Rendering, Output, Delivery, Completed}
	for i := 0; i < len(flow)-1; i++ {
		if !CanTransition(flow[i], flow[i+1]) {
			t.Fatalf("expected transition %s -> %s", flow[i], flow[i+1])
		}
	}
}

func TestValidSessionFlowWithPayment(t *testing.T) {
	flow := [8]State{Idle, Preflight, AwaitingPayment, Capturing, Rendering, Output, Delivery, Completed}
	for i := 0; i < len(flow)-1; i++ {
		if !CanTransition(flow[i], flow[i+1]) {
			t.Fatalf("expected transition %s -> %s", flow[i], flow[i+1])
		}
	}
}

func TestTerminalStatesCannotContinue(t *testing.T) {
	for _, state := range []State{Completed, Failed, Cancelled, TimedOut} {
		if !state.Terminal() {
			t.Fatalf("expected %s to be terminal", state)
		}
		if CanTransition(state, Idle) {
			t.Fatalf("terminal state %s must not transition", state)
		}
	}
}

func TestInvalidTransitionsAreRejected(t *testing.T) {
	invalid := [][2]State{
		{Idle, Capturing},
		{Preflight, Rendering},
		{Capturing, Output},
		{Rendering, Completed},
		{Output, Capturing},
	}
	for _, pair := range invalid {
		if CanTransition(pair[0], pair[1]) {
			t.Fatalf("unexpected transition %s -> %s", pair[0], pair[1])
		}
	}
}
