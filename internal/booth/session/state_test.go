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

func TestPartialOutputIsTerminal(t *testing.T) {
	if !CanTransition(Output, PartialSuccess) {
		t.Fatal("expected Output -> PartialSuccess transition")
	}
	if !PartialSuccess.Terminal() {
		t.Fatal("partial success should be terminal")
	}
	if CanTransition(PartialSuccess, Idle) {
		t.Fatal("partial success must not continue to another session state")
	}
}

func TestTerminalStatesCannotContinue(t *testing.T) {
	for _, state := range []State{Completed, PartialSuccess, Failed, Cancelled, TimedOut} {
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
