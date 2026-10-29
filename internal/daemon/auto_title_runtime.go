package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	eventspkg "github.com/compozy/compozy/internal/events"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

const (
	autoTitleQueueCapacity = 128
	// autoTitleMaxAttempts bounds title generation per session; a failed attempt
	// is retried on a later turn rather than in a tight loop.
	autoTitleMaxAttempts = 3
)

var errAutoTitleQueueFull = errors.New("daemon: automatic title queue is full")

type autoTitleSessions interface {
	Events(context.Context, string, store.EventQuery) ([]store.SessionEvent, error)
	ApplyAutomaticSessionTitle(context.Context, string, string) (bool, error)
}

type autoTitleRuntime struct {
	sessions  autoTitleSessions
	generator autoTitleGenerator
	deadline  time.Duration
	logger    *slog.Logger

	events roleEventSummaryWriter

	mu       sync.Mutex
	queue    []autoTitleJob
	claimed  map[string]struct{}
	failures map[string]autoTitleFailure
	wake     chan struct{}
	done     chan struct{}
	cancel   context.CancelFunc
	started  bool
	closing  bool
}

// autoTitleFailure fences retries of a session whose title attempt failed: the
// next attempt waits for a different turn and stops after autoTitleMaxAttempts.
type autoTitleFailure struct {
	attempts int
	turnID   string
}

type autoTitleRuntimeOption func(*autoTitleRuntime)

// withAutoTitleEventWriter records failed attempts as durable role events.
func withAutoTitleEventWriter(events roleEventSummaryWriter) autoTitleRuntimeOption {
	return func(r *autoTitleRuntime) {
		r.events = events
	}
}

type autoTitleJob struct {
	sessionID     string
	profileID     string
	agentName     string
	workspaceID   string
	turnID        string
	assistantText string
}

func newAutoTitleRuntime(
	sessions autoTitleSessions,
	generator autoTitleGenerator,
	deadline time.Duration,
	logger *slog.Logger,
	opts ...autoTitleRuntimeOption,
) *autoTitleRuntime {
	if logger == nil {
		logger = slog.Default()
	}
	runtime := &autoTitleRuntime{
		sessions:  sessions,
		generator: generator,
		deadline:  deadline,
		logger:    logger,
		claimed:   make(map[string]struct{}),
		failures:  make(map[string]autoTitleFailure),
		wake:      make(chan struct{}, 1),
		done:      make(chan struct{}),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(runtime)
		}
	}
	return runtime
}

func (r *autoTitleRuntime) Start(ctx context.Context) error {
	if r == nil {
		return errors.New("daemon: automatic title runtime is required")
	}
	if ctx == nil {
		return errors.New("daemon: automatic title start context is required")
	}
	if r.sessions == nil || r.generator == nil {
		return errors.New("daemon: automatic title dependencies are required")
	}
	if r.deadline <= 0 {
		return errors.New("daemon: automatic title deadline must be positive")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return nil
	}
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	r.cancel = cancel
	r.done = make(chan struct{})
	r.started = true
	r.closing = false
	go r.run(runCtx)
	return nil
}

func (r *autoTitleRuntime) HandleSessionMessagePersisted(
	_ context.Context,
	payload hookspkg.SessionMessagePersistedPayload,
) error {
	if r == nil || !eligibleAutoTitlePayload(payload) {
		return nil
	}
	job := autoTitleJob{
		sessionID:     strings.TrimSpace(payload.SessionID),
		profileID:     strings.TrimSpace(payload.ProfileID),
		agentName:     strings.TrimSpace(payload.AgentName),
		workspaceID:   firstNonEmptyString(payload.WorkspaceID, payload.Workspace),
		turnID:        strings.TrimSpace(payload.TurnID),
		assistantText: strings.TrimSpace(payload.Text),
	}
	r.mu.Lock()
	if !r.started || r.closing {
		r.mu.Unlock()
		return nil
	}
	if _, exists := r.claimed[job.sessionID]; exists {
		r.mu.Unlock()
		return nil
	}
	if failure, failed := r.failures[job.sessionID]; failed &&
		(failure.attempts >= autoTitleMaxAttempts || failure.turnID == job.turnID) {
		r.mu.Unlock()
		return nil
	}
	if len(r.queue) >= autoTitleQueueCapacity {
		r.mu.Unlock()
		return errAutoTitleQueueFull
	}
	r.claimed[job.sessionID] = struct{}{}
	r.queue = append(r.queue, job)
	r.mu.Unlock()
	r.signal()
	return nil
}

