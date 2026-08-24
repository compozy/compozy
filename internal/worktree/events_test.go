package worktree

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnostics"
)

// Canonical suite: worktree mutation gates and fail-open observation hooks.
func TestServiceLifecycleHooks(t *testing.T) {
	t.Parallel()

	t.Run("Should stop creation only for an explicit pre-create denial", func(t *testing.T) {
		t.Parallel()
		fixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		WithHooks(removalHookDispatcher{dispatch: func(request HookRequest) (HookVerdict, error) {
			if request.Event == EventPreCreate {
				payload, ok := request.Payload.(HookWorktree)
				if !ok || payload.WorkspaceRoot != fixture.workspace.Root {
					t.Fatalf("pre-create payload = %#v, want parent workspace root", request.Payload)
				}
				return HookVerdict{Denied: true, HookName: "protect-main", Reason: "policy denied"}, nil
			}
			return HookVerdict{}, nil
		}})(fixture.service)
		_, err := fixture.service.Create(context.Background(), fixture.workspace.ID, CreateOptions{
			ProfileID: testWorktreeProfileID, Name: "denied",
		})
		if !errors.Is(err, ErrDeniedByHook) || !strings.Contains(err.Error(), "protect-main") {
			t.Fatalf("Create(denied) error = %v, want named hook denial", err)
		}
		if rows, listErr := fixture.store.List(
			context.Background(),
			fixture.workspace.ID,
		); listErr != nil ||
			len(rows) != 0 {
			t.Fatalf("rows after denial = %#v, %v, want empty", rows, listErr)
		}
		for _, command := range invocationCommands(fixture.runner.invocations()) {
			if strings.HasPrefix(command, "branch denied ") || strings.HasPrefix(command, "worktree add ") {
				t.Fatalf("post-gate mutation ran after denial: %q", command)
			}
		}
	})

	t.Run("Should fail open on hook execution errors and observe without an event sink", func(t *testing.T) {
		t.Parallel()
		fixture := newCreateTestFixture(t, config.DefaultWorktreesConfig())
		hooks := &recordingLifecycleHooks{failPreCreate: true}
		WithHooks(hooks)(fixture.service)
		fixture.service.events = nil
		item, err := fixture.service.Create(
			context.Background(),
			fixture.workspace.ID,
			CreateOptions{ProfileID: testWorktreeProfileID, Name: "hook-fail-open"},
		)
		if err != nil || item.State != StateReady {
			t.Fatalf("Create(fail-open) = %#v, %v, want ready", item, err)
		}
		if hooks.count(EventPreCreate) != 1 || hooks.count(EventCreated) != 1 {
			t.Fatalf("hook calls = %#v, want pre-create and created exactly once", hooks.events())
		}
	})

	t.Run("Should restore ready when pre-remove explicitly denies with risk payload", func(t *testing.T) {
		t.Parallel()
		fixture := newRemovalTestFixture(t)
		WithHooks(removalHookDispatcher{dispatch: func(request HookRequest) (HookVerdict, error) {
			if request.Event != EventPreRemove {
				return HookVerdict{}, nil
			}
			payload, ok := request.Payload.(HookPreRemovePayload)
			if !ok || payload.Worktree.WorkspaceRoot != fixture.workspace.Root || payload.Force {
				t.Fatalf("pre-remove payload = %#v, want parent root and force=false", request.Payload)
			}
			return HookVerdict{Denied: true, HookName: "keep-worktree", Reason: "still needed"}, nil
		}})(fixture.service)
		if _, err := fixture.service.Remove(
			context.Background(), fixture.workspace.ID, fixture.item.ID, false,
		); !errors.Is(err, ErrDeniedByHook) {
			t.Fatalf("Remove(denied) error = %v, want ErrDeniedByHook", err)
		}
		stored, err := fixture.store.Get(context.Background(), fixture.workspace.ID, fixture.item.ID)
		if err != nil || stored.State != StateReady {
			t.Fatalf("stored removal denial = %#v, %v, want ready", stored, err)
		}
	})

	t.Run("Should redact registered secrets from hook and canonical event payloads", func(t *testing.T) {
		t.Parallel()
		const secret = "event-bound-secret-123456"
		cleanup := diagnostics.RegisterRequiredSecret(secret)
		t.Cleanup(cleanup)
		events := &recordingEventSink{}
		hooks := &recordingPayloadHooks{}
		service := NewService(
			newMemoryWorktreeStore(), &recordingGitRunner{}, WithEvents(events), WithHooks(hooks),
			WithWorkspaceResolver(staticWorkspaceResolver{workspaces: map[string]Workspace{
				"ws-event-redaction": {ID: "ws-event-redaction", Root: "/workspace/" + secret},
			}}),
		)
		item := Worktree{
			ID: "wt-event-redaction", WorkspaceID: "ws-event-redaction",
			Name: "token=" + secret, Branch: "feature/" + secret,
			Path: "/tmp/" + secret, Origin: OriginManual,
		}
		service.emit(context.Background(), EventCreated, item)
		events.mu.Lock()
		if len(events.events) != 1 {
			events.mu.Unlock()
			t.Fatalf("canonical events = %d, want one", len(events.events))
		}
		frame := string(events.events[0].Payload)
		events.mu.Unlock()
		if strings.Contains(frame, secret) || !strings.Contains(frame, "[REDACTED]") {
			t.Fatalf("canonical event payload = %s, want redacted", frame)
		}
		payload := hooks.payload(EventCreated)
		encoded := payload.Name + payload.Branch + payload.Path + payload.WorkspaceRoot
		if strings.Contains(encoded, secret) || !strings.Contains(encoded, "[REDACTED]") {
			t.Fatalf("hook event payload = %#v, want redacted", payload)
		}
	})
}

type recordingLifecycleHooks struct {
	mu            sync.Mutex
	calls         []string
	failPreCreate bool
}

func (h *recordingLifecycleHooks) DispatchWorktreeHook(
	_ context.Context,
	request HookRequest,
) (HookVerdict, error) {
	h.mu.Lock()
	h.calls = append(h.calls, request.Event)
	h.mu.Unlock()
	if request.Event == EventPreCreate && h.failPreCreate {
		return HookVerdict{}, errors.New("hook process failed")
	}
	return HookVerdict{}, nil
}

func (h *recordingLifecycleHooks) count(event string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	count := 0
	for _, current := range h.calls {
		if current == event {
			count++
		}
	}
	return count
}

func (h *recordingLifecycleHooks) events() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.calls...)
}

type recordingPayloadHooks struct {
	mu       sync.Mutex
	payloads map[string]HookWorktree
}

func (h *recordingPayloadHooks) DispatchWorktreeHook(
	_ context.Context,
	request HookRequest,
) (HookVerdict, error) {
	payload, ok := request.Payload.(HookWorktree)
	if !ok {
		return HookVerdict{}, nil
	}
	h.mu.Lock()
	if h.payloads == nil {
		h.payloads = make(map[string]HookWorktree)
	}
	h.payloads[request.Event] = payload
	h.mu.Unlock()
	return HookVerdict{}, nil
}

func (h *recordingPayloadHooks) payload(event string) HookWorktree {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.payloads[event]
}
