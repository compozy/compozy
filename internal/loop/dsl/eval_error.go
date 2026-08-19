package dsl

// EvalErrorPolicy selects the authored outcome for a predicate evaluation error.
type EvalErrorPolicy string

const (
	// EvalErrorFail turns the predicate error into a routable authoring failure.
	EvalErrorFail EvalErrorPolicy = "fail"
	// EvalErrorExit exits the current loop through its normal completion path.
	EvalErrorExit EvalErrorPolicy = "exit"
)

// Valid reports whether the policy belongs to the closed authoring vocabulary.
func (p EvalErrorPolicy) Valid() bool {
	return p == "" || p == EvalErrorFail || p == EvalErrorExit
}
