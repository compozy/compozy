package presets

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	bridgepkg "github.com/compozy/compozy/internal/bridges"
	"github.com/compozy/compozy/internal/diagnostics"
	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/notifications"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
)

func TestNotificationPresetMatchingAndFilters(t *testing.T) {
	t.Parallel()

	t.Run("Should match canonical task run wildcard events", func(t *testing.T) {
		t.Parallel()

		if !MatchesAny([]string{"task.run_*"}, eventspkg.TaskRunCompleted) {
			t.Fatal("MatchesAny(task.run_*) = false, want task.run_completed match")
		}
		if !MatchesAny([]string{"task.run_*"}, eventspkg.TaskRunOperatorRetry) {
			t.Fatal("MatchesAny(task.run_*) = false, want task.run_operator_retry match")
		}
		retry, ok := eventspkg.Lookup(eventspkg.TaskRunOperatorRetry)
		if !ok || !retry.NotificationEligible {
			t.Fatalf("TaskRunOperatorRetry metadata = %#v, want notification eligible", retry)
		}
		if MatchesAny([]string{"task.run_*"}, eventspkg.SessionUnhealthy) {
			t.Fatal("MatchesAny(task.run_*) matched session.unhealthy, want family isolation")
		}
	})

	t.Run("Should evaluate supported LL1 filter expressions", func(t *testing.T) {
		t.Parallel()

		filter, err := CompileFilter(
			`severity >= warning AND workspace = "ws-alpha" OR provider = codex`,
		)
		if err != nil {
			t.Fatalf("CompileFilter() error = %v", err)
		}
		if !filter.Eval(Event{
			Type:    eventspkg.ProviderPermissionDenied,
			Outcome: eventspkg.OutcomeFailure,
			Scope: notifications.ScopeRef{
				Kind:        notifications.ScopeKindWorkspace,
				WorkspaceID: "ws-alpha",
			},
			Provider: "claude",
		}) {
			t.Fatal("filter.Eval(failure ws-alpha) = false, want match")
		}
		if !filter.Eval(Event{
			Type:    eventspkg.ProviderRateLimited,
			Outcome: eventspkg.OutcomeInfo,
			Scope: notifications.ScopeRef{
				Kind:        notifications.ScopeKindWorkspace,
				WorkspaceID: "ws-beta",
			},
			Provider: "codex",
		}) {
			t.Fatal("filter.Eval(provider codex) = false, want OR match")
		}
		if filter.Eval(Event{
			Type:    eventspkg.TaskRunCompleted,
			Outcome: eventspkg.OutcomeSuccess,
			Scope: notifications.ScopeRef{
				Kind:        notifications.ScopeKindWorkspace,
				WorkspaceID: "ws-beta",
			},
			Provider: "claude",
		}) {
			t.Fatal("filter.Eval(success ws-beta) = true, want no match")
		}
	})

	t.Run("Should reject unsupported filter fields", func(t *testing.T) {
		t.Parallel()

		if _, err := CompileFilter("severity >= warning AND tenant = acme"); !errors.Is(
			err,
			ErrInvalidPreset,
		) {
			t.Fatalf("CompileFilter(unsupported field) error = %v, want ErrInvalidPreset", err)
		}
	})
}

func TestNotificationPresetBuiltIns(t *testing.T) {
	t.Parallel()

	t.Run("Should seed disabled presets with stable default metadata", func(t *testing.T) {
		t.Parallel()

		builtIns := BuiltInPresets(time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC))
		if got, want := len(builtIns), 3; got != want {
			t.Fatalf("len(BuiltInPresets) = %d, want %d", got, want)
		}
		for _, preset := range builtIns {
			if preset.Enabled {
				t.Fatalf("preset %q enabled = true, want disabled", preset.Name)
			}
			if !preset.BuiltIn || preset.DefaultVersion == "" || preset.DefaultHash == "" {
				t.Fatalf(
					"preset %q metadata = %#v, want built-in default metadata",
					preset.Name,
					preset,
				)
			}
			for _, pattern := range preset.Events {
				if strings.Contains(pattern, "*") {
					continue
				}
				meta, ok := eventspkg.Lookup(pattern)
				if !ok || !meta.NotificationEligible {
					t.Fatalf(
						"preset %q event %q registry meta = %#v, want notification eligible",
						preset.Name,
						pattern,
						meta,
					)
				}
			}
		}
	})
}

func TestNotificationPresetTargetValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should reject noncanonical delivery modes instead of merging identities", func(t *testing.T) {
		t.Parallel()

		err := (Target{
			BridgeID:       "brg-1",
			CanonicalRoute: "#ops",
			DeliveryMode:   " direct-send ",
		}).Validate()
		if !errors.Is(err, ErrInvalidPreset) {
			t.Fatalf("Target.Validate() error = %v, want ErrInvalidPreset", err)
		}
	})
}

func TestNotificationPresetDispatch(t *testing.T) {
	t.Parallel()

	t.Run("Should advance cursor for suppressed bridge without delivery", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		store := newPresetMemoryStore([]Preset{{
			Name:    "task_terminal",
			Events:  []string{"task.run_*"},
			Targets: []Target{{BridgeID: "brg-1", CanonicalRoute: "#ops"}},
			Enabled: true,
		}})
		cursors := newPresetMemoryCursorStore()
		bridges := &presetFakeBridgeRuntime{
			instance: bridgepkg.BridgeInstance{
				ID:                   "brg-1",
				Scope:                bridgepkg.ScopeGlobal,
				Platform:             "slack",
				ExtensionName:        "slack-extension",
				DisplayName:          "Slack",
				Enabled:              true,
				Status:               bridgepkg.BridgeStatusReady,
				NotificationSuppress: true,
			},
		}
		service := NewService(Config{
			Store:   store,
			Cursors: cursors,
			Bridges: bridges,
			Now:     presetTestNow,
		})

		result, err := service.Dispatch(ctx, Event{
			ID:       "evt-1",
			Type:     eventspkg.TaskRunCompleted,
			Scope:    notifications.ScopeRef{Kind: notifications.ScopeKindWorkspace, WorkspaceID: "ws-alpha"},
			Sequence: 7,
			Summary:  "Build finished",
		})
		if err != nil {
			t.Fatalf("Dispatch() error = %v", err)
		}
		if result.Matched != 1 || result.Suppressed != 1 || result.Delivered != 0 ||
			result.Failed != 0 {
			t.Fatalf("Dispatch() result = %#v, want one suppressed delivery", result)
		}
		if bridges.deliveries != 0 {
			t.Fatalf("bridge deliveries = %d, want 0 for suppressed bridge", bridges.deliveries)
		}
		stored, err := cursors.ListCursors(ctx, notifications.CursorQuery{})
		if err != nil {
			t.Fatalf("ListCursors() error = %v", err)
		}
		if len(stored) != 1 {
			t.Fatalf("len(cursors) = %d, want 1", len(stored))
		}
		if stored[0].LastSequence != 7 || stored[0].LastDeliveryID == "" ||
			strings.Contains(stored[0].LastDeliveryID, ":") {
			t.Fatalf("cursor = %#v, want suppressed skip at sequence 7", stored[0])
		}
		if stored[0].Key.StreamName != string(eventspkg.TaskRunCompleted) ||
			stored[0].Key.SubjectID != "evt-1" {
			t.Fatalf("cursor key = %#v, want exact event type and id", stored[0].Key)
		}

		replay, err := service.Dispatch(ctx, Event{
			ID:       "evt-1",
			Type:     eventspkg.TaskRunCompleted,
			Scope:    notifications.ScopeRef{Kind: notifications.ScopeKindWorkspace, WorkspaceID: "ws-alpha"},
			Sequence: 7,
			Summary:  "Build finished",
		})
		if err != nil {
			t.Fatalf("Dispatch(replay) error = %v", err)
		}
		if replay.Skipped != 1 || replay.Suppressed != 0 || replay.Delivered != 0 {
			t.Fatalf("Dispatch(replay) result = %#v, want cursor skip", replay)
		}
	})

	t.Run("Should isolate global and workspace-global cursors with colon-bearing identities", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		store := newPresetMemoryStore([]Preset{{
			Name:    "task_terminal",
			Events:  []string{"task.run_*"},
			Targets: []Target{{BridgeID: "brg-1", CanonicalRoute: "#ops:incident"}},
			Enabled: true,
		}})
		cursors := newPresetMemoryCursorStore()
		bridges := &presetFakeBridgeRuntime{instance: bridgepkg.BridgeInstance{
			ID:            "brg-1",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "slack-extension",
			DisplayName:   "Slack",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
		}}
		service := NewService(Config{
			Store:   store,
			Cursors: cursors,
			Bridges: bridges,
			Now:     presetTestNow,
		})

		globalResult, err := service.Dispatch(ctx, Event{
			ID:       "evt:terminal",
			Type:     eventspkg.TaskRunCompleted,
			Scope:    notifications.ScopeRef{Kind: notifications.ScopeKindGlobal},
			Sequence: 7,
			Summary:  "Global task finished",
		})
		if err != nil {
			t.Fatalf("Dispatch(global) error = %v", err)
		}
		workspaceResult, err := service.Dispatch(ctx, Event{
			ID:       "evt:terminal",
			Type:     eventspkg.TaskRunCompleted,
			Scope:    notifications.ScopeRef{Kind: notifications.ScopeKindWorkspace, WorkspaceID: "global"},
			Sequence: 7,
			Summary:  "Workspace task finished",
		})
		if err != nil {
			t.Fatalf("Dispatch(workspace global) error = %v", err)
		}
		if globalResult.Delivered != 1 || workspaceResult.Delivered != 1 {
			t.Fatalf(
				"dispatch results = %#v, %#v, want one delivery from each distinct scope",
				globalResult,
				workspaceResult,
			)
		}

		stored, err := cursors.ListCursors(ctx, notifications.CursorQuery{})
		if err != nil {
			t.Fatalf("ListCursors() error = %v", err)
		}
		if len(stored) != 2 {
			t.Fatalf("len(cursors) = %d, want 2 independent cursor rows", len(stored))
		}
		seenScopes := make(map[notifications.ScopeRef]bool, len(stored))
		seenDeliveryIDs := make(map[string]bool, len(stored))
		for _, cursor := range stored {
			seenScopes[cursor.Key.Scope] = true
			if cursor.Key.SubjectID != "evt:terminal" {
				t.Fatalf("cursor subject = %q, want event ID without scope concatenation", cursor.Key.SubjectID)
			}
			if cursor.LastDeliveryID == "" || strings.Contains(cursor.LastDeliveryID, ":") {
				t.Fatalf("cursor delivery ID = %q, want opaque base64url identity", cursor.LastDeliveryID)
			}
			if seenDeliveryIDs[cursor.LastDeliveryID] {
				t.Fatalf("duplicate delivery ID %q across distinct scope identities", cursor.LastDeliveryID)
			}
			seenDeliveryIDs[cursor.LastDeliveryID] = true
		}
		for _, scope := range []notifications.ScopeRef{
			{Kind: notifications.ScopeKindGlobal},
			{Kind: notifications.ScopeKindWorkspace, WorkspaceID: "global"},
		} {
			if !seenScopes[scope] {
				t.Fatalf("cursor scopes = %#v, missing %#v", seenScopes, scope)
			}
		}
		if got := bridges.deliveries; got != 2 {
			t.Fatalf("bridge deliveries = %d, want 2 independent deliveries", got)
		}
	})

	t.Run("Should retain distinct opaque target identities that differ by whitespace", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		store := newPresetMemoryStore([]Preset{{
			Name:   "task_terminal",
			Events: []string{"task.run_*"},
			Targets: []Target{
				{BridgeID: "brg-1", CanonicalRoute: "#ops"},
				{BridgeID: "brg-1", CanonicalRoute: " #ops"},
			},
			Enabled: true,
		}})
		cursors := newPresetMemoryCursorStore()
		bridges := &presetFakeBridgeRuntime{instance: bridgepkg.BridgeInstance{
			ID:            "brg-1",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "slack-extension",
			DisplayName:   "Slack",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
		}}
		service := NewService(Config{
			Store:   store,
			Cursors: cursors,
			Bridges: bridges,
			Now:     presetTestNow,
		})

		result, err := service.Dispatch(ctx, Event{
			ID:       "evt-opaque-target",
			Type:     eventspkg.TaskRunCompleted,
			Scope:    notifications.ScopeRef{Kind: notifications.ScopeKindWorkspace, WorkspaceID: "ws-alpha"},
			Sequence: 7,
		})
		if err != nil {
			t.Fatalf("Dispatch() error = %v", err)
		}
		if got, want := result.Delivered, 2; got != want {
			t.Fatalf("Dispatch() delivered = %d, want %d distinct opaque targets", got, want)
		}
		if got, want := bridges.deliveries, 2; got != want {
			t.Fatalf("bridge deliveries = %d, want %d", got, want)
		}
		stored, err := cursors.ListCursors(ctx, notifications.CursorQuery{})
		if err != nil {
			t.Fatalf("ListCursors() error = %v", err)
		}
		if got, want := len(stored), 2; got != want {
			t.Fatalf("stored cursor rows = %d, want %d", got, want)
		}
		if stored[0].Key.ConsumerID == stored[1].Key.ConsumerID {
			t.Fatalf(
				"consumer identities = %q and %q, want distinct opaque identities",
				stored[0].Key.ConsumerID,
				stored[1].Key.ConsumerID,
			)
		}
	})

	t.Run("Should deliver every repeated target occurrence independently", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		const route = "#ops"
		store := newPresetMemoryStore([]Preset{{
			Name:    "task_terminal",
			Events:  []string{"task.run_*"},
			Targets: []Target{{BridgeID: "brg-1", CanonicalRoute: route}, {BridgeID: "brg-1", CanonicalRoute: route}},
			Enabled: true,
		}})
		cursors := newPresetMemoryCursorStore()
		bridges := &presetFakeBridgeRuntime{instance: bridgepkg.BridgeInstance{
			ID:            "brg-1",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: "slack-extension",
			DisplayName:   "Slack",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
		}}
		service := NewService(Config{
			Store:   store,
			Cursors: cursors,
			Bridges: bridges,
			Now:     presetTestNow,
		})

		result, err := service.Dispatch(ctx, Event{
			ID:       "evt-repeated-target",
			Type:     eventspkg.TaskRunCompleted,
			Scope:    notifications.ScopeRef{Kind: notifications.ScopeKindWorkspace, WorkspaceID: "ws-alpha"},
			Sequence: 7,
		})
		if err != nil {
			t.Fatalf("Dispatch() error = %v", err)
		}
		if got, want := result.Delivered, 2; got != want {
			t.Fatalf("Dispatch() delivered = %d, want %d", got, want)
		}
		if got, want := bridges.deliveries, 2; got != want {
			t.Fatalf("bridge deliveries = %d, want %d", got, want)
		}
		if got, want := len(bridges.requests), 2; got != want {
			t.Fatalf("bridge delivery requests = %d, want %d", got, want)
		}
		if bridges.requests[0].Event.DeliveryID == bridges.requests[1].Event.DeliveryID {
			t.Fatalf(
				"delivery IDs = %q and %q, want independent occurrences",
				bridges.requests[0].Event.DeliveryID,
				bridges.requests[1].Event.DeliveryID,
			)
		}

		stored, err := cursors.ListCursors(ctx, notifications.CursorQuery{})
		if err != nil {
			t.Fatalf("ListCursors() error = %v", err)
		}
		if got, want := len(stored), 2; got != want {
			t.Fatalf("stored cursor rows = %d, want %d", got, want)
		}
		if stored[0].Key.ConsumerID == stored[1].Key.ConsumerID {
			t.Fatalf(
				"cursor consumers = %q and %q, want independent occurrences",
				stored[0].Key.ConsumerID,
				stored[1].Key.ConsumerID,
			)
		}
		if stored[0].LastDeliveryID == stored[1].LastDeliveryID {
			t.Fatalf(
				"cursor delivery IDs = %q and %q, want independent occurrences",
				stored[0].LastDeliveryID,
				stored[1].LastDeliveryID,
			)
		}
	})
}

