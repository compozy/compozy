package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	extensionpkg "github.com/compozy/agh/internal/extension"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
)

const (
	checkpointSummaryMaxMessages   = 256
	checkpointSummaryMaxInputBytes = 192_000
	checkpointSummarySessionName   = "Checkpoint summary"
	checkpointSummaryTruncated     = "\n...[checkpoint input truncated]"
	checkpointSummaryQueueCapacity = 256
)

type checkpointSummarySessionEvents interface {
	Events(ctx context.Context, id string, query store.EventQuery) ([]store.SessionEvent, error)
}

type checkpointSummaryLifecycle interface {
	OnSessionEnd(ctx context.Context, record memcontract.SessionEndRecord) error
	OnPreCompress(
		ctx context.Context,
		request memcontract.PreCompressRequest,
	) (memcontract.PreCompressHint, error)
}

type checkpointSummaryRuntime struct {
	sessions        checkpointSummarySessionEvents
	providers       *extensionpkg.MemoryProviderRegistry
	providerTimeout time.Duration
	bundledTimeout  time.Duration
	logger          *slog.Logger
	lifecycle       checkpointSummaryLifecycle

	mu        sync.Mutex
	queue     []checkpointSummaryJob
	wake      chan struct{}
	done      chan struct{}
	cancel    context.CancelFunc
	started   bool
	closing   bool
	dropped   int64
	coalesced int64
}

type checkpointSummaryJob struct {
	sessionID   string
	agentName   string
	workspaceID string
	endedAt     time.Time
}

func newCheckpointSummaryRuntime(
	sessions checkpointSummarySessionEvents,
	providers *extensionpkg.MemoryProviderRegistry,
	providerTimeout time.Duration,
	bundledTimeout time.Duration,
	logger *slog.Logger,
	lifecycle checkpointSummaryLifecycle,
) *checkpointSummaryRuntime {
	if logger == nil {
		logger = slog.Default()
	}
	return &checkpointSummaryRuntime{
		sessions:        sessions,
		providers:       providers,
		providerTimeout: providerTimeout,
		bundledTimeout:  bundledTimeout,
		logger:          logger,
		lifecycle:       lifecycle,
		wake:            make(chan struct{}, 1),
		done:            make(chan struct{}),
	}
}

func (r *checkpointSummaryRuntime) Start(ctx context.Context) error {
	if r == nil {
		return errors.New("daemon: checkpoint summary runtime is required")
	}
	if ctx == nil {
		return errors.New("daemon: checkpoint summary start context is required")
	}
	if r.sessions == nil {
		return errors.New("daemon: checkpoint summary sessions are required")
	}
	if r.providers == nil {
		return errors.New("daemon: checkpoint summary provider registry is required")
	}
	if r.lifecycle == nil {
		return errors.New("daemon: checkpoint summary lifecycle is required")
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

func (r *checkpointSummaryRuntime) OnSessionCreated(context.Context, *session.Session) {}

func (r *checkpointSummaryRuntime) OnSessionStopped(_ context.Context, stopped *session.Session) {
	if r == nil || stopped == nil {
		return
	}
	info := stopped.Info()
	if info == nil || !checkpointSummaryEligible(info) {
		return
	}
	job := checkpointSummaryJob{
		sessionID:   strings.TrimSpace(info.ID),
		agentName:   strings.TrimSpace(info.AgentName),
		workspaceID: strings.TrimSpace(info.WorkspaceID),
		endedAt:     info.UpdatedAt.UTC(),
	}
	if job.sessionID == "" || job.workspaceID == "" {
		return
	}
	r.mu.Lock()
	if !r.started || r.closing {
		r.mu.Unlock()
		return
	}
	for idx := range r.queue {
		if r.queue[idx].sessionID != job.sessionID || r.queue[idx].workspaceID != job.workspaceID {
			continue
		}
		r.queue[idx] = job
		r.coalesced++
		r.mu.Unlock()
		return
	}
	var dropped checkpointSummaryJob
	if len(r.queue) >= checkpointSummaryQueueCapacity {
		dropped = r.queue[0]
		copy(r.queue, r.queue[1:])
		r.queue[len(r.queue)-1] = checkpointSummaryJob{}
		r.queue = r.queue[:len(r.queue)-1]
		r.dropped++
	}
	r.queue = append(r.queue, job)
	r.mu.Unlock()
	if dropped.sessionID != "" {
		r.logger.Warn(
			"checkpoint summary queue full; dropped oldest session",
			"session_id", dropped.sessionID,
			"workspace_id", dropped.workspaceID,
			"queue_capacity", checkpointSummaryQueueCapacity,
		)
	}
	r.signal()
}

func (r *checkpointSummaryRuntime) Shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: checkpoint summary shutdown context is required")
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
		return fmt.Errorf("daemon: wait checkpoint summary runtime: %w", ctx.Err())
	}
}

