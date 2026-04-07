package runshared

import "time"

const (
	ProcessTerminationGracePeriod = 3 * time.Second
	GracefulShutdownTimeout       = 3 * time.Second
)

type ShutdownPhase string

const (
	ShutdownPhaseIdle     ShutdownPhase = ""
	ShutdownPhaseDraining ShutdownPhase = "draining"
	ShutdownPhaseForcing  ShutdownPhase = "forcing"
)

type ShutdownSource string

const (
	ShutdownSourceUI     ShutdownSource = "ui"
	ShutdownSourceSignal ShutdownSource = "signal"
	ShutdownSourceTimer  ShutdownSource = "timer"
)

type ShutdownState struct {
	Phase       ShutdownPhase
	Source      ShutdownSource
	RequestedAt time.Time
	DeadlineAt  time.Time
}

func (s ShutdownState) Active() bool {
	return s.Phase != ShutdownPhaseIdle
}

type UIQuitRequest int

const (
	UIQuitRequestDrain UIQuitRequest = iota
	UIQuitRequestForce
)

type UISession interface {
	Enqueue(any)
	SetQuitHandler(func(UIQuitRequest))
	CloseEvents()
	Shutdown()
	Wait() error
}