func TestNotificationPresetDispatchRedactsCursorDiagnostics(t *testing.T) {
	t.Parallel()

	t.Run("Should reuse one safe diagnostic for cursor, event, and returned error", func(t *testing.T) {
		t.Parallel()

		const dynamicSecret = "runtime-preset-dispatch-secret"

		release := diagnostics.RegisterDynamicSecret(dynamicSecret)
		t.Cleanup(release)

		ctx := testutil.Context(t)
		cursors := newPresetMemoryCursorStore()
		events := &presetMemoryEventSummaryStore{}
		bridges := &presetFakeBridgeRuntime{
			instance: bridgepkg.BridgeInstance{
				ID:            "brg-1",
				Scope:         bridgepkg.ScopeGlobal,
				Platform:      "slack",
				ExtensionName: "slack-extension",
				DisplayName:   "Slack",
				Enabled:       true,
				Status:        bridgepkg.BridgeStatusReady,
			},
			deliverErr: errors.New(strings.Join([]string{
				"dynamic=" + dynamicSecret,
				"authorization=Bearer bearer-preset-secret",
				"claim=compozy_claim_preset-secret",
				"oauth_code=oauth-preset-secret",
				"code_verifier=pkce-preset-secret",
			}, " ")),
		}
		service := NewService(Config{
			Store: newPresetMemoryStore([]Preset{{
				Name:    "task_terminal",
				Events:  []string{"task.run_*"},
				Targets: []Target{{BridgeID: "brg-1", CanonicalRoute: "#ops"}},
				Enabled: true,
			}}),
			Cursors: cursors,
			Bridges: bridges,
			Events:  events,
			Now:     presetTestNow,
		})

		result, err := service.Dispatch(ctx, Event{
			ID:       "evt-redacted-diagnostic",
			Type:     eventspkg.TaskRunCompleted,
			Scope:    notifications.ScopeRef{Kind: notifications.ScopeKindWorkspace, WorkspaceID: "ws-alpha"},
			Sequence: 7,
		})
		if err == nil {
			t.Fatal("Dispatch() error = nil, want delivery failure")
		}
		if got, want := result.Failed, 1; got != want {
			t.Fatalf("Dispatch() failed = %d, want %d", got, want)
		}

		for _, secret := range []string{
			dynamicSecret,
			"bearer-preset-secret",
			"compozy_claim_preset-secret",
			"oauth-preset-secret",
			"pkce-preset-secret",
		} {
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("Dispatch() error = %q, leaked %q", err, secret)
			}
		}

		stored, listErr := cursors.ListCursors(ctx, notifications.CursorQuery{})
		if listErr != nil {
			t.Fatalf("ListCursors() error = %v", listErr)
		}
		if got, want := len(stored), 1; got != want {
			t.Fatalf("stored cursor rows = %d, want %d", got, want)
		}
		if !utf8.ValidString(stored[0].LastError) {
			t.Fatalf("stored cursor last_error = %q, want valid UTF-8", stored[0].LastError)
		}
		if got, limit := len(stored[0].LastError), 2048; got > limit {
			t.Fatalf("stored cursor last_error bytes = %d, want <= %d", got, limit)
		}
		if got, want := len(events.items), 1; got != want {
			t.Fatalf("dispatch failure event count = %d, want %d", got, want)
		}
		for _, secret := range []string{
			dynamicSecret,
			"bearer-preset-secret",
			"compozy_claim_preset-secret",
			"oauth-preset-secret",
			"pkce-preset-secret",
		} {
			if strings.Contains(string(events.items[0].Content), secret) {
				t.Fatalf("dispatch failure event = %s, leaked %q", events.items[0].Content, secret)
			}
		}
		if got, want := err.Error(), stored[0].LastError; got != want {
			t.Fatalf("Dispatch() error = %q, want stored cursor diagnostic %q", got, want)
		}
		var payload struct {
			LastError string `json:"last_error"`
		}
		if err := json.Unmarshal(events.items[0].Content, &payload); err != nil {
			t.Fatalf("decode dispatch failure event = %v", err)
		}
		if got, want := payload.LastError, stored[0].LastError; got != want {
			t.Fatalf("dispatch failure event last_error = %q, want cursor diagnostic %q", got, want)
		}
	})
}

