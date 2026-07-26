package daemon

import (
	"bufio"

	"context"

	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/fileutil"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	extractorpkg "github.com/compozy/agh/internal/memory/extractor"

	"github.com/compozy/agh/internal/session"
)

const (
	memoryRuntimeBuiltinKey = "builtin"
	memoryRuntimeValueKey   = "value"
)

const (
	memoryExtractorConsumeInterval = time.Second
	memoryExtractorStopTimeout     = 10 * time.Second
	memoryExtractorSyntheticTaskID = "memory-extractor"
)

type memoryExtractorSessionManager interface {
	Spawn(context.Context, session.SpawnOpts) (*session.Session, error)
	PromptSynthetic(context.Context, string, session.SyntheticPromptOpts) (<-chan acp.AgentEvent, error)
	StopWithCause(context.Context, string, session.StopCause, string) error
}

type daemonMemoryExtractor struct {
	runtime        *extractorpkg.Runtime
	consumer       *extractorpkg.InboxConsumer
	failuresDir    string
	proposalSink   extractorpkg.ProposalSink
	logger         *slog.Logger
	now            func() time.Time
	workspaceRoots *sync.Map

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func newDaemonMemoryExtractor(
	ctx context.Context,
	state *bootState,
	sessions SessionManager,
	now func() time.Time,
) (*daemonMemoryExtractor, error) {
	if state == nil || state.memoryStore == nil || !state.cfg.Memory.Enabled {
		return nil, nil
	}
	forkSessions, ok := sessions.(memoryExtractorSessionManager)
	if !ok {
		return nil, errors.New("daemon: session manager does not implement memory extractor spawn surface")
	}
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}
	workspaceRoots := &sync.Map{}
	forked := &forkedMemoryExtractor{
		sessions:       forkSessions,
		roles:          roleResolverForState(state),
		deadline:       state.cfg.Memory.Extractor.Deadline,
		logger:         state.logger,
		now:            now,
		workspaceRoots: workspaceRoots,
	}
	runtime, err := extractorpkg.NewRuntime(
		context.WithoutCancel(ctx),
		state.globalMemoryDir,
		forked,
		extractorpkg.WithEventSink(state.memoryStore),
		extractorpkg.WithLogger(state.logger),
		extractorpkg.WithClock(now),
		extractorpkg.WithCoalesceMax(state.cfg.Memory.Extractor.Queue.CoalesceMax),
		extractorpkg.WithQueueCapacity(state.cfg.Memory.Extractor.Queue.Capacity),
		extractorpkg.WithThrottleTurns(state.cfg.Memory.Extractor.ThrottleTurns),
		extractorpkg.WithInboxPath(state.cfg.Memory.Extractor.InboxPath),
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: create memory extractor runtime: %w", err)
	}
	sink := &daemonMemoryProposalSink{
		base:              state.memoryStore,
		workspaceResolver: state.workspaceResolver,
	}
	consumer, err := extractorpkg.NewInboxConsumer(
		state.globalMemoryDir,
		sink,
		extractorpkg.WithConsumerEventSink(state.memoryStore),
		extractorpkg.WithConsumerLogger(state.logger),
		extractorpkg.WithConsumerClock(now),
		extractorpkg.WithConsumerInboxPath(state.cfg.Memory.Extractor.InboxPath),
		extractorpkg.WithConsumerFailurePath(state.cfg.Memory.Extractor.DLQPath),
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: create memory extractor inbox consumer: %w", err)
	}
	return &daemonMemoryExtractor{
		runtime:        runtime,
		consumer:       consumer,
		failuresDir:    extractorFailureDir(state),
		proposalSink:   sink,
		logger:         state.logger,
		now:            now,
		workspaceRoots: workspaceRoots,
		done:           make(chan struct{}),
	}, nil
}

func extractorFailureDir(state *bootState) string {
	if state == nil {
		return ""
	}
	if path := strings.TrimSpace(state.cfg.Memory.Extractor.DLQPath); path != "" {
		return filepath.Clean(path)
	}
	return filepath.Join(state.globalMemoryDir, "_system", "extractor", "failures")
}

