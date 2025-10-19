package usage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/require"
)

type logEvent struct {
	level  string
	msg    string
	fields map[string]any
}

type capturingLog struct {
	mu      *sync.Mutex
	entries *[]logEvent
	fields  map[string]any
}

func newCapturingLog() *capturingLog {
	events := make([]logEvent, 0)
	return &capturingLog{mu: &sync.Mutex{}, entries: &events, fields: map[string]any{}}
}

func (l *capturingLog) Debug(msg string, keyvals ...any) {
	l.record("debug", msg, keyvals...)
}

func (l *capturingLog) Info(msg string, keyvals ...any) {
	l.record("info", msg, keyvals...)
}

func (l *capturingLog) Warn(msg string, keyvals ...any) {
	l.record("warn", msg, keyvals...)
}

func (l *capturingLog) Error(msg string, keyvals ...any) {
	l.record("error", msg, keyvals...)
}

func (l *capturingLog) With(args ...any) logger.Logger {
	if l.mu == nil {
		l.mu = &sync.Mutex{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	nextFields := core.CloneMap(l.fields)
	if nextFields == nil {
		nextFields = make(map[string]any, len(args)/2)
	}
	for i := 0; i < len(args); i += 2 {
		key := fmt.Sprint(args[i])
		var val any
		if i+1 < len(args) {
			val = args[i+1]
		}
		nextFields[key] = val
	}
	return &capturingLog{mu: l.mu, entries: l.entries, fields: nextFields}
}

func (l *capturingLog) record(level, msg string, keyvals ...any) {
	if l.mu == nil {
		l.mu = &sync.Mutex{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.entries == nil {
		return
	}
	fields := core.CloneMap(l.fields)
	if fields == nil {
		fields = make(map[string]any, len(keyvals)/2)
	}
	for i := 0; i < len(keyvals); i += 2 {
		key := fmt.Sprint(keyvals[i])
		var val any
		if i+1 < len(keyvals) {
			val = keyvals[i+1]
		}
		fields[key] = val
	}
	*l.entries = append(*l.entries, logEvent{level: level, msg: msg, fields: fields})
}

func (l *capturingLog) snapshot() []logEvent {
	if l.mu == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.entries == nil {
		return nil
	}
	out := make([]logEvent, len(*l.entries))
	copy(out, *l.entries)
	return out
}

func TestRepositoryPersistInvokesCallback(t *testing.T) {
	t.Run("Should invoke persist callback and clone metadata", func(t *testing.T) {
		t.Helper()
		var received atomic.Pointer[Finalized]
		repo, err := NewRepository(func(_ context.Context, finalized *Finalized) error {
			received.Store(finalized)
			return nil
		}, &RepositoryOptions{QueueCapacity: 4, WorkerCount: 1})
		require.NoError(t, err)
		t.Cleanup(repo.Stop)

		agentID := "agent-123"
		summary := &Summary{Entries: []Entry{{
			Provider:         "openai",
			Model:            "gpt-4o-mini",
			PromptTokens:     10,
			CompletionTokens: 5,
		}}}
		finalized := &Finalized{
			Metadata: Metadata{
				Component:      core.ComponentTask,
				WorkflowExecID: core.MustNewID(),
				TaskExecID:     core.MustNewID(),
				AgentID:        &agentID,
			},
			Summary: summary,
		}

		ctx := logger.ContextWithLogger(t.Context(), logger.NewLogger(logger.TestConfig()))
		require.NoError(t, repo.Persist(ctx, finalized))

		require.Eventually(t, func() bool { return received.Load() != nil }, time.Second, 10*time.Millisecond)

		result := received.Load()
		require.NotNil(t, result)
		require.NotSame(t, summary, result.Summary)
		require.Equal(t, len(summary.Entries), len(result.Summary.Entries))
		require.NotSame(t, &summary.Entries[0], &result.Summary.Entries[0])
		require.NotNil(t, result.Metadata.AgentID)
		require.NotSame(t, finalized.Metadata.AgentID, result.Metadata.AgentID)
		require.Equal(t, *finalized.Metadata.AgentID, *result.Metadata.AgentID)
	})
}

func TestRepositoryStopPreventsNewRequests(t *testing.T) {
	t.Run("Should return ErrRepositoryClosed and allow repeated Stop", func(t *testing.T) {
		repo, err := NewRepository(func(_ context.Context, _ *Finalized) error { return nil }, nil)
		require.NoError(t, err)

		repo.Stop()
		require.ErrorIs(t, repo.Persist(t.Context(), &Finalized{
			Summary: &Summary{Entries: []Entry{
				{
					Provider: "openai",
					Model:    "gpt",
				},
			}},
		}), ErrRepositoryClosed)
		require.NotPanics(t, repo.Stop)
	})
}

func TestCategorizePersistenceError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "Should map nil to unknown", err: nil, want: repositoryErrorUnknown},
		{name: "Should map deadline", err: context.DeadlineExceeded, want: repositoryErrorTimeout},
		{name: "Should map canceled", err: context.Canceled, want: repositoryErrorTimeout},
		{name: "Should map not found", err: errors.New("task not found"), want: repositoryErrorValidation},
		{name: "Should map validation", err: errors.New("validation failed"), want: repositoryErrorValidation},
		{name: "Should map timeout substring", err: errors.New("db timeout"), want: repositoryErrorTimeout},
		{name: "Should default to database", err: errors.New("database unavailable"), want: repositoryErrorDatabase},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, categorizePersistenceError(tt.err))
		})
	}
}