func TestNotificationPresetTaskEventScope(t *testing.T) {
	t.Parallel()

	t.Run("Should deliver with global scope when task ownership cannot be read", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		cursors := newPresetMemoryCursorStore()
		bridges := newPresetReadyBridgeRuntime()
		service := newPresetTaskTerminalService(cursors, bridges)
		observer := NewTaskEventObserver(
			service,
			presetFakeTaskReader{err: taskpkg.ErrTaskNotFound},
			slog.New(slog.DiscardHandler),
		)

		observer.OnTaskEvent(ctx, presetTaskEventRecord("evt-orphan", "task-deleted"))

		if got, want := bridges.deliveries, 1; got != want {
			t.Fatalf("bridge deliveries = %d, want %d for an unreadable task", got, want)
		}
		stored, err := cursors.ListCursors(ctx, notifications.CursorQuery{})
		if err != nil {
			t.Fatalf("ListCursors() error = %v", err)
		}
		if got, want := len(stored), 1; got != want {
			t.Fatalf("stored cursor rows = %d, want %d", got, want)
		}
		wantScope := notifications.ScopeRef{Kind: notifications.ScopeKindGlobal}
		if got := stored[0].Key.Scope; got != wantScope {
			t.Fatalf("cursor scope = %#v, want %#v", got, wantScope)
		}
	})

	t.Run("Should carry workspace ownership verbatim from the task record", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		cursors := newPresetMemoryCursorStore()
		bridges := newPresetReadyBridgeRuntime()
		service := newPresetTaskTerminalService(cursors, bridges)
		observer := NewTaskEventObserver(
			service,
			presetFakeTaskReader{task: taskpkg.Task{
				ID:          "task-owned",
				Scope:       taskpkg.ScopeWorkspace,
				WorkspaceID: " global ",
				Title:       " Build finished ",
			}},
			slog.New(slog.DiscardHandler),
		)

		observer.OnTaskEvent(ctx, presetTaskEventRecord("evt-owned", "task-owned"))

		stored, err := cursors.ListCursors(ctx, notifications.CursorQuery{})
		if err != nil {
			t.Fatalf("ListCursors() error = %v", err)
		}
		if got, want := len(stored), 1; got != want {
			t.Fatalf("stored cursor rows = %d, want %d", got, want)
		}
		wantScope := notifications.ScopeRef{
			Kind:        notifications.ScopeKindWorkspace,
			WorkspaceID: " global ",
		}
		if got := stored[0].Key.Scope; got != wantScope {
			t.Fatalf("cursor scope = %#v, want %#v", got, wantScope)
		}
		if got, want := len(bridges.requests), 1; got != want {
			t.Fatalf("bridge delivery requests = %d, want %d", got, want)
		}
		if got, want := bridges.requests[0].Event.Content.Text, "Compozy task_terminal: Build finished"; got != want {
			t.Fatalf("delivery text = %q, want %q", got, want)
		}
	})

	t.Run("Should map a global task to global scope without a workspace binding", func(t *testing.T) {
		t.Parallel()

		got := taskEventScope(taskpkg.Task{ID: "task-global", Scope: taskpkg.ScopeGlobal})
		want := notifications.ScopeRef{Kind: notifications.ScopeKindGlobal}
		if got != want {
			t.Fatalf("taskEventScope(global) = %#v, want %#v", got, want)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("taskEventScope(global).Validate() error = %v", err)
		}
	})
}

