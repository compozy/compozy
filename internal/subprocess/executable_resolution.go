package subprocess

import (
	"errors"
	"os/exec"
	"path/filepath"
	"slices"
)

// ExecutableResolutionState identifies one immutable terminal executable lookup result.
type ExecutableResolutionState string

const (
	ExecutableResolutionResolved ExecutableResolutionState = "resolved"
	ExecutableResolutionNotFound ExecutableResolutionState = "not_found"
	ExecutableResolutionFailed   ExecutableResolutionState = "failed"
)

// ExecutableResolution retains one terminal executable lookup result without
// exposing its operational cause as diagnostic data.
type ExecutableResolution struct {
	state      ExecutableResolutionState
	command    string
	dir        string
	env        []string
	executable string
	cause      error
}

// NewExecutableResolution snapshots one terminal result for an exact command runtime.
func NewExecutableResolution(
	command string,
	env []string,
	dir string,
	executable string,
	cause error,
) ExecutableResolution {
	resolution := ExecutableResolution{
		command: command,
		dir:     dir,
		env:     append([]string(nil), env...),
	}
	switch {
	case cause == nil && filepath.IsAbs(executable):
		resolution.state = ExecutableResolutionResolved
		resolution.executable = filepath.Clean(executable)
	case cause == nil:
		resolution.state = ExecutableResolutionFailed
		resolution.cause = errors.New("subprocess: resolved executable must be absolute")
	case errors.Is(cause, exec.ErrNotFound):
		resolution.state = ExecutableResolutionNotFound
		resolution.cause = cause
	default:
		resolution.state = ExecutableResolutionFailed
		resolution.cause = cause
	}
	return resolution
}

// Clone returns an independent immutable snapshot.
func (r ExecutableResolution) Clone() ExecutableResolution {
	clone := r
	clone.env = append([]string(nil), r.env...)
	return clone
}

// Valid reports whether the value contains a terminal result.
func (r ExecutableResolution) Valid() bool {
	return r.state != ""
}

// State returns the safe terminal state without exposing the private cause.
func (r ExecutableResolution) State() ExecutableResolutionState {
	return r.state
}

// ResolvedExecutable returns the absolute executable only for a resolved result.
func (r ExecutableResolution) ResolvedExecutable() string {
	if r.state != ExecutableResolutionResolved {
		return ""
	}
	return r.executable
}

// Consume returns the pinned result only for its exact command runtime.
func (r ExecutableResolution) Consume(command string, env []string, dir string) (string, error) {
	if !r.Valid() {
		return "", errors.New("subprocess: executable resolution is required")
	}
	if r.command != command || r.dir != dir || !slices.Equal(r.env, env) {
		return "", errors.New("subprocess: executable resolution does not match final runtime")
	}
	if r.state == ExecutableResolutionResolved {
		return r.executable, nil
	}
	if r.cause == nil {
		return "", errors.New("subprocess: executable resolution failure cause is required")
	}
	return "", r.cause
}
