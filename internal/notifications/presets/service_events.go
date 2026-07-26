package presets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	eventspkg "github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/notifications"
	"github.com/compozy/agh/internal/store"
)

func (s *Service) recordDispatchError(
	ctx context.Context,
	key notifications.CursorKey,
	preset Preset,
	event Event,
	err error,
) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	if len(message) > 2048 {
		message = message[:2048]
	}
	if _, recordErr := s.cursors.RecordError(ctx, notifications.CursorError{
		Key:       key,
		LastError: message,
		Now:       s.now(),
	}); recordErr != nil {
		return errors.Join(err, recordErr)
	}
	if eventErr := s.recordDispatchFailureEvent(ctx, key, preset, event, message); eventErr != nil {
		return errors.Join(err, eventErr)
	}
	return err
}

func (s *Service) recordPresetLifecycleEvent(
	ctx context.Context,
	eventType string,
	preset Preset,
) error {
	if s == nil || s.events == nil {
		return nil
	}
	payload := struct {
		Name                   string   `json:"name"`
		Events                 []string `json:"events,omitempty"`
		Enabled                bool     `json:"enabled"`
		BuiltIn                bool     `json:"built_in"`
		UserModified           bool     `json:"user_modified"`
		DefaultUpdateAvailable bool     `json:"default_update_available"`
	}{
		Name:                   preset.Name,
		Events:                 append([]string(nil), preset.Events...),
		Enabled:                preset.Enabled,
		BuiltIn:                preset.BuiltIn,
		UserModified:           preset.UserModified,
		DefaultUpdateAvailable: preset.DefaultUpdateAvailable,
	}
	content, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("notifications: encode preset lifecycle event: %w", err)
	}
	if err := s.events.WriteEventSummary(detachedContext(ctx), store.EventSummary{
		Type:      eventType,
		Outcome:   string(eventspkg.OutcomeFor(eventType)),
		Content:   content,
		Summary:   notificationPresetLifecycleSummary(eventType, preset.Name),
		Timestamp: s.now().UTC(),
	}); err != nil {
		return fmt.Errorf("notifications: record preset lifecycle event: %w", err)
	}
	return nil
}

func (s *Service) recordDispatchFailureEvent(
	ctx context.Context,
	key notifications.CursorKey,
	preset Preset,
	event Event,
	message string,
) error {
	if s == nil || s.events == nil {
		return nil
	}
	payload := struct {
		Preset    string                  `json:"preset"`
		EventID   string                  `json:"event_id"`
		EventType string                  `json:"event_type"`
		CursorKey notifications.CursorKey `json:"cursor_key"`
		LastError string                  `json:"last_error"`
		Sequence  int64                   `json:"sequence"`
	}{
		Preset:    preset.Name,
		EventID:   event.ID,
		EventType: event.Type,
		CursorKey: key,
		LastError: message,
		Sequence:  event.Sequence,
	}
	content, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("notifications: encode preset dispatch failure event: %w", err)
	}
	if err := s.events.WriteEventSummary(detachedContext(ctx), store.EventSummary{
		Type:      eventspkg.NotificationPresetDispatchFailed,
		Outcome:   string(eventspkg.OutcomeFor(eventspkg.NotificationPresetDispatchFailed)),
		Content:   content,
		Summary:   fmt.Sprintf("notification preset %s dispatch failed", preset.Name),
		Timestamp: s.now().UTC(),
	}); err != nil {
		return fmt.Errorf("notifications: record preset dispatch failure event: %w", err)
	}
	return nil
}

func (s *Service) checkStore() error {
	if s == nil || s.store == nil {
		return fmt.Errorf("%w: store is required", ErrInvalidPreset)
	}
	return nil
}

func (s *Service) checkDispatch() error {
	if err := s.checkStore(); err != nil {
		return err
	}
	if s.cursors == nil {
		return fmt.Errorf("%w: cursor store is required", ErrInvalidPreset)
	}
	if s.bridges == nil {
		return fmt.Errorf("%w: bridge runtime is required", ErrInvalidPreset)
	}
	return nil
}
