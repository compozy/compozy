package presets

import (
	"context"

	"errors"
	"fmt"
	"log/slog"

	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	eventspkg "github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/notifications"
	"github.com/compozy/agh/internal/store"
)

const defaultDispatchTimeout = 10 * time.Second

// Store persists notification presets.
type Store interface {
	ListPresets(ctx context.Context, query Query) ([]Preset, error)
	GetPreset(ctx context.Context, name string) (Preset, error)
	CreatePreset(ctx context.Context, preset Preset) (Preset, error)
	UpdatePreset(ctx context.Context, name string, req UpdateRequest) (Preset, error)
	DeletePreset(ctx context.Context, name string) error
	EnsureBuiltInPresets(ctx context.Context, defaults []Preset) error
}

// BridgeRuntime is the bridge surface needed for preset fanout.
type BridgeRuntime interface {
	GetBridgeInstance(ctx context.Context, id string) (bridgepkg.BridgeInstance, error)
	ResolveBridgeTarget(
		ctx context.Context,
		bridgeID string,
		query string,
	) (bridgepkg.ResolveBridgeTargetResult, error)
	DeliverBridge(
		ctx context.Context,
		extensionName string,
		req bridgepkg.DeliveryRequest,
	) (bridgepkg.DeliveryAck, error)
}

// Service owns preset CRUD and cursor-backed fanout.
type Service struct {
	store   Store
	cursors *notifications.Service
	bridges BridgeRuntime
	events  store.EventSummaryStore
	logger  *slog.Logger
	now     func() time.Time
	timeout time.Duration
}

// Config wires a preset service.
type Config struct {
	Store   Store
	Cursors notifications.CursorStore
	Bridges BridgeRuntime
	Events  store.EventSummaryStore
	Logger  *slog.Logger
	Now     func() time.Time
	Timeout time.Duration
}

func NewService(cfg Config) *Service {
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultDispatchTimeout
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		store:   cfg.Store,
		cursors: notifications.NewService(cfg.Cursors),
		bridges: cfg.Bridges,
		events:  cfg.Events,
		logger:  logger,
		now:     now,
		timeout: timeout,
	}
}

func (s *Service) List(ctx context.Context, query Query) ([]Preset, error) {
	if err := s.checkStore(); err != nil {
		return nil, err
	}
	items, err := s.store.ListPresets(ctx, query.Normalize())
	if err != nil {
		return nil, fmt.Errorf("notifications: list presets: %w", err)
	}
	return items, nil
}

func (s *Service) Get(ctx context.Context, name string) (Preset, error) {
	if err := s.checkStore(); err != nil {
		return Preset{}, err
	}
	preset, err := s.store.GetPreset(ctx, normalizePresetName(name))
	if err != nil {
		return Preset{}, fmt.Errorf(
			"notifications: get preset %q: %w",
			normalizePresetName(name),
			err,
		)
	}
	return preset, nil
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (Preset, error) {
	if err := s.checkStore(); err != nil {
		return Preset{}, err
	}
	preset, err := req.Normalize(s.now())
	if err != nil {
		return Preset{}, err
	}
	created, err := s.store.CreatePreset(ctx, preset)
	if err != nil {
		return Preset{}, fmt.Errorf("notifications: create preset %q: %w", preset.Name, err)
	}
	if err := s.recordPresetLifecycleEvent(ctx, eventspkg.NotificationPresetCreated, created); err != nil {
		return Preset{}, err
	}
	return created, nil
}

func (s *Service) Update(ctx context.Context, name string, req UpdateRequest) (Preset, error) {
	if err := s.checkStore(); err != nil {
		return Preset{}, err
	}
	if !req.HasMutableField() {
		return Preset{}, fmt.Errorf(
			"%w: update requires at least one mutable field",
			ErrInvalidPreset,
		)
	}
	req = normalizeUpdateRequest(req, s.now())
	updated, err := s.store.UpdatePreset(ctx, normalizePresetName(name), req)
	if err != nil {
		return Preset{}, fmt.Errorf(
			"notifications: update preset %q: %w",
			normalizePresetName(name),
			err,
		)
	}
	if err := s.recordPresetLifecycleEvent(ctx, eventspkg.NotificationPresetUpdated, updated); err != nil {
		return Preset{}, err
	}
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, name string) error {
	if err := s.checkStore(); err != nil {
		return err
	}
	normalizedName := normalizePresetName(name)
	if err := s.store.DeletePreset(ctx, normalizedName); err != nil {
		return fmt.Errorf("notifications: delete preset %q: %w", normalizedName, err)
	}
	if err := s.recordPresetLifecycleEvent(
		ctx,
		eventspkg.NotificationPresetDeleted,
		Preset{Name: normalizedName},
	); err != nil {
		return err
	}
	return nil
}

func (s *Service) EnsureBuiltIns(ctx context.Context) error {
	if err := s.checkStore(); err != nil {
		return err
	}
	if err := s.store.EnsureBuiltInPresets(ctx, BuiltInPresets(s.now())); err != nil {
		return fmt.Errorf("notifications: ensure built-in presets: %w", err)
	}
	return nil
}

func (s *Service) Dispatch(ctx context.Context, event Event) (DispatchResult, error) {
	if err := s.checkDispatch(); err != nil {
		return DispatchResult{}, err
	}
	normalizedEvent := event.Normalize(s.now())
	if err := normalizedEvent.Validate(); err != nil {
		return DispatchResult{}, err
	}
	meta, ok := eventspkg.Lookup(normalizedEvent.Type)
	if !ok || !meta.NotificationEligible {
		return DispatchResult{Skipped: 1}, nil
	}
	enabled := true
	presets, err := s.store.ListPresets(ctx, Query{Enabled: &enabled})
	if err != nil {
		return DispatchResult{}, fmt.Errorf(
			"notifications: list enabled presets for dispatch: %w",
			err,
		)
	}

	result := DispatchResult{}
	var joined error
	for _, preset := range presets {
		if !MatchesAny(preset.Events, normalizedEvent.Type) {
			continue
		}
		result.Matched++
		presetResult, dispatchErr := s.dispatchPreset(ctx, preset, normalizedEvent)
		result.Delivered += presetResult.Delivered
		result.Suppressed += presetResult.Suppressed
		result.Skipped += presetResult.Skipped
		result.Failed += presetResult.Failed
		if dispatchErr != nil {
			joined = errors.Join(joined, dispatchErr)
		}
	}
	return result, joined
}
