package daemon

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/testutil"
)

type sessionWakeManagerStub struct {
	mu          sync.Mutex
	status      *session.Info
	statusErr   error
	promptErr   error
	promptID    string
	promptOpts  session.SyntheticPromptOpts
	promptCalls int
	events      chan acp.AgentEvent
	ctxErr      error
}

func (stub *sessionWakeManagerStub) Status(ctx context.Context, _ string) (*session.Info, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.ctxErr = ctx.Err()
	return stub.status, stub.statusErr
}

func (stub *sessionWakeManagerStub) PromptSynthetic(
	ctx context.Context,
	id string,
	opts session.SyntheticPromptOpts,
) (<-chan acp.AgentEvent, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.ctxErr = ctx.Err()
	stub.promptID = id
	stub.promptOpts = opts
	stub.promptCalls++
	return stub.events, stub.promptErr
}

func TestSessionWakeBridge(t *testing.T) {
	t.Parallel()

	t.Run("Should deliver detached bounded metadata and drain through shutdown", func(t *testing.T) {
		t.Parallel()

		events := make(chan acp.AgentEvent)
		manager := &sessionWakeManagerStub{
			status: &session.Info{ID: "parent-1", State: session.StateActive},
			events: events,
		}
		bridge, err := newSessionWakeBridge(
			testutil.Context(t),
			func() sessionWakeSessionManager { return manager },
			slog.New(slog.NewTextHandler(io.Discard, nil)),
		)
		if err != nil {
			t.Fatalf("newSessionWakeBridge() error = %v", err)
		}
		canceledCtx, cancel := context.WithCancel(testutil.Context(t))
		cancel()
		event := session.SpawnWakeEvent{
			ChildSessionID: "child-1",
			ChildAgentName: "researcher",
			Reason:         session.SpawnWakeReasonNeedsAttention,
			Badge:          session.BadgeWaitingForInput,
			Detail:         "Include vendored packages? compozy_claim_private-token" + strings.Repeat("界", 300),
			WakeEventID:    "wake-1",
		}

		if err := bridge.WakeSpawnCreator(canceledCtx, "parent-1", event); err != nil {
			t.Fatalf("WakeSpawnCreator() error = %v", err)
		}
		manager.mu.Lock()
		promptOpts := manager.promptOpts
		promptID := manager.promptID
		promptCalls := manager.promptCalls
		ctxErr := manager.ctxErr
		manager.mu.Unlock()
		if ctxErr != nil {
			t.Fatalf("delivery context error = %v, want detached context", ctxErr)
		}
		if promptCalls != 1 || promptID != "parent-1" || promptOpts.SkipIfBusy {
			t.Fatalf("prompt delivery = calls %d id %q opts %#v, want one queued wake", promptCalls, promptID, promptOpts)
		}
		if got := len([]rune(promptOpts.Message)); got > 240 {
			t.Fatalf("wake prompt runes = %d, want <= 240", got)
		}
		if strings.Contains(promptOpts.Message, "compozy_claim_private-token") {
			t.Fatalf("wake prompt leaked claim token: %q", promptOpts.Message)
		}
		if !strings.Contains(promptOpts.Message, "compozy__session_clarify_answer") ||
			!strings.Contains(promptOpts.Message, "compozy__session_events") {
			t.Fatalf("wake prompt = %q, want actionable native tools", promptOpts.Message)
		}
		if promptOpts.Metadata.ChildSessionID != "child-1" ||
			promptOpts.Metadata.ChildAgentName != "researcher" ||
			promptOpts.Metadata.Badge != string(session.BadgeWaitingForInput) ||
			promptOpts.Metadata.WakeEventID != "wake-1" {
			t.Fatalf("synthetic metadata = %#v, want exact child identity", promptOpts.Metadata)
		}
		if err := bridge.shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("shutdown() error = %v", err)
		}
	})

	t.Run("Should classify a stopped creator as not live", func(t *testing.T) {
		t.Parallel()

		manager := &sessionWakeManagerStub{status: &session.Info{ID: "parent-1", State: session.StateStopped}}
		bridge, err := newSessionWakeBridge(
			testutil.Context(t),
			func() sessionWakeSessionManager { return manager },
			nil,
		)
		if err != nil {
			t.Fatalf("newSessionWakeBridge() error = %v", err)
		}
		t.Cleanup(func() {
			if shutdownErr := bridge.shutdown(testutil.Context(t)); shutdownErr != nil {
				t.Errorf("shutdown() error = %v", shutdownErr)
			}
		})

		err = bridge.WakeSpawnCreator(testutil.Context(t), "parent-1", validSessionWakeEvent())
		if !errors.Is(err, session.ErrSpawnWakeSessionNotLive) {
			t.Fatalf("WakeSpawnCreator() error = %v, want ErrSpawnWakeSessionNotLive", err)
		}
		if manager.promptCalls != 0 {
			t.Fatalf("PromptSynthetic() calls = %d, want 0", manager.promptCalls)
		}
	})

	t.Run("Should reject wake admission after shutdown starts", func(t *testing.T) {
		t.Parallel()

		manager := &sessionWakeManagerStub{status: &session.Info{ID: "parent-1", State: session.StateActive}}
		bridge, err := newSessionWakeBridge(
			testutil.Context(t),
			func() sessionWakeSessionManager { return manager },
			nil,
		)
		if err != nil {
			t.Fatalf("newSessionWakeBridge() error = %v", err)
		}
		if err := bridge.shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("shutdown() error = %v", err)
		}
		if err := bridge.WakeSpawnCreator(testutil.Context(t), "parent-1", validSessionWakeEvent()); err == nil || !strings.Contains(err.Error(), "stopping") {
			t.Fatalf("WakeSpawnCreator() error = %v, want stopping rejection", err)
		}
	})
}

func validSessionWakeEvent() session.SpawnWakeEvent {
	return session.SpawnWakeEvent{
		ChildSessionID: "child-1",
		Reason:         session.SpawnWakeReasonStopped,
		Badge:          session.BadgeStopped,
		WakeEventID:    "wake-1",
	}
}
