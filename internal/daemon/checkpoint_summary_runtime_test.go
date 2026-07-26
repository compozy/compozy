package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/compozy/agh/internal/acp"
	extensionpkg "github.com/compozy/agh/internal/extension"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/transcript"
)

func TestCheckpointSummaryRuntime(t *testing.T) {
	t.Parallel()

	t.Run("Should select the workspace provider and join in-flight work during shutdown", func(t *testing.T) {
		t.Parallel()

		started := make(chan struct{})
		release := make(chan struct{})
		provider := &checkpointProviderStub{
			onSessionEnd: func(_ context.Context, record memcontract.SessionEndRecord) error {
				close(started)
				<-release
				if record.WorkspaceID != "ws-alpha" || record.SessionID != "sess-alpha" {
					return errors.New("checkpoint record identity mismatch")
				}
				if len(record.Snapshot.Messages) != 1 ||
					!strings.Contains(record.Snapshot.Messages[0].Content, "cobalt decision") {
					return errors.New("checkpoint transcript snapshot mismatch")
				}
				return nil
			},
		}
		registry := extensionpkg.NewMemoryProviderRegistry()
		if err := registry.Register(testutil.Context(t), extensionpkg.MemoryProviderRegistration{
			Name:     "fixture",
			Provider: provider,
		}); err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		if err := registry.SetActive(testutil.Context(t), "ws-alpha", "fixture"); err != nil {
			t.Fatalf("SetActive() error = %v", err)
		}
		events := []store.SessionEvent{checkpointRuntimeEvent(t, "cobalt decision")}
		runtime := newCheckpointSummaryRuntime(
			&checkpointRuntimeSessionStub{events: events},
			registry,
			time.Second,
			time.Second,
			slog.Default(),
			&checkpointLifecycleStub{},
		)
		if err := runtime.Start(testutil.Context(t)); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		runtime.OnSessionStopped(testutil.Context(t), &session.Session{
			ID:          "sess-alpha",
			Name:        "User session",
			AgentName:   "coder",
			WorkspaceID: "ws-alpha",
			Workspace:   "/workspace/alpha",
			Type:        session.SessionTypeUser,
			UpdatedAt:   time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC),
		})
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("provider session-end call did not start")
		}

		shutdownErr := make(chan error, 1)
		go func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			shutdownErr <- runtime.Shutdown(shutdownCtx)
		}()
		select {
		case err := <-shutdownErr:
			t.Fatalf("Shutdown() returned before provider release: %v", err)
		case <-time.After(100 * time.Millisecond):
		}
		close(release)
		if err := <-shutdownErr; err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	t.Run("Should bound queued session summaries and coalesce duplicate stops", func(t *testing.T) {
		t.Parallel()

		started := make(chan struct{})
		release := make(chan struct{})
		var blockFirst sync.Once
		provider := &checkpointProviderStub{onSessionEnd: func(
			context.Context,
			memcontract.SessionEndRecord,
		) error {
			blockFirst.Do(func() {
				close(started)
				<-release
			})
			return nil
		}}
		registry := extensionpkg.NewMemoryProviderRegistry()
		if err := registry.Register(testutil.Context(t), extensionpkg.MemoryProviderRegistration{
			Name: "fixture", Provider: provider,
		}); err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		if err := registry.SetActive(testutil.Context(t), "ws-alpha", "fixture"); err != nil {
			t.Fatalf("SetActive() error = %v", err)
		}
		runtime := newCheckpointSummaryRuntime(
			&checkpointRuntimeSessionStub{events: []store.SessionEvent{checkpointRuntimeEvent(t, "decision")}},
			registry,
			time.Second,
			time.Second,
			slog.Default(),
			&checkpointLifecycleStub{},
		)
		if err := runtime.Start(testutil.Context(t)); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		enqueue := func(sessionID string, endedAt time.Time) {
			runtime.OnSessionStopped(testutil.Context(t), &session.Session{
				ID: sessionID, Name: "User session", AgentName: "coder", WorkspaceID: "ws-alpha",
				Workspace: "/workspace/alpha", Type: session.SessionTypeUser, UpdatedAt: endedAt,
			})
		}
		enqueue("sess-active", time.Now())
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("provider session-end call did not start")
		}
		baseTime := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
		for idx := range checkpointSummaryQueueCapacity + 5 {
			enqueue(fmt.Sprintf("sess-%03d", idx), baseTime.Add(time.Duration(idx)*time.Second))
		}
		enqueue(
			fmt.Sprintf("sess-%03d", checkpointSummaryQueueCapacity+4),
			baseTime.Add(time.Hour),
		)

		runtime.mu.Lock()
		queued := len(runtime.queue)
		dropped := runtime.dropped
		coalesced := runtime.coalesced
		runtime.mu.Unlock()
		if queued != checkpointSummaryQueueCapacity {
			t.Fatalf("queued summaries = %d, want %d", queued, checkpointSummaryQueueCapacity)
		}
		if dropped != 5 {
			t.Fatalf("dropped summaries = %d, want 5", dropped)
		}
		if coalesced != 1 {
			t.Fatalf("coalesced summaries = %d, want 1", coalesced)
		}

		close(release)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := runtime.Shutdown(shutdownCtx); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	t.Run("Should include consolidation sessions without recursing into checkpoint sessions", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name     string
			info     session.Info
			eligible bool
		}{
			{
				name:     "ordinary dream",
				info:     session.Info{Name: "Memory consolidation", Type: session.SessionTypeDream},
				eligible: true,
			},
			{
				name:     "internal checkpoint dream",
				info:     session.Info{Name: checkpointSummarySessionName, Type: session.SessionTypeDream},
				eligible: false,
			},
			{
				name:     "different-case user name",
				info:     session.Info{Name: "CHECKPOINT SUMMARY", Type: session.SessionTypeDream},
				eligible: true,
			},
			{
				name:     "spawned worker",
				info:     session.Info{Name: "Worker", Type: session.SessionTypeSpawned},
				eligible: false,
			},
		}
		for _, tc := range cases {
			t.Run("Should classify "+tc.name, func(t *testing.T) {
				t.Parallel()

				if got := checkpointSummaryEligible(&tc.info); got != tc.eligible {
					t.Fatalf("checkpointSummaryEligible() = %t, want %t", got, tc.eligible)
				}
			})
		}
	})

	t.Run("Should bound a single oversized transcript message", func(t *testing.T) {
		t.Parallel()

		snapshot, err := checkpointTranscriptSnapshot([]store.SessionEvent{
			checkpointRuntimeEvent(t, strings.Repeat("é", checkpointSummaryMaxInputBytes)),
		})
		if err != nil {
			t.Fatalf("checkpointTranscriptSnapshot() error = %v", err)
		}
		if len(snapshot.Messages) != 1 {
			t.Fatalf("checkpoint transcript messages = %d, want 1", len(snapshot.Messages))
		}
		content := snapshot.Messages[0].Content
		if len(content) > checkpointSummaryMaxInputBytes {
			t.Fatalf(
				"checkpoint transcript bytes = %d, want at most %d",
				len(content),
				checkpointSummaryMaxInputBytes,
			)
		}
		if !utf8.ValidString(content) || !strings.HasSuffix(content, checkpointSummaryTruncated) {
			t.Fatalf("checkpoint transcript tail = %q, want valid UTF-8 truncation marker", content[len(content)-64:])
		}
	})

	t.Run("Should cover the span before invoking one active external provider", func(t *testing.T) {
		t.Parallel()

		order := make([]string, 0, 2)
		service := &checkpointLifecycleStub{
			onPreCompress: func(
				_ context.Context,
				request memcontract.PreCompressRequest,
			) (memcontract.PreCompressHint, error) {
				order = append(order, "summary")
				if request.WorkspaceID != "ws-alpha" || request.SessionID != "sess-alpha" ||
					request.FromSequence != 1 || request.ToSequence != 1 || len(request.Snapshot.Messages) != 1 {
					return memcontract.PreCompressHint{}, errors.New("pre-compress request mismatch")
				}
				return memcontract.PreCompressHint{Markdown: "covered cobalt decision"}, nil
			},
		}
		providerCalls := 0
		provider := &checkpointProviderStub{
			onPreCompress: func(
				_ context.Context,
				_ memcontract.PreCompressRequest,
			) (memcontract.PreCompressHint, error) {
				providerCalls++
				order = append(order, "provider")
				return memcontract.PreCompressHint{}, nil
			},
		}
		registry := extensionpkg.NewMemoryProviderRegistry()
		if err := registry.Register(testutil.Context(t), extensionpkg.MemoryProviderRegistration{
			Name:     "external",
			Provider: provider,
		}); err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		if err := registry.SetActive(testutil.Context(t), "ws-alpha", "external"); err != nil {
			t.Fatalf("SetActive() error = %v", err)
		}
		runtime := newCheckpointSummaryRuntime(
			&checkpointRuntimeSessionStub{},
			registry,
			time.Second,
			time.Second,
			slog.Default(),
			service,
		)
		result, err := runtime.CompactSessionContext(testutil.Context(t), session.CompactionRequest{
			WorkspaceID:  "ws-alpha",
			SessionID:    "sess-alpha",
			AgentName:    "coder",
			FromSequence: 1,
			ToSequence:   1,
			Events:       []store.SessionEvent{checkpointRuntimeEvent(t, "cobalt decision")},
		})
		if err != nil {
			t.Fatalf("CompactSessionContext() error = %v", err)
		}
		if result.Summary != "covered cobalt decision" || providerCalls != 1 ||
			strings.Join(order, ",") != "summary,provider" {
			t.Fatalf("compaction result/calls/order = %#v / %d / %v", result, providerCalls, order)
		}
	})
}

