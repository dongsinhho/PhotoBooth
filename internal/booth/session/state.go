package session

type State string

const (
	Idle            State = "idle"
	Preflight       State = "preflight"
	AwaitingPayment State = "awaiting_payment"
	Capturing       State = "capturing"
	Rendering       State = "rendering"
	Output          State = "output"
	Delivery        State = "delivery"
	Completed       State = "completed"
	PartialSuccess  State = "partial_success"
	Failed          State = "failed"
	Cancelled       State = "cancelled"
	TimedOut        State = "timed_out"
)

func (s State) Terminal() bool {
	return s == Completed || s == PartialSuccess || s == Failed || s == Cancelled || s == TimedOut
}

func CanTransition(from, to State) bool {
	if from == to {
		return false
	}
	switch from {
	case Idle:
		return to == Preflight
	case Preflight:
		return to == AwaitingPayment || to == Capturing || to == Failed || to == Cancelled || to == TimedOut
	case AwaitingPayment:
		return to == Capturing || to == Failed || to == Cancelled || to == TimedOut
	case Capturing:
		return to == Rendering || to == Failed || to == Cancelled || to == TimedOut
	case Rendering:
		return to == Output || to == Failed || to == Cancelled || to == TimedOut
	case Output:
		return to == Delivery || to == Completed || to == PartialSuccess || to == Failed || to == Cancelled || to == TimedOut
	case Delivery:
		return to == Completed || to == PartialSuccess || to == Failed || to == Cancelled || to == TimedOut
	default:
		return false
	}
}