func (e *daemonMemoryExtractor) Start(ctx context.Context) error {
	if e == nil || e.consumer == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: memory extractor start context is required")
	}
	e.mu.Lock()
	if e.cancel != nil {
		e.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	e.done = make(chan struct{})
	e.mu.Unlock()

	go func() {
		defer close(e.done)
		ticker := time.NewTicker(memoryExtractorConsumeInterval)
		defer ticker.Stop()
		e.consumeOnce(runCtx)
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				e.consumeOnce(runCtx)
			}
		}
	}()
	return nil
}

func (e *daemonMemoryExtractor) Close(ctx context.Context) error {
	if e == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: memory extractor close context is required")
	}
	e.mu.Lock()
	cancel := e.cancel
	done := e.done
	e.cancel = nil
	e.mu.Unlock()
	if cancel != nil {
		cancel()
		if done != nil {
			select {
			case <-done:
			case <-ctx.Done():
				return fmt.Errorf("daemon: wait memory extractor consumer: %w", ctx.Err())
			}
		}
	}
	if e.runtime != nil {
		if err := e.runtime.Close(ctx); err != nil {
			return err
		}
	}
	e.consumeOnce(ctx)
	return nil
}

func (e *daemonMemoryExtractor) Status(context.Context) (contract.MemoryExtractorStatusPayload, error) {
	if e == nil || e.runtime == nil {
		return contract.MemoryExtractorStatusPayload{Status: contract.MemoryExtractorStateStopped}, nil
	}
	stats := e.runtime.Stats()
	status := contract.MemoryExtractorStateIdle
	if stats.Closed {
		status = contract.MemoryExtractorStateStopped
	} else if stats.InFlightSessions > 0 || stats.QueuedSessions > 0 {
		status = contract.MemoryExtractorStateRunning
	}
	failureCount, err := e.failureCount()
	if err != nil {
		return contract.MemoryExtractorStatusPayload{}, err
	}
	return contract.MemoryExtractorStatusPayload{
		Status:                 status,
		QueuedSessions:         stats.QueuedSessions,
		InFlightSessions:       stats.InFlightSessions,
		ActiveProviderSessions: stats.ActiveProviderSessions,
		DroppedTurns:           intFromInt64(stats.DroppedTurns),
		CoalescedTurns:         intFromInt64(stats.CoalescedTurns),
		SkippedTurns:           intFromInt64(stats.SkippedTurns),
		BackpressuredSessions:  intFromInt64(stats.BackpressuredSessions),
		FailureCount:           failureCount,
	}, nil
}

func (e *daemonMemoryExtractor) ListFailures(context.Context) ([]contract.MemoryExtractorFailurePayload, error) {
	if e == nil {
		return []contract.MemoryExtractorFailurePayload{}, nil
	}
	failures, err := e.loadFailures()
	if err != nil {
		return nil, err
	}
	payloads := make([]contract.MemoryExtractorFailurePayload, 0, len(failures))
	for _, failure := range failures {
		payloads = append(payloads, failure.Payload)
	}
	return payloads, nil
}

func (e *daemonMemoryExtractor) Retry(
	ctx context.Context,
	req contract.MemoryExtractorRetryRequest,
) (contract.MemoryExtractorRetryResponse, error) {
	if e == nil || e.proposalSink == nil {
		return contract.MemoryExtractorRetryResponse{}, errors.New("daemon: memory extractor is not configured")
	}
	failures, err := e.loadFailures()
	if err != nil {
		return contract.MemoryExtractorRetryResponse{}, err
	}
	targetFailureID := strings.TrimSpace(req.FailureID)
	targetSessionID := strings.TrimSpace(req.SessionID)
	var response contract.MemoryExtractorRetryResponse
	for _, failure := range failures {
		if targetFailureID != "" && failure.Payload.ID != targetFailureID {
			continue
		}
		if targetSessionID != "" && failure.Payload.SessionID != targetSessionID {
			continue
		}
		if err := ctx.Err(); err != nil {
			return response, fmt.Errorf("daemon: retry memory extractor failures: %w", err)
		}
		candidates, decodeErr := failure.Candidates()
		if decodeErr != nil {
			response.Failed++
			continue
		}
		if len(candidates) == 0 {
			response.Failed++
			continue
		}
		var failed bool
		for _, candidate := range candidates {
			if _, proposeErr := e.proposalSink.ProposeCandidate(ctx, candidate); proposeErr != nil {
				failed = true
				break
			}
		}
		if failed {
			response.Failed++
			continue
		}
		if err := fileutil.AtomicRemoveFile(failure.Payload.Path); err != nil {
			return response, fmt.Errorf("daemon: remove retried extractor failure: %w", err)
		}
		response.Retried++
	}
	return response, nil
}

