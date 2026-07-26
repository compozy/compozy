package presets

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	eventspkg "github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/notifications"
	"github.com/compozy/agh/internal/testutil"
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
			Type:        eventspkg.ProviderPermissionDenied,
			Outcome:     eventspkg.OutcomeFailure,
			WorkspaceID: "ws-alpha",
			Provider:    "claude",
		}) {
			t.Fatal("filter.Eval(failure ws-alpha) = false, want match")
		}
		if !filter.Eval(Event{
			Type:        eventspkg.ProviderRateLimited,
			Outcome:     eventspkg.OutcomeInfo,
			WorkspaceID: "ws-beta",
			Provider:    "codex",
		}) {
			t.Fatal("filter.Eval(provider codex) = false, want OR match")
		}
		if filter.Eval(Event{
			Type:        eventspkg.TaskRunCompleted,
			Outcome:     eventspkg.OutcomeSuccess,
			WorkspaceID: "ws-beta",
			Provider:    "claude",
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
			ID:          "evt-1",
			Type:        eventspkg.TaskRunCompleted,
			WorkspaceID: "ws-alpha",
			Sequence:    7,
			Summary:     "Build finished",
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
		if stored[0].LastSequence != 7 ||
			!strings.Contains(stored[0].LastDeliveryID, ":skip:suppressed") {
			t.Fatalf("cursor = %#v, want suppressed skip at sequence 7", stored[0])
		}

		replay, err := service.Dispatch(ctx, Event{
			ID:          "evt-1",
			Type:        eventspkg.TaskRunCompleted,
			WorkspaceID: "ws-alpha",
			Sequence:    7,
			Summary:     "Build finished",
		})
		if err != nil {
			t.Fatalf("Dispatch(replay) error = %v", err)
		}
		if replay.Skipped != 1 || replay.Suppressed != 0 || replay.Delivered != 0 {
			t.Fatalf("Dispatch(replay) result = %#v, want cursor skip", replay)
		}
	})
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
	deliveries int
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
	context.Context,
	string,
	string,
) (bridgepkg.ResolveBridgeTargetResult, error) {
	return bridgepkg.ResolveBridgeTargetResult{
		Match: &bridgepkg.BridgeTarget{
			BridgeID:       r.instance.ID,
			CanonicalRoute: "#ops",
			DisplayName:    "ops",
			TargetType:     bridgepkg.BridgeTargetTypeChannel,
		},
	}, nil
}

func (r *presetFakeBridgeRuntime) DeliverBridge(
	context.Context,
	string,
	bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	r.deliveries++
	return bridgepkg.DeliveryAck{}, errors.New("unexpected delivery")
}

type presetMemoryCursorStore struct {
	mu      sync.Mutex
	cursors map[string]notifications.Cursor
}

func newPresetMemoryCursorStore() *presetMemoryCursorStore {
	return &presetMemoryCursorStore{cursors: make(map[string]notifications.Cursor)}
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
	cursor, ok := s.cursors[presetCursorKey(normalized)]
	if !ok {
		return notifications.Cursor{}, notifications.ErrCursorNotFound
	}
	return cursor, nil
}

func (s *presetMemoryCursorStore) ListCursors(
	_ context.Context,
	query notifications.CursorQuery,
) ([]notifications.Cursor, error) {
	q := query.Normalize()
	s.mu.Lock()
	defer s.mu.Unlock()
	cursors := make([]notifications.Cursor, 0, len(s.cursors))
	for _, cursor := range s.cursors {
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
	key := presetCursorKey(normalized.Key)
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
	s.cursors[presetCursorKey(normalized.Key)] = cursor
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
	cursor := s.cursors[presetCursorKey(normalized.Key)]
	cursor.Key = normalized.Key
	cursor.LastError = normalized.LastError
	cursor.UpdatedAt = normalized.Now
	s.cursors[presetCursorKey(normalized.Key)] = cursor
	return cursor, nil
}

func presetCursorKey(key notifications.CursorKey) string {
	return key.ConsumerID + "\x00" + key.StreamName + "\x00" + key.SubjectID
}