func checkpointRuntimeEvent(t *testing.T, text string) store.SessionEvent {
	t.Helper()

	at := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	content, err := transcript.MarshalAgentEvent(acp.AgentEvent{
		Type:      acp.EventTypeUserMessage,
		TurnID:    "turn-1",
		Text:      text,
		Timestamp: at,
	})
	if err != nil {
		t.Fatalf("MarshalAgentEvent() error = %v", err)
	}
	return store.SessionEvent{
		ID:        "event-1",
		SessionID: "sess-alpha",
		TurnID:    "turn-1",
		Type:      acp.EventTypeUserMessage,
		Content:   content,
		Timestamp: at,
		Sequence:  1,
	}
}

type checkpointRuntimeSessionStub struct {
	events []store.SessionEvent
	err    error
}

func (s *checkpointRuntimeSessionStub) Events(
	_ context.Context,
	_ string,
	_ store.EventQuery,
) ([]store.SessionEvent, error) {
	return append([]store.SessionEvent(nil), s.events...), s.err
}

type checkpointProviderStub struct {
	onSessionEnd  func(context.Context, memcontract.SessionEndRecord) error
	onPreCompress func(context.Context, memcontract.PreCompressRequest) (memcontract.PreCompressHint, error)
}

func (p *checkpointProviderStub) Initialize(context.Context, memcontract.ProviderInit) error {
	return nil
}