func TestRepositoryPersistReturnsContextErrorWhenQueueFull(t *testing.T) {
	t.Run("Should return context error when queue is blocked", func(t *testing.T) {
		queue := make(chan *persistRequest, 1)
		queue <- &persistRequest{}
		repo := newRepositoryForTest(nil, queue)
		ctx := logger.ContextWithLogger(t.Context(), logger.NewLogger(logger.TestConfig()))
		ctx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
		defer cancel()
		err := repo.Persist(ctx, &Finalized{
			Summary: &Summary{Entries: []Entry{
				{
					Provider: "openai",
					Model:    "gpt",
				},
			}},
		})
		require.ErrorIs(t, err, context.DeadlineExceeded)
		select {
		case <-queue:
		default:
		}
	})
}

func TestRepositoryPersistRejectsNilContext(t *testing.T) {
	t.Run("Should return error when ctx is nil", func(t *testing.T) {
		repo, err := NewRepository(func(_ context.Context, _ *Finalized) error {
			return nil
		}, nil)
		require.NoError(t, err)
		t.Cleanup(repo.Stop)

		var nilCtx context.Context
		err = repo.Persist(nilCtx, &Finalized{
			Summary: &Summary{Entries: []Entry{
				{
					Provider: "openai",
					Model:    "gpt-4o-mini",
				},
			}},
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "nil context")
	})
}

func TestRepositoryRecoverFromPersistPanic(t *testing.T) {
	t.Run("Should recover from panic and warn", func(t *testing.T) {
		var callCount atomic.Int64
		var processed atomic.Bool
		repo, err := NewRepository(func(_ context.Context, _ *Finalized) error {
			switch callCount.Add(1) {
			case 1:
				panic("boom")
			default:
				processed.Store(true)
				return nil
			}
		}, &RepositoryOptions{QueueCapacity: 4, WorkerCount: 1})
		require.NoError(t, err)
		t.Cleanup(repo.Stop)

		log := newCapturingLog()
		finalized := &Finalized{
			Summary: &Summary{Entries: []Entry{
				{
					Provider: "openai",
					Model:    "gpt-4o-mini",
				},
			}},
		}
		ctx := logger.ContextWithLogger(t.Context(), log)

		require.NoError(t, repo.Persist(ctx, finalized))
		require.NoError(t, repo.Persist(ctx, finalized))

		require.Eventually(t, processed.Load, time.Second, 10*time.Millisecond)
		require.GreaterOrEqual(t, callCount.Load(), int64(2))
		require.Eventually(t, func() bool { return len(log.snapshot()) > 0 }, time.Second, 10*time.Millisecond)
		events := log.snapshot()
		last := events[len(events)-1]
		require.Equal(t, "warn", last.level)
		require.Equal(t, "Failed to persist usage summary", last.msg)
		require.NotNil(t, last.fields["error"])
	})
}
