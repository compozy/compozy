package session

import (
	"context"

	"strings"
	"sync"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/store"

	"github.com/compozy/agh/internal/transcript"
)

const (
	runtimeActivityKindPromptStarted   = "prompt_started"
	runtimeActivityKindAgentWaiting    = "agent_waiting"
	runtimeActivityKindWarning         = "warning"
	runtimeActivityKindTimeout         = "timeout"
	runtimeActivityEvidenceStallReason = "stall_reason"
)

type promptActivitySupervisor struct {
	manager *Manager
	session *Session

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	turnID     string
	turnSource TurnSource
	startedAt  time.Time
	deadlineAt *time.Time
	config     aghconfig.SessionSupervisionConfig
	events     chan acp.AgentEvent

	mu                    sync.Mutex
	activity              store.SessionActivityMeta
	warned                bool
	timedOut              bool
	unhealthy             bool
	unhealthyWarned       bool
	deadlineWarnAck       chan struct{}
	deadlineWarnEvent     acp.AgentEvent
	deadlineWarnPending   bool
	deadlineWarnDelivered bool
	closeOnce             sync.Once
}

func newPromptActivitySupervisor(
	ctx context.Context,
	manager *Manager,
	session *Session,
	turnState *promptTurnDispatchState,
	config aghconfig.SessionSupervisionConfig,
) *promptActivitySupervisor {
	supervisorBase := context.Background()
	if ctx != nil {
		supervisorBase = context.WithoutCancel(ctx)
	}
	supervisorCtx, cancel := context.WithCancel(supervisorBase)
	startedAt := time.Now().UTC()
	if manager != nil && manager.now != nil {
		startedAt = manager.now().UTC()
	}
	deadlineAt, hasDeadline := deadlineFromContext(ctx)
	if config.PromptDeadline > 0 {
		deadlineAt = startedAt.Add(config.PromptDeadline)
		hasDeadline = true
	}
	turnID := ""
	turnSource := TurnSourceUser
	if turnState != nil {
		turnID = strings.TrimSpace(turnState.turnID)
		turnSource = normalizeTurnSource(turnState.turnSource)
	}
	if turnSource == "" {
		turnSource = TurnSourceUser
	}

	return &promptActivitySupervisor{
		manager:    manager,
		session:    session,
		ctx:        supervisorCtx,
		cancel:     cancel,
		done:       make(chan struct{}),
		turnID:     turnID,
		turnSource: turnSource,
		startedAt:  startedAt,
		deadlineAt: deadlinePointer(deadlineAt, hasDeadline),
		config:     config,
		events:     make(chan acp.AgentEvent, 8),
		activity: store.SessionActivityMeta{
			TurnID:        turnID,
			TurnSource:    string(turnSource),
			TurnStartedAt: timePtr(startedAt),
		},
	}
}

func (s *promptActivitySupervisor) start() {
	if s == nil {
		return
	}
	s.touch(s.startedAt, runtimeActivityKindPromptStarted, "prompt started")
	go s.run()
}

func (s *promptActivitySupervisor) stop() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		s.cancel()
		<-s.done
	})
}

func (s *promptActivitySupervisor) eventsChannel() <-chan acp.AgentEvent {
	if s == nil {
		return nil
	}
	return s.events
}

func (s *promptActivitySupervisor) report(report acp.PromptActivityReport) {
	if s == nil {
		return
	}
	ts := report.Timestamp
	if ts.IsZero() {
		ts = s.now()
	}
	kind := strings.TrimSpace(report.Kind)
	if kind == "" {
		kind = runtimeActivityKindAgentWaiting
	}
	if kind == runtimeActivityKindAgentWaiting {
		s.recordWaitingHeartbeat(ts, report.Detail)
		return
	}
	s.touch(ts, kind, report.Detail)
}

func (s *promptActivitySupervisor) observeEvent(event acp.AgentEvent) {
	if s == nil {
		return
	}
	kind, detail, currentTool, toolCallID, clearTool := activityFromEvent(event)
	if kind == "" {
		return
	}
	s.touchWithTool(event.Timestamp, kind, detail, currentTool, toolCallID, clearTool)
}