func (p *checkpointProviderStub) SystemPromptBlock(
	context.Context,
	memcontract.SnapshotRequest,
) (memcontract.SnapshotResult, error) {
	return memcontract.SnapshotResult{}, nil
}

func (p *checkpointProviderStub) Recall(
	context.Context,
	memcontract.RecallRequest,
) (memcontract.RecallResult, error) {
	return memcontract.RecallResult{}, nil
}

func (p *checkpointProviderStub) Prefetch(context.Context, memcontract.PrefetchRequest) error {
	return nil
}

func (p *checkpointProviderStub) SyncTurn(context.Context, memcontract.TurnRecord) error {
	return nil
}

func (p *checkpointProviderStub) OnSessionEnd(
	ctx context.Context,
	record memcontract.SessionEndRecord,
) error {
	if p.onSessionEnd == nil {
		return nil
	}
	return p.onSessionEnd(ctx, record)
}

func (p *checkpointProviderStub) OnSessionSwitch(
	context.Context,
	memcontract.SessionSwitchRecord,
) error {
	return nil
}

func (p *checkpointProviderStub) OnPreCompress(
	ctx context.Context,
	request memcontract.PreCompressRequest,
) (memcontract.PreCompressHint, error) {
	if p.onPreCompress != nil {
		return p.onPreCompress(ctx, request)
	}
	return memcontract.PreCompressHint{}, nil
}

func (p *checkpointProviderStub) OnMemoryWrite(context.Context, memcontract.WriteRecord) error {
	return nil
}

func (p *checkpointProviderStub) Shutdown(context.Context) error {
	return nil
}

type checkpointLifecycleStub struct {
	onSessionEnd  func(context.Context, memcontract.SessionEndRecord) error
	onPreCompress func(context.Context, memcontract.PreCompressRequest) (memcontract.PreCompressHint, error)
}

func (s *checkpointLifecycleStub) OnSessionEnd(
	ctx context.Context,
	record memcontract.SessionEndRecord,
) error {
	if s.onSessionEnd == nil {
		return nil
	}
	return s.onSessionEnd(ctx, record)
}

func (s *checkpointLifecycleStub) OnPreCompress(
	ctx context.Context,
	request memcontract.PreCompressRequest,
) (memcontract.PreCompressHint, error) {
	if s.onPreCompress == nil {
		return memcontract.PreCompressHint{}, nil
	}
	return s.onPreCompress(ctx, request)
}