func (e *daemonMemoryExtractor) Drain(ctx context.Context) (contract.MemoryExtractorDrainResponse, error) {
	if e == nil || e.runtime == nil {
		return contract.MemoryExtractorDrainResponse{DrainedAt: e.nowUTC()}, nil
	}
	if err := e.runtime.Drain(ctx); err != nil {
		return contract.MemoryExtractorDrainResponse{}, err
	}
	e.consumeOnce(ctx)
	stats := e.runtime.Stats()
	return contract.MemoryExtractorDrainResponse{
		DrainedAt: e.nowUTC(),
		Remaining: stats.QueuedSessions + stats.InFlightSessions,
	}, nil
}

func (e *daemonMemoryExtractor) consumeOnce(ctx context.Context) {
	if e == nil || e.consumer == nil {
		return
	}
	if _, err := e.consumer.ConsumeOnce(ctx); err != nil && ctx.Err() == nil && e.logger != nil {
		e.logger.Warn("daemon: memory extractor inbox consume failed", "error", err)
	}
}

func (e *daemonMemoryExtractor) failureCount() (int, error) {
	failures, err := e.loadFailures()
	if err != nil {
		return 0, err
	}
	return len(failures), nil
}

func (e *daemonMemoryExtractor) loadFailures() ([]extractorFailure, error) {
	dir := strings.TrimSpace(e.failuresDir)
	if dir == "" {
		return []extractorFailure{}, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []extractorFailure{}, nil
		}
		return nil, fmt.Errorf("daemon: read memory extractor failures: %w", err)
	}
	failures := make([]extractorFailure, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		failure, err := readExtractorFailure(path)
		if err != nil {
			return nil, err
		}
		failures = append(failures, failure)
	}
	slices.SortFunc(failures, func(a, b extractorFailure) int {
		if !a.Payload.CreatedAt.Equal(b.Payload.CreatedAt) {
			return a.Payload.CreatedAt.Compare(b.Payload.CreatedAt)
		}
		return strings.Compare(a.Payload.ID, b.Payload.ID)
	})
	return failures, nil
}

func (e *daemonMemoryExtractor) nowUTC() time.Time {
	if e != nil && e.now != nil {
		return e.now().UTC()
	}
	return time.Now().UTC()
}

type extractorFailure struct {
	Payload contract.MemoryExtractorFailurePayload
	Report  extractorFailureReport
}

type extractorFailureReport struct {
	Stage      string `json:"stage"`
	Source     string `json:"source"`
	Error      string `json:"error"`
	Content    string `json:"content"`
	RecordedAt string `json:"recorded_at"`
}

func readExtractorFailure(path string) (extractorFailure, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return extractorFailure{}, fmt.Errorf("daemon: read extractor failure %q: %w", path, err)
	}
	var report extractorFailureReport
	if err := json.Unmarshal(data, &report); err != nil {
		return extractorFailure{}, fmt.Errorf("daemon: decode extractor failure %q: %w", path, err)
	}
	createdAt := parseMemoryTime(report.RecordedAt)
	if createdAt.IsZero() {
		if info, statErr := os.Stat(path); statErr == nil {
			createdAt = info.ModTime().UTC()
		}
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	sessionID, workspaceID, agentName := failureCandidateMetadata(report.Content)
	return extractorFailure{
		Payload: contract.MemoryExtractorFailurePayload{
			ID:          strings.TrimSuffix(filepath.Base(path), ".json"),
			SessionID:   sessionID,
			WorkspaceID: workspaceID,
			AgentName:   agentName,
			Reason:      firstNonEmptyString(report.Error, report.Stage),
			Path:        path,
			CreatedAt:   createdAt,
		},
		Report: report,
	}, nil
}

func (f extractorFailure) Candidates() ([]memcontract.Candidate, error) {
	scanner := bufio.NewScanner(strings.NewReader(f.Report.Content))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	candidates := make([]memcontract.Candidate, 0)
	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		var candidate memcontract.Candidate
		if err := json.Unmarshal([]byte(raw), &candidate); err != nil {
			return nil, fmt.Errorf("daemon: decode extractor failure candidate: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("daemon: scan extractor failure candidates: %w", err)
	}
	return candidates, nil
}