func (s *promptActivitySupervisor) finish(now time.Time) {
	if s == nil || s.session == nil {
		return
	}
	if now.IsZero() {
		now = s.now()
	}
	s.session.clearRuntimeActivity(now)
	if err := s.manager.persistSessionMetadataOnly(s.session); err != nil {
		s.manager.sessionLogger(s.session).
			Warn("session: persist runtime activity clear failed", "turn_id", s.turnID, "error", err)
	}
	healthCtx, cancel := s.manager.detachedSessionHealthContext(s.ctx)
	defer cancel()
	if _, err := s.manager.persistSessionIdlePresence(healthCtx, s.session, now); err != nil {
		s.manager.sessionLogger(s.session).
			Warn("session: persist runtime idle health failed", "turn_id", s.turnID, "error", err)
	}
}

func (s *promptActivitySupervisor) emitRecoveredMarkerIfNeeded(reason string) {
	if s == nil || s.manager == nil || s.session == nil {
		return
	}
	if strings.TrimSpace(reason) == "" {
		return
	}
	s.manager.emitTranscriptMarker(
		s.ctx,
		s.session,
		s.turnID,
		transcript.MarkerSessionRecovered,
		"Runtime activity recovered.",
		map[string]any{runtimeActivityEvidenceStallReason: reason},
	)
}

func (s *promptActivitySupervisor) run() {
	defer close(s.done)
	defer close(s.events)

	interval := s.config.ActivityHeartbeatInterval
	if interval <= 0 {
		interval = aghconfig.DefaultSessionSupervisionConfig().ActivityHeartbeatInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var deadlineTimer *time.Timer
	var deadlineCh <-chan time.Time
	if deadlineAt := cloneTimePointer(s.deadlineAt); deadlineAt != nil {
		wait := max(time.Until(deadlineAt.UTC()), 0)
		deadlineTimer = time.NewTimer(wait)
		deadlineCh = deadlineTimer.C
		defer deadlineTimer.Stop()
	}

	for {
		select {
		case <-s.ctx.Done():
			return
		case now := <-ticker.C:
			if s.evaluate(now.UTC()) {
				return
			}
		case now := <-deadlineCh:
			s.handlePromptDeadline(now.UTC())
			return
		}
	}
}

func (s *promptActivitySupervisor) evaluate(now time.Time) bool {
	if s == nil {
		return false
	}
	processUnhealthy := s.handleUnhealthyProcess(now, true)
	if !processUnhealthy && s.shouldEmitProgress(now) {
		s.emitRuntimeEvent(acp.EventTypeRuntimeProgress, s.progressText(now), now, nil)
	}
	if s.shouldEmitWarning(now) {
		s.emitRuntimeEvent(acp.EventTypeRuntimeWarning, s.warningText(now), now, nil)
	}
	if s.shouldTimeout(now) {
		s.handleTimeout(now)
		return true
	}
	return false
}

func (s *promptActivitySupervisor) shouldEmitProgress(now time.Time) bool {
	if s.config.ProgressNotifyInterval <= 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	base := s.startedAt
	if s.activity.LastProgressAt != nil && !s.activity.LastProgressAt.IsZero() {
		base = s.activity.LastProgressAt.UTC()
	}
	if now.Sub(base) < s.config.ProgressNotifyInterval {
		return false
	}
	progressAt := now.UTC()
	s.activity.LastProgressAt = &progressAt
	s.activity.IdleSeconds = store.SessionActivityIdleSeconds(&s.activity, now)
	return true
}

func (s *promptActivitySupervisor) shouldEmitWarning(now time.Time) bool {
	if s.config.InactivityWarningAfter <= 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.warned || s.idleSecondsLocked(now) < int64(s.config.InactivityWarningAfter.Seconds()) {
		return false
	}
	s.warned = true
	s.activity.LastActivityKind = runtimeActivityKindWarning
	s.activity.LastActivityDetail = "runtime activity is stale"
	s.activity.IdleSeconds = s.idleSecondsLocked(now)
	return true
}

func (s *promptActivitySupervisor) shouldTimeout(now time.Time) bool {
	if s.config.InactivityTimeout <= 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.timedOut || s.idleSecondsLocked(now) < int64(s.config.InactivityTimeout.Seconds()) {
		return false
	}
	s.timedOut = true
	s.activity.LastActivityKind = runtimeActivityKindTimeout
	s.activity.LastActivityDetail = "runtime activity timed out"
	s.activity.IdleSeconds = s.idleSecondsLocked(now)
	return true
}
