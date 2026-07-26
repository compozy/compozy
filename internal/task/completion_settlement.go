package task

// StatusTransition records one durable task status change committed as
// part of successful run settlement.
type StatusTransition struct {
	Task           Task
	PreviousStatus Status
}

// CompletedRunSettlement is the durable result of completing one run and
// reconciling successful parent-task aggregation in the same transaction.
type CompletedRunSettlement struct {
	Run               Run
	Task              Task
	StatusTransitions []StatusTransition
	RolledUpRuns      []Run
}
