package agent

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/core/model"
)

// Session represents an active ACP session streaming updates.
type Session interface {
	// ID returns the ACP session identifier.
	ID() string
	// Identity returns the canonical session identifiers known for this session.
	Identity() SessionIdentity
	// Updates returns the streamed session updates in arrival order.
	Updates() <-chan model.SessionUpdate
	// Done closes when the session has fully completed.
	Done() <-chan struct{}
	// Err returns the terminal session error, if any.
	Err() error
	// SlowPublishes returns the number of publishes that waited for backpressure to clear.
	SlowPublishes() uint64
	// DroppedUpdates returns the number of updates dropped after backpressure timed out.
	DroppedUpdates() uint64
}

// SessionIdentity captures the stable ACP and provider-specific ids for a session.
type SessionIdentity struct {
	ACPSessionID   string `json:"acp_session_id"`
	AgentSessionID string `json:"agent_session_id,omitempty"`
	Resumed        bool   `json:"resumed,omitempty"`
}

const (
	sessionUpdatesBufferCap = 1024
	sessionDropLogInterval  = time.Second
)

var sessionPublishBackpressureTimeout = 5 * time.Second

type sessionImpl struct {
	id           string
	identity     SessionIdentity
	workingDir   string
	allowedRoots []string
	updates      chan model.SessionUpdate
	done         chan struct{}

	slowPublishes  atomic.Uint64
	droppedUpdates atomic.Uint64

	mu                          sync.RWMutex
	publishCond                 *sync.Cond
	err                         error
	finished                    bool
	activePublishes             int
	suppressUpdates             bool
	updatesSeen                 int
	lastUpdateWasFailedToolCall bool
	lastDropLogAt               time.Time
}

var _ Session = (*sessionImpl)(nil)

func newSession(id string) *sessionImpl {
	session := &sessionImpl{
		id: id,
		identity: SessionIdentity{
			ACPSessionID: id,
		},
		updates: make(chan model.SessionUpdate, sessionUpdatesBufferCap),
		done:    make(chan struct{}),
	}
	session.publishCond = sync.NewCond(&session.mu)
	return session
}

func newSessionWithAccess(id string, workingDir string, allowedRoots []string) *sessionImpl {
	session := newSession(id)
	session.workingDir = workingDir
	session.allowedRoots = append([]string(nil), allowedRoots...)
	return session
}

func newLoadedSession(id string, workingDir string, allowedRoots []string) *sessionImpl {
	session := newSessionWithAccess(id, workingDir, allowedRoots)
	session.identity.Resumed = true
	session.suppressUpdates = true
	return session
}

func (s *sessionImpl) Updates() <-chan model.SessionUpdate {
	return s.updates
}

func (s *sessionImpl) ID() string {
	return s.id
}

func (s *sessionImpl) Identity() SessionIdentity {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.identity
}

func (s *sessionImpl) Done() <-chan struct{} {
	return s.done
}

func (s *sessionImpl) Err() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.err
}

func (s *sessionImpl) SlowPublishes() uint64 {
	return s.slowPublishes.Load()
}

func (s *sessionImpl) DroppedUpdates() uint64 {
	return s.droppedUpdates.Load()
}

func (s *sessionImpl) publish(ctx context.Context, update model.SessionUpdate) {
	s.mu.Lock()
	if s.finished {
		s.mu.Unlock()
		return
	}
	if update.Status == "" {
		update.Status = model.StatusRunning
	}
	s.updatesSeen++
	s.lastUpdateWasFailedToolCall = update.Kind == model.UpdateKindToolCallUpdated &&
		update.ToolCallState == model.ToolCallStateFailed
	if s.suppressUpdates {
		s.mu.Unlock()
		return
	}
	s.activePublishes++
	s.mu.Unlock()
	defer s.endPublish()

	select {
	case s.updates <- update:
		return
	default:
	}

	timer := time.NewTimer(sessionPublishBackpressureTimeout)
	defer timer.Stop()

	select {
	case s.updates <- update:
		s.slowPublishes.Add(1)
	case <-timer.C:
		droppedTotal := s.droppedUpdates.Add(1)
		s.warnDroppedUpdate(update.Kind, droppedTotal)
	case <-ctx.Done():
		return
	}
}

func (s *sessionImpl) warnDroppedUpdate(kind model.SessionUpdateKind, droppedTotal uint64) {
	s.mu.Lock()
	now := time.Now()
	if !s.lastDropLogAt.IsZero() && now.Sub(s.lastDropLogAt) < sessionDropLogInterval {
		s.mu.Unlock()
		return
	}
	s.lastDropLogAt = now
	sessionID := s.id
	bufferCap := cap(s.updates)
	s.mu.Unlock()

	slog.Warn(
		"acp session update dropped after backpressure timeout",
		"session_id", sessionID,
		"buffer_cap", bufferCap,
		"dropped_total", droppedTotal,
		"kind", kind,
	)
}

func (s *sessionImpl) endPublish() {
	s.mu.Lock()
	s.activePublishes--
	if s.activePublishes == 0 {
		s.publishCond.Broadcast()
	}
	s.mu.Unlock()
}

func (s *sessionImpl) finish(status model.SessionStatus, err error) {
	s.mu.Lock()
	if s.finished {
		s.mu.Unlock()
		return
	}
	s.finished = true
	s.err = err
	for s.activePublishes > 0 {
		s.publishCond.Wait()
	}
	select {
	case s.updates <- model.SessionUpdate{Status: status}:
	default:
	}
	close(s.updates)
	close(s.done)
	s.mu.Unlock()
}

func (s *sessionImpl) setAgentSessionID(agentSessionID string) {
	trimmed := strings.TrimSpace(agentSessionID)
	if trimmed == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.identity.AgentSessionID = trimmed
}

func (s *sessionImpl) resumeUpdates() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.suppressUpdates = false
}

func (s *sessionImpl) waitForIdle(ctx context.Context, idleWindow time.Duration) {
	s.mu.RLock()
	lastSeen := s.updatesSeen
	s.mu.RUnlock()

	timer := time.NewTimer(idleWindow)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			s.mu.RLock()
			currentSeen := s.updatesSeen
			s.mu.RUnlock()
			if currentSeen == lastSeen {
				return
			}
			lastSeen = currentSeen
			timer.Reset(idleWindow)
		}
	}
}

func (s *sessionImpl) lastUpdateFailedToolCall() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastUpdateWasFailedToolCall
}