func (r *checkpointSummaryRuntime) run(ctx context.Context) {
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

func (r *checkpointSummaryRuntime) nextJob() (checkpointSummaryJob, bool, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.queue) == 0 {
		return checkpointSummaryJob{}, false, r.closing
	}
	job := r.queue[0]
	r.queue[0] = checkpointSummaryJob{}
	r.queue = r.queue[1:]
	return job, true, false
}

func (r *checkpointSummaryRuntime) process(ctx context.Context, job checkpointSummaryJob) {
	registration, err := r.providers.Select(ctx, job.workspaceID, "")
	if err != nil {
		r.warn(job, "select provider", err)
		return
	}
	timeout := r.providerTimeout
	if registration.Bundled {
		timeout = r.bundledTimeout
	}
	if timeout <= 0 {
		timeout = time.Minute
	}
	jobCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	events, err := r.sessions.Events(jobCtx, job.sessionID, store.EventQuery{
		Archive: store.EventArchiveUnarchived,
	})
	if err != nil {
		r.warn(job, "query events", err)
		return
	}
	snapshot, err := checkpointTranscriptSnapshot(events)
	if err != nil {
		r.warn(job, "assemble transcript", err)
		return
	}
	if len(snapshot.Messages) == 0 {
		return
	}
	record := memcontract.SessionEndRecord{
		WorkspaceID: job.workspaceID,
		SessionID:   job.sessionID,
		AgentName:   job.agentName,
		EndedAt:     job.endedAt,
		Snapshot:    snapshot,
	}
	if !registration.Bundled {
		if err := r.lifecycle.OnSessionEnd(jobCtx, record); err != nil {
			r.warn(job, "update canonical checkpoint", err)
			return
		}
	}
	if err := registration.Provider.OnSessionEnd(jobCtx, record); err != nil {
		r.warn(job, "update provider checkpoint", err)
	}
}

// CompactSessionContext updates canonical coverage and the active provider before archive.
func (r *checkpointSummaryRuntime) CompactSessionContext(
	ctx context.Context,
	request session.CompactionRequest,
) (session.CompactionResult, error) {
	if r == nil {
		return session.CompactionResult{}, errors.New("daemon: checkpoint summary runtime is required")
	}
	if ctx == nil {
		return session.CompactionResult{}, errors.New("daemon: checkpoint compaction context is required")
	}
	if r.providers == nil || r.lifecycle == nil {
		return session.CompactionResult{}, errors.New("daemon: checkpoint compaction lifecycle is unavailable")
	}
	if strings.TrimSpace(request.WorkspaceID) == "" || strings.TrimSpace(request.SessionID) == "" {
		return session.CompactionResult{}, errors.New("daemon: checkpoint compaction identity is required")
	}
	if request.FromSequence <= 0 || request.ToSequence < request.FromSequence {
		return session.CompactionResult{}, fmt.Errorf(
			"daemon: invalid checkpoint compaction range %d..%d",
			request.FromSequence,
			request.ToSequence,
		)
	}
	snapshot, err := checkpointTranscriptSnapshot(request.Events)
	if err != nil {
		return session.CompactionResult{}, fmt.Errorf("daemon: assemble compaction snapshot: %w", err)
	}
	if len(snapshot.Messages) == 0 {
		return session.CompactionResult{}, errors.New("daemon: checkpoint compaction snapshot is empty")
	}
	registration, err := r.providers.Select(ctx, request.WorkspaceID, "")
	if err != nil {
		return session.CompactionResult{}, fmt.Errorf("daemon: select compaction provider: %w", err)
	}
	timeout := r.providerTimeout
	if registration.Bundled {
		timeout = r.bundledTimeout
	}
	if timeout <= 0 {
		timeout = time.Minute
	}
	compactCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	providerRequest := memcontract.PreCompressRequest{
		WorkspaceID:  strings.TrimSpace(request.WorkspaceID),
		SessionID:    strings.TrimSpace(request.SessionID),
		AgentName:    strings.TrimSpace(request.AgentName),
		FromSequence: request.FromSequence,
		ToSequence:   request.ToSequence,
		Snapshot:     snapshot,
	}
	var canonicalHint memcontract.PreCompressHint
	if !registration.Bundled {
		canonicalHint, err = r.lifecycle.OnPreCompress(compactCtx, providerRequest)
		if err != nil {
			return session.CompactionResult{}, fmt.Errorf("daemon: update canonical compaction checkpoint: %w", err)
		}
	}
	providerHint, err := registration.Provider.OnPreCompress(compactCtx, providerRequest)
	if err != nil {
		return session.CompactionResult{}, fmt.Errorf("daemon: notify compaction provider: %w", err)
	}
	if registration.Bundled {
		canonicalHint = providerHint
	}
	return session.CompactionResult{
		Summary: firstNonEmptyString(canonicalHint.Markdown, providerHint.Markdown),
	}, nil
}

