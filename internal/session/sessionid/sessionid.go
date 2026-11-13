// Package sessionid validates the canonical shapes of session, turn, and run identities for callers
// that must not depend on the session runtime.
package sessionid

import "regexp"

var (
	// sessionPattern matches generated and subagent-derived ids (`sess-` and 16 hex digits) and
	// deterministic Loop binding ids (`sess_` and 32 hex digits).
	sessionPattern = regexp.MustCompile(`^sess(?:-[0-9a-f]{16}|_[0-9a-f]{32})$`)
	// turnPattern matches generated ids (`turn-` and 16 hex digits), restart-continuation ids
	// (`turn-ev_restart_` and 32 hex digits), and the daemon's task claim-handoff and role-activation
	// turn ids (32 hex digits), which the session runtime uses verbatim.
	turnPattern = regexp.MustCompile(
		`^(?:turn-(?:[0-9a-f]{16}|ev_restart_[0-9a-f]{32})|task-(?:claim-handoff|role-activation)-[0-9a-f]{32})$`,
	)
	// runPattern matches generated prompt run ids.
	runPattern = regexp.MustCompile(`^run-[0-9a-f]{16}$`)
)

// IsSessionID reports whether value has a session identity shape.
func IsSessionID(value string) bool {
	return sessionPattern.MatchString(value)
}

// IsTurnID reports whether value has a turn identity shape.
func IsTurnID(value string) bool {
	return turnPattern.MatchString(value)
}

// IsRunID reports whether value has a prompt run identity shape.
func IsRunID(value string) bool {
	return runPattern.MatchString(value)
}