type presetFakeTaskReader struct {
	task taskpkg.Task
	err  error
}

func (r presetFakeTaskReader) GetTask(_ context.Context, _ string) (taskpkg.Task, error) {
	if r.err != nil {
		return taskpkg.Task{}, r.err
	}
	return r.task, nil
}

func newPresetReadyBridgeRuntime() *presetFakeBridgeRuntime {
	return &presetFakeBridgeRuntime{instance: bridgepkg.BridgeInstance{
		ID:            "brg-1",
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "slack",
		ExtensionName: "slack-extension",
		DisplayName:   "Slack",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
	}}
}

func newPresetTaskTerminalService(
	cursors notifications.CursorStore,
	bridges BridgeRuntime,
) *Service {
	return NewService(Config{
		Store: newPresetMemoryStore([]Preset{{
			Name:    "task_terminal",
			Events:  []string{"task.run_*"},
			Targets: []Target{{BridgeID: "brg-1", CanonicalRoute: "#ops"}},
			Enabled: true,
		}}),
		Cursors: cursors,
		Bridges: bridges,
		Now:     presetTestNow,
	})
}

func presetTaskEventRecord(eventID string, taskID string) taskpkg.EventRecord {
	return taskpkg.EventRecord{
		Sequence: 9,
		Event: taskpkg.Event{
			ID:        eventID,
			TaskID:    taskID,
			RunID:     "run-1",
			EventType: eventspkg.TaskRunCompleted,
			Actor:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAgentSession, Ref: "sess-1"},
			Timestamp: presetTestNow(),
		},
	}
}

