package loop

// RunCancelKind records the operator's run-level cancellation intent.
type RunCancelKind uint8

const (
	RunCancelNone RunCancelKind = iota
	RunCancelCancel
	RunCancelKill
	runCancelInvalid
)

// ParseRunCancelKind converts the durable string representation into the compact runtime enum.
func ParseRunCancelKind(value string) RunCancelKind {
	switch value {
	case "":
		return RunCancelNone
	case "cancel":
		return RunCancelCancel
	case "kill":
		return RunCancelKill
	default:
		return runCancelInvalid
	}
}

// String returns the durable run-cancel representation.
func (k RunCancelKind) String() string {
	switch k {
	case RunCancelNone:
		return ""
	case RunCancelCancel:
		return "cancel"
	case RunCancelKill:
		return "kill"
	default:
		return "invalid"
	}
}

// Valid reports whether value belongs to the closed run-cancel vocabulary.
func (k RunCancelKind) Valid() bool {
	return k < runCancelInvalid
}