func (r *checkpointSummaryRuntime) signal() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (r *checkpointSummaryRuntime) warn(job checkpointSummaryJob, operation string, err error) {
	if r == nil || r.logger == nil || err == nil {
		return
	}
	r.logger.Warn(
		"daemon: checkpoint summary update failed",
		"operation", operation,
		"session_id", job.sessionID,
		"workspace_id", job.workspaceID,
		"error", err,
	)
}

func checkpointSummaryEligible(info *session.Info) bool {
	if info == nil || strings.TrimSpace(info.Name) == checkpointSummarySessionName {
		return false
	}
	switch info.Type {
	case session.SessionTypeUser, session.SessionTypeDream, session.SessionTypeCoordinator:
		return true
	default:
		return false
	}
}

func checkpointTranscriptSnapshot(events []store.SessionEvent) (memcontract.TranscriptSnapshot, error) {
	messages, err := transcript.Assemble(events)
	if err != nil {
		return memcontract.TranscriptSnapshot{}, err
	}
	messages = transcript.Prune(messages, transcript.PruneOptions{Dedup: true})
	if len(messages) > checkpointSummaryMaxMessages {
		messages = messages[len(messages)-checkpointSummaryMaxMessages:]
	}
	encoded := make([]memcontract.TranscriptMessage, 0, len(messages))
	totalBytes := 0
	for index, message := range slices.Backward(messages) {
		content, marshalErr := json.Marshal(message)
		if marshalErr != nil {
			return memcontract.TranscriptSnapshot{}, fmt.Errorf("marshal checkpoint transcript message: %w", marshalErr)
		}
		remaining := checkpointSummaryMaxInputBytes - totalBytes
		if remaining <= 0 || len(encoded) > 0 && len(content) > remaining {
			break
		}
		if len(content) > remaining {
			content = []byte(truncateCheckpointSummaryContent(string(content), remaining))
		}
		encoded = append(encoded, memcontract.TranscriptMessage{
			Sequence: int64(index + 1),
			Role:     string(message.Role),
			Content:  string(content),
			At:       message.Timestamp.UTC(),
		})
		totalBytes += len(content)
	}
	slices.Reverse(encoded)
	return memcontract.TranscriptSnapshot{Messages: encoded}, nil
}

func truncateCheckpointSummaryContent(content string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(content) <= maxBytes {
		return content
	}
	if maxBytes <= len(checkpointSummaryTruncated) {
		return truncateUTF8Bytes(checkpointSummaryTruncated, maxBytes)
	}
	prefix := truncateUTF8Bytes(content, maxBytes-len(checkpointSummaryTruncated))
	return prefix + checkpointSummaryTruncated
}

type checkpointMemoryShutdowner struct {
	runtime  *checkpointSummaryRuntime
	provider memoryProviderShutdowner
}

func (s checkpointMemoryShutdowner) Shutdown(ctx context.Context) error {
	var errs []error
	if s.runtime != nil {
		errs = append(errs, s.runtime.Shutdown(ctx))
	}
	if s.provider != nil {
		errs = append(errs, s.provider.Shutdown(ctx))
	}
	return errors.Join(errs...)
}