func presetTestNow() time.Time {
	return time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC)
}

type presetMemoryStore struct {
	mu      sync.Mutex
	presets map[string]Preset
}

func newPresetMemoryStore(items []Preset) *presetMemoryStore {
	store := &presetMemoryStore{presets: make(map[string]Preset, len(items))}
	for _, item := range items {
		preset := item.Normalize()
		if preset.CreatedAt.IsZero() {
			preset.CreatedAt = presetTestNow()
		}
		if preset.UpdatedAt.IsZero() {
			preset.UpdatedAt = preset.CreatedAt
		}
		store.presets[preset.Name] = preset
	}
	return store
}

func (s *presetMemoryStore) ListPresets(_ context.Context, query Query) ([]Preset, error) {
	q := query.Normalize()
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]Preset, 0, len(s.presets))
	for _, preset := range s.presets {
		if q.Enabled != nil && preset.Enabled != *q.Enabled {
			continue
		}
		if q.BuiltIn != nil && preset.BuiltIn != *q.BuiltIn {
			continue
		}
		if q.Name != "" && preset.Name != q.Name {
			continue
		}
		items = append(items, preset)
	}
	return items, nil
}

func (s *presetMemoryStore) GetPreset(_ context.Context, name string) (Preset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	preset, ok := s.presets[strings.TrimSpace(name)]
	if !ok {
		return Preset{}, ErrPresetNotFound
	}
	return preset, nil
}