func eligibleAutoTitlePayload(payload hookspkg.SessionMessagePersistedPayload) bool {
	return strings.TrimSpace(payload.SessionID) != "" &&
		strings.TrimSpace(payload.AgentName) != "" &&
		strings.TrimSpace(payload.TurnID) != "" &&
		strings.EqualFold(strings.TrimSpace(payload.Role), "assistant") &&
		session.Type(strings.TrimSpace(payload.SessionType)) == session.SessionTypeUser &&
		strings.TrimSpace(payload.SessionName) == "" &&
		strings.TrimSpace(payload.Text) != ""
}

func (r *autoTitleRuntime) Shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: automatic title shutdown context is required")
	}
	r.mu.Lock()
	if !r.started {
		r.mu.Unlock()
		return nil
	}
	r.closing = true
	done := r.done
	cancel := r.cancel
	r.mu.Unlock()
	r.signal()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		if cancel != nil {
			cancel()
		}
		<-done
		return fmt.Errorf("daemon: wait automatic title runtime: %w", ctx.Err())
	}
}

func (r *autoTitleRuntime) run(ctx context.Context) {
	defer func() {
		r.mu.Lock()
		r.started = false
		r.cancel = nil
		r.mu.Unlock()
		close(r.done)
	}()
	for {
		job, ok, closing := r.nextJob()
		if ok {
			r.process(ctx, job)
			continue
		}
		if closing {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-r.wake:
		}
	}
}

func (r *autoTitleRuntime) nextJob() (autoTitleJob, bool, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.queue) == 0 {
		return autoTitleJob{}, false, r.closing
	}
	job := r.queue[0]
	r.queue[0] = autoTitleJob{}
	r.queue = r.queue[1:]
	return job, true, false
}

func (r *autoTitleRuntime) process(ctx context.Context, job autoTitleJob) {
	jobCtx, cancel := context.WithTimeout(ctx, r.deadline)
	defer cancel()
	events, err := r.sessions.Events(jobCtx, job.sessionID, store.EventQuery{
		Type:   acp.EventTypeUserMessage,
		TurnID: job.turnID,
		Limit:  1,
	})
	if err != nil {
		r.fail(ctx, job, "query persisted turn", err)
		return
	}
	userMessage, err := firstAutoTitleUserMessage(events)
	if err != nil {
		r.fail(ctx, job, "decode persisted turn", err)
		return
	}
	if userMessage == "" {
		return
	}
	title, err := r.generator.Generate(jobCtx, autoTitleRequest{
		SessionID:      job.sessionID,
		ProfileID:      job.profileID,
		AgentName:      job.agentName,
		WorkspaceID:    job.workspaceID,
		UserMessage:    userMessage,
		AssistantReply: job.assistantText,
	})
	if err != nil {
		if errors.Is(err, errAutoTitleRoleDisabled) {
			return
		}
		r.fail(ctx, job, "generate title", err)
		return
	}
	if _, err := r.sessions.ApplyAutomaticSessionTitle(jobCtx, job.sessionID, title); err != nil {
		r.fail(ctx, job, "persist title", err)
		return
	}
	r.mu.Lock()
	delete(r.failures, job.sessionID)
	r.mu.Unlock()
}

