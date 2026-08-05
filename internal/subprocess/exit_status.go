package subprocess

import "os"

// ExitStatus captures the stable OS facts available after a process exits.
type ExitStatus struct {
	ExitCode int
	Signal   string
}

// ExitCodeValue returns a real numeric exit code. Unix uses -1 when a signal
// terminated the process, which is not an exit code and must not be serialized.
func (s ExitStatus) ExitCodeValue() (int, bool) {
	if s.ExitCode < 0 || s.Signal != "" {
		return 0, false
	}
	return s.ExitCode, true
}

// ExitStatusFromProcessState extracts portable process-exit diagnostics.
func ExitStatusFromProcessState(state *os.ProcessState) (ExitStatus, bool) {
	if state == nil {
		return ExitStatus{}, false
	}
	return ExitStatus{
		ExitCode: state.ExitCode(),
		Signal:   processStateSignal(state),
	}, true
}

// ExitStatus returns the captured OS exit diagnostics after process completion.
func (p *Process) ExitStatus() (ExitStatus, bool) {
	if p == nil {
		return ExitStatus{}, false
	}
	p.exitStatusMu.RLock()
	defer p.exitStatusMu.RUnlock()
	return p.exitStatus, p.hasExitStatus
}

func (p *Process) captureExitStatus(state *os.ProcessState) {
	status, ok := ExitStatusFromProcessState(state)
	if !ok {
		return
	}
	p.exitStatusMu.Lock()
	p.exitStatus = status
	p.hasExitStatus = true
	p.exitStatusMu.Unlock()
}