func (s *presetMemoryStore) CreatePreset(_ context.Context, preset Preset) (Preset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.presets[preset.Name]; exists {
		return Preset{}, ErrPresetDuplicateName
	}
	s.presets[preset.Name] = preset
	return preset, nil
}

func (s *presetMemoryStore) UpdatePreset(
	_ context.Context,
	name string,
	req UpdateRequest,
) (Preset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	preset, ok := s.presets[strings.TrimSpace(name)]
	if !ok {
		return Preset{}, ErrPresetNotFound
	}
	if req.Events != nil {
		preset.Events = *req.Events
	}
	if req.Targets != nil {
		preset.Targets = *req.Targets
	}
	if req.Filter != nil {
		preset.Filter = *req.Filter
	}
	if req.Enabled != nil {
		preset.Enabled = *req.Enabled
	}
	preset.UpdatedAt = req.Now
	s.presets[preset.Name] = preset
	return preset, nil
}

func (s *presetMemoryStore) DeletePreset(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.presets, strings.TrimSpace(name))
	return nil
}

func (s *presetMemoryStore) EnsureBuiltInPresets(_ context.Context, defaults []Preset) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, preset := range defaults {
		s.presets[preset.Name] = preset
	}
	return nil
}

type presetFakeBridgeRuntime struct {
	instance   bridgepkg.BridgeInstance
	deliverErr error
	deliveries int
	requests   []bridgepkg.DeliveryRequest
}

func (r *presetFakeBridgeRuntime) GetBridgeInstance(
	_ context.Context,
	id string,
) (bridgepkg.BridgeInstance, error) {
	if strings.TrimSpace(id) != r.instance.ID {
		return bridgepkg.BridgeInstance{}, bridgepkg.ErrBridgeInstanceNotFound
	}
	return r.instance, nil
}

func (r *presetFakeBridgeRuntime) ResolveBridgeTarget(
	_ context.Context,
	bridgeID string,
	canonicalRoute string,
) (bridgepkg.ResolveBridgeTargetResult, error) {
	return bridgepkg.ResolveBridgeTargetResult{
		Match: &bridgepkg.BridgeTarget{
			BridgeID:       strings.TrimSpace(bridgeID),
			CanonicalRoute: strings.TrimSpace(canonicalRoute),
			DisplayName:    "ops",
			TargetType:     bridgepkg.BridgeTargetTypeChannel,
		},
	}, nil
}

func (r *presetFakeBridgeRuntime) DeliverBridge(
	_ context.Context,
	_ string,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	r.deliveries++
	r.requests = append(r.requests, request)
	if r.deliverErr != nil {
		return bridgepkg.DeliveryAck{}, r.deliverErr
	}
	return bridgepkg.DeliveryAck{DeliveryID: request.Event.DeliveryID, Seq: request.Event.Seq}, nil
}