// fail releases the session's claim so a later turn can retry, until the
// attempt budget is spent, and records the failure for diagnostics.
func (r *autoTitleRuntime) fail(ctx context.Context, job autoTitleJob, operation string, err error) {
	r.mu.Lock()
	failure := r.failures[job.sessionID]
	failure.attempts++
	failure.turnID = job.turnID
	r.failures[job.sessionID] = failure
	delete(r.claimed, job.sessionID)
	r.mu.Unlock()
	retry := failure.attempts < autoTitleMaxAttempts
	r.warn(job, operation, err, failure.attempts, retry)
	if eventErr := r.recordFailure(ctx, job, operation, err, failure.attempts, retry); eventErr != nil {
		r.logger.Warn("daemon: record automatic title failure", "session_id", job.sessionID, "error", eventErr)
	}
}

func firstAutoTitleUserMessage(events []store.SessionEvent) (string, error) {
	for _, storedEvent := range events {
		if storedEvent.Type != acp.EventTypeUserMessage {
			continue
		}
		event, err := transcript.UnmarshalAgentEvent(storedEvent.Content)
		if err != nil {
			return "", err
		}
		if text := strings.TrimSpace(event.Text); text != "" {
			return text, nil
		}
	}
	return "", nil
}

func (r *autoTitleRuntime) signal() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (r *autoTitleRuntime) warn(job autoTitleJob, operation string, err error, attempt int, retry bool) {
	if r == nil || r.logger == nil || err == nil {
		return
	}
	r.logger.Warn(
		"daemon: automatic title operation failed",
		"operation", operation,
		"session_id", job.sessionID,
		"turn_id", job.turnID,
		"attempt", attempt,
		"max_attempts", autoTitleMaxAttempts,
		"retry_on_later_turn", retry,
		"error", err,
	)
}

// autoTitleFailureErrorBytes bounds the error text a failure event retains.
const autoTitleFailureErrorBytes = 1024

type autoTitleFailureEventPayload struct {
	Role             string `json:"role"`
	Operation        string `json:"operation"`
	TurnID           string `json:"turn_id"`
	Attempt          int    `json:"attempt"`
	MaxAttempts      int    `json:"max_attempts"`
	RetryOnLaterTurn bool   `json:"retry_on_later_turn"`
	Error            string `json:"error"`
}

func (r *autoTitleRuntime) recordFailure(
	ctx context.Context,
	job autoTitleJob,
	operation string,
	cause error,
	attempt int,
	retry bool,
) error {
	if r == nil || r.events == nil || cause == nil {
		return nil
	}
	content, err := json.Marshal(autoTitleFailureEventPayload{
		Role: string(compozyconfig.RoleAutoTitle), Operation: operation, TurnID: job.turnID,
		Attempt: attempt, MaxAttempts: autoTitleMaxAttempts, RetryOnLaterTurn: retry,
		Error: truncateUTF8Bytes(cause.Error(), autoTitleFailureErrorBytes),
	})
	if err != nil {
		return fmt.Errorf("daemon: marshal automatic title failure event: %w", err)
	}
	eventCtx, cancel := detachedDaemonOperationContext(ctx, roleEventWriteTimeout)
	defer cancel()
	summary := fmt.Sprintf("auto_title role failed to %s (attempt %d/%d)", operation, attempt, autoTitleMaxAttempts)
	if err := r.events.WriteEventSummary(eventCtx, daemonEventSummary(store.EventSummary{
		ProfileID: job.profileID, SessionID: job.sessionID, WorkspaceID: job.workspaceID,
		Type: eventspkg.RoleAutoTitleFailed, AgentName: job.agentName,
		Outcome: string(eventspkg.OutcomeFor(eventspkg.RoleAutoTitleFailed)),
		Summary: summary,
	}, content)); err != nil {
		return fmt.Errorf("daemon: record %s: %w", eventspkg.RoleAutoTitleFailed, err)
	}
	return nil
}
