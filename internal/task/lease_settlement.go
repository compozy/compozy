package task

// ClaimedRunSettlement is the durable claim projection without the raw bearer token.
type ClaimedRunSettlement struct {
	Run               Run
	Task              Task
	StatusTransitions []StatusTransition
}

// HeartbeatRunLeaseSettlement is the durable task projection produced with a heartbeat.
type HeartbeatRunLeaseSettlement struct {
	Run  Run
	Task Task
}

// ReleasedRunLeaseSettlement is the durable task projection produced with a release.
type ReleasedRunLeaseSettlement struct {
	Run               Run
	PreviousRun       Run
	Task              Task
	StatusTransitions []StatusTransition
}

// FailedRunLeaseMutation is the fenced run mutation with its canonical failure.
type FailedRunLeaseMutation struct {
	Run     Run
	Failure RunFailure
}

// FailedRunLeaseSettlement is the durable task projection produced with a lease failure.
type FailedRunLeaseSettlement struct {
	Run               Run
	Task              Task
	StatusTransitions []StatusTransition
}