type presetMemoryEventSummaryStore struct {
	items []store.EventSummary
}

func (s *presetMemoryEventSummaryStore) WriteEventSummary(
	_ context.Context,
	summary store.EventSummary,
) error {
	s.items = append(s.items, summary)
	return nil
}

func (s *presetMemoryEventSummaryStore) ListEventSummaries(
	_ context.Context,
	_ store.EventSummaryQuery,
) ([]store.EventSummary, error) {
	return append([]store.EventSummary(nil), s.items...), nil
}

type presetMemoryCursorStore struct {
	mu      sync.Mutex
	cursors map[notifications.CursorKey]notifications.Cursor
}

func newPresetMemoryCursorStore() *presetMemoryCursorStore {
	return &presetMemoryCursorStore{cursors: make(map[notifications.CursorKey]notifications.Cursor)}
}

func (s *presetMemoryCursorStore) GetCursor(
	_ context.Context,
	key notifications.CursorKey,
) (notifications.Cursor, error) {
	normalized, err := key.Normalize()
	if err != nil {
		return notifications.Cursor{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cursor, ok := s.cursors[normalized]
	if !ok {
		return notifications.Cursor{}, notifications.ErrCursorNotFound
	}
	return cursor, nil
}

func (s *presetMemoryCursorStore) ListCursors(
	_ context.Context,
	query notifications.CursorQuery,
) ([]notifications.Cursor, error) {
	q, err := query.Normalize()
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cursors := make([]notifications.Cursor, 0, len(s.cursors))
	for _, cursor := range s.cursors {
		if q.Scope.Kind != "" && cursor.Key.Scope != q.Scope {
			continue
		}
		if q.ConsumerID != "" && cursor.Key.ConsumerID != q.ConsumerID {
			continue
		}
		if q.StreamName != "" && cursor.Key.StreamName != q.StreamName {
			continue
		}
		if q.SubjectID != "" && cursor.Key.SubjectID != q.SubjectID {
			continue
		}
		cursors = append(cursors, cursor)
	}
	return cursors, nil
}

func (s *presetMemoryCursorStore) AdvanceCursor(
	_ context.Context,
	update notifications.AdvanceCursor,
) (notifications.Cursor, error) {
	normalized, err := update.Normalize(presetTestNow())
	if err != nil {
		return notifications.Cursor{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := normalized.Key
	current, exists := s.cursors[key]
	if exists {
		if normalized.LastSequence < current.LastSequence ||
			(normalized.LastSequence == current.LastSequence && normalized.DeliveryID != current.LastDeliveryID) {
			return notifications.Cursor{}, notifications.ErrNonMonotonicCursor
		}
		if normalized.LastSequence == current.LastSequence {
			return current, nil
		}
	}
	cursor := notifications.Cursor{
		Key:             normalized.Key,
		LastSequence:    normalized.LastSequence,
		LastDeliveryID:  normalized.DeliveryID,
		LastDeliveredAt: normalized.LastDeliveredAt,
		UpdatedAt:       normalized.Now,
	}
	s.cursors[key] = cursor
	return cursor, nil
}

func (s *presetMemoryCursorStore) ResetCursor(
	_ context.Context,
	reset notifications.ResetCursor,
) (notifications.Cursor, error) {
	normalized, err := reset.Normalize(presetTestNow())
	if err != nil {
		return notifications.Cursor{}, err
	}
	cursor := notifications.Cursor{
		Key:             normalized.Key,
		LastSequence:    normalized.LastSequence,
		LastDeliveryID:  normalized.LastDeliveryID,
		LastDeliveredAt: normalized.LastDeliveredAt,
		LastError:       normalized.Reason,
		UpdatedAt:       normalized.Now,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cursors[normalized.Key] = cursor
	return cursor, nil
}

func (s *presetMemoryCursorStore) RecordCursorError(
	_ context.Context,
	report notifications.CursorError,
) (notifications.Cursor, error) {
	normalized, err := report.Normalize(presetTestNow())
	if err != nil {
		return notifications.Cursor{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cursor := s.cursors[normalized.Key]
	cursor.Key = normalized.Key
	cursor.LastError = normalized.LastError
	cursor.UpdatedAt = normalized.Now
	s.cursors[normalized.Key] = cursor
	return cursor, nil
}
