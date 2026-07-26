package automation

import (
	"context"
	"errors"
	"fmt"
	"regexp"
)

// ErrDaemonLifecycleCommandBlocked reports a rejected daemon lifecycle command.
var ErrDaemonLifecycleCommandBlocked = errors.New("automation: daemon lifecycle command blocked")

// DaemonLifecycleCommandClass identifies the blocked command family.
type DaemonLifecycleCommandClass string

const (
	// DaemonLifecycleCommandClassAGHDaemon identifies the native AGH daemon CLI.
	DaemonLifecycleCommandClassAGHDaemon DaemonLifecycleCommandClass = "agh_daemon"
	// DaemonLifecycleCommandClassProcessSignal identifies process-targeting kill commands.
	DaemonLifecycleCommandClassProcessSignal DaemonLifecycleCommandClass = "process_signal"
	// DaemonLifecycleCommandClassServiceManager identifies service-manager lifecycle commands.
	DaemonLifecycleCommandClassServiceManager DaemonLifecycleCommandClass = "service_manager"
)

// DaemonLifecycleCommandError carries the deterministic blocked command class.
type DaemonLifecycleCommandError struct {
	Class DaemonLifecycleCommandClass
}

// Error returns the stable rejection message.
func (e *DaemonLifecycleCommandError) Error() string {
	if e == nil {
		return ErrDaemonLifecycleCommandBlocked.Error()
	}
	return fmt.Sprintf("%s (class=%s)", ErrDaemonLifecycleCommandBlocked, e.Class)
}

// Unwrap exposes the shared lifecycle-command sentinel.
func (e *DaemonLifecycleCommandError) Unwrap() error {
	return ErrDaemonLifecycleCommandBlocked
}

type daemonLifecycleCommandPattern struct {
	class   DaemonLifecycleCommandClass
	pattern *regexp.Regexp
}

var daemonLifecycleCommandPatterns = []daemonLifecycleCommandPattern{
	{
		class: DaemonLifecycleCommandClassAGHDaemon,
		pattern: regexp.MustCompile(
			`(?i)\bagh(?:[[:space:]]+(?:--json|--output(?:=[^[:space:]\n]+|[[:space:]]+[^[:space:]\n]+)|-o(?:=[^[:space:]\n]+|[[:space:]]+[^[:space:]\n]+)))*[[:space:]]+daemon[[:space:]]+(restart|stop)\b`,
		),
	},
	{
		class:   DaemonLifecycleCommandClassProcessSignal,
		pattern: regexp.MustCompile(`(?i)\b(pkill|killall)\b[^\n]*\bagh\b`),
	},
	{
		class:   DaemonLifecycleCommandClassProcessSignal,
		pattern: regexp.MustCompile(`(?i)\bkill\b[^\n]*\bpgrep\b[^\n]*\bagh\b`),
	},
	{
		class: DaemonLifecycleCommandClassServiceManager,
		pattern: regexp.MustCompile(
			`(?i)\bsystemctl\b[^\n]*\b(restart|stop|start)\b[^\n]*\bagh\b`,
		),
	},
	{
		class: DaemonLifecycleCommandClassServiceManager,
		pattern: regexp.MustCompile(
			`(?i)\blaunchctl[[:space:]]+(kickstart|unload|load|stop|restart)\b[^\n]*\bagh\b`,
		),
	},
	{
		class: DaemonLifecycleCommandClassServiceManager,
		pattern: regexp.MustCompile(
			`(?i)\bservice[[:space:]]+[^[:space:]\n]*agh[^[:space:]\n]*[[:space:]]+(restart|stop|start)\b`,
		),
	},
}

func validateJobDaemonLifecycleCommand(job Job) error {
	texts := []string{job.Prompt}
	if job.Task != nil {
		texts = append(texts, job.Task.Description)
	}

	for _, text := range texts {
		for _, candidate := range daemonLifecycleCommandPatterns {
			if candidate.pattern.MatchString(text) {
				return &DaemonLifecycleCommandError{Class: candidate.class}
			}
		}
	}
	return nil
}

func (m *Manager) validateJobDefinition(ctx context.Context, job Job) error {
	if err := validateJobDaemonLifecycleCommand(job); err != nil {
		return err
	}
	if err := job.Validate("job"); err != nil {
		return err
	}
	return m.validateJobLoopTarget(ctx, job)
}
