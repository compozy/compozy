package sessioncleanup

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// RunState is where one cleanup run is.
type RunState string

const (
	// RunRunning deletes in the background.
	RunRunning RunState = "running"
	// RunCompleted deleted every planned session.
	RunCompleted RunState = "completed"
	// RunPartial deleted some sessions; Failures name the threads that kept the rest.
	RunPartial RunState = "partial"
	// RunFailed deleted nothing; Failures say why.
	RunFailed RunState = "failed"
	// RunStopped ended on request or at daemon shutdown before finishing.
	RunStopped RunState = "stopped"
)

// FailureCode classifies why a thread kept some of its sessions.
type FailureCode string

const (
	FailureInstanceMain FailureCode = "instance_main"
	FailureDeleteFailed FailureCode = "delete_failed"
)

// maxReportedFailures bounds the failures one run reports; FailedThreads keeps the count.
const maxReportedFailures = 100

// Failure is one thread that kept sessions the run meant to delete. Retrying
// it means a new run limited to RootID.
type Failure struct {
	RootID      string
	WorkspaceID string
	Title       string
	Code        FailureCode
	Message     string
}

// Run is a snapshot of one cleanup run.
type Run struct {
	ID             string
	ProfileID      string
	Scope          Scope
	Selection      Selection
	ArchivedBefore *time.Time
	State          RunState
	// Total is every session the run planned to delete.
	Total int
	// Deleted counts sessions removed (or already gone).
	Deleted int
	// Skipped counts sessions restored or moved after the plan, so kept.
	Skipped       int
	FailedThreads int
	Failures      []Failure
	StartedAt     time.Time
	FinishedAt    *time.Time
}

// Done counts the planned sessions the run has settled.
func (r Run) Done() int {
	return r.Deleted + r.Skipped
}

// cleanupRun is the live state of one run; mu guards everything below it.
type cleanupRun struct {
	cancel func()
	done   chan struct{}

	mu          sync.Mutex
	run         Run
	lastPublish time.Time
}

func (r *cleanupRun) snapshot() Run {
	r.mu.Lock()
	defer r.mu.Unlock()
	run := r.run
	run.Failures = slices.Clone(r.run.Failures)
	return run
}

func (r *cleanupRun) running() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.run.State == RunRunning
}

// settle records one delete; it reports whether progress is due to publish.
func (r *cleanupRun) settle(err error, now time.Time, interval time.Duration) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case err == nil || errors.Is(err, store.ErrSessionNotFound):
		r.run.Deleted++
	case errors.Is(err, store.ErrSessionNotArchived):
		r.run.Skipped++
	}
	if now.Sub(r.lastPublish) < interval {
		return false
	}
	r.lastPublish = now
	return true
}

func (r *cleanupRun) fail(thread Thread, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.run.FailedThreads++
	if len(r.run.Failures) >= maxReportedFailures {
		return
	}
	r.run.Failures = append(r.run.Failures, Failure{
		RootID:      thread.RootID,
		WorkspaceID: thread.WorkspaceID,
		Title:       thread.Title,
		Code:        failureCode(err),
		Message:     failureMessage(err),
	})
}

// finish settles the run; interrupted means a stop or shutdown left planned
// threads unprocessed.
func (r *cleanupRun) finish(now time.Time, interrupted bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case interrupted:
		r.run.State = RunStopped
	case r.run.FailedThreads > 0 && r.run.Deleted == 0:
		r.run.State = RunFailed
	case r.run.FailedThreads > 0:
		r.run.State = RunPartial
	default:
		r.run.State = RunCompleted
	}
	r.run.FinishedAt = &now
}

func failureCode(err error) FailureCode {
	if errors.Is(err, store.ErrSessionIsInstanceMain) {
		return FailureInstanceMain
	}
	return FailureDeleteFailed
}

func failureMessage(err error) string {
	if errors.Is(err, store.ErrSessionIsInstanceMain) {
		return "It is an always-on agent's main session."
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return "It couldn't be deleted."
	}
	return message
}
