package run

import "time"

const (
	processTerminationGracePeriod = 3 * time.Second
	gracefulShutdownTimeout       = 3 * time.Second
)

type shutdownPhase string

const (
	shutdownPhaseIdle     shutdownPhase = ""
	shutdownPhaseDraining shutdownPhase = "draining"
	shutdownPhaseForcing  shutdownPhase = "forcing"
)

type shutdownSource string

const (
	shutdownSourceUI     shutdownSource = "ui"
	shutdownSourceSignal shutdownSource = "signal"
	shutdownSourceTimer  shutdownSource = "timer"
)

type shutdownState struct {
	Phase       shutdownPhase
	Source      shutdownSource
	RequestedAt time.Time
	DeadlineAt  time.Time
}

func (s shutdownState) active() bool {
	return s.Phase != shutdownPhaseIdle
}

type uiQuitRequest int

const (
	uiQuitRequestDrain uiQuitRequest = iota
	uiQuitRequestForce
)

type shutdownStatusMsg struct {
	State shutdownState
}
