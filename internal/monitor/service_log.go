package monitor

import (
	"context"
	"log/slog"
	"strings"
	"sync"
)

// Failure causes recorded on monitor.wake_failed; they never carry raw errors.
const (
	causeSessionGone        = "session_gone"
	causeRetryExhausted     = "retry_exhausted"
	causePersistUnavailable = "persist_unavailable"
)

type lifecycleEntry struct {
	level                                                    slog.Level
	event, monitorID, sessionID, kind, status, reason, cause string
}

// logQueue hands lifecycle entries to one service-owned goroutine so a slow log
// destination never stalls a monitor path that holds the service lock. Entries keep
// their order; once the dispatcher has drained and closed, callers emit directly.
type logQueue struct {
	mu     sync.Mutex
	items  []lifecycleEntry
	closed bool
	ready  chan struct{}
}

func newLogQueue() *logQueue { return &logQueue{ready: make(chan struct{}, 1)} }

// push queues the entry, or reports false when the queue is already closed.
func (q *logQueue) push(entry lifecycleEntry) bool {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return false
	}
	q.items = append(q.items, entry)
	q.mu.Unlock()
	select {
	case q.ready <- struct{}{}:
	default:
	}
	return true
}

// pop returns the next entry; with closeWhenEmpty it closes the queue atomically with the final empty check.
func (q *logQueue) pop(closeWhenEmpty bool) (lifecycleEntry, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		if closeWhenEmpty {
			q.closed = true
		}
		return lifecycleEntry{}, false
	}
	entry := q.items[0]
	q.items[0] = lifecycleEntry{}
	q.items = q.items[1:]
	return entry, true
}

func (s *Service) dispatchLogs() {
	for {
		select {
		case <-s.ctx.Done():
			for {
				entry, ok := s.logs.pop(true)
				if !ok {
					return
				}
				s.emit(entry)
			}
		case <-s.logs.ready:
		}
		for {
			entry, ok := s.logs.pop(false)
			if !ok {
				break
			}
			s.emit(entry)
		}
	}
}

func (s *Service) emit(entry lifecycleEntry) {
	attrs := []any{
		"event", entry.event, "monitor_id", entry.monitorID, "session_id", entry.sessionID,
		"kind", entry.kind, "status", entry.status, "reason", entry.reason,
	}
	if entry.cause != "" {
		attrs = append(attrs, "cause", entry.cause)
	}
	s.logger.Log(context.WithoutCancel(s.ctx), entry.level, entry.event, attrs...)
}

// logLifecycle deliberately excludes descriptions, patterns, output, and raw errors.
func (s *Service) logLifecycle(event string, mon Monitor, reason string) {
	s.logLifecycleCause(event, mon, reason, "")
}

func (s *Service) logLifecycleCause(event string, mon Monitor, reason, cause string) {
	switch reason {
	case "registered", "requested", "session_stopped", "daemon_shutdown", "admission_retry",
		ReasonExit, ReasonMatch, ReasonTimeout, ReasonStalled, ReasonTimerElapsed, ReasonTerminalFailed:
	default:
		// Owner restart-eligibility reasons come from a fixed vocabulary, never from user input.
		if !strings.HasPrefix(reason, ReasonOwnerPrefix) {
			reason = "unknown"
		}
	}
	level := slog.LevelInfo
	if event == "monitor.wake_failed" {
		level = slog.LevelWarn
	}
	entry := lifecycleEntry{
		level: level, event: event, monitorID: mon.ID, sessionID: mon.SessionID, kind: string(mon.Kind),
		status: string(mon.Status), reason: reason, cause: cause,
	}
	if !s.logs.push(entry) {
		s.emit(entry)
	}
}
