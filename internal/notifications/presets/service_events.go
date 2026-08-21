package presets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/notifications"
	"github.com/compozy/compozy/internal/store"
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
	report, normalizeErr := (notifications.CursorError{
		Key:       key,
		LastError: err.Error(),
		Now:       s.now(),
	}).Normalize(s.now())
	if normalizeErr != nil {
		return &dispatchDiagnosticError{
			cause:      err,
			diagnostic: "notification delivery failure",
		}
	}
	safeErr := &dispatchDiagnosticError{
		cause:      err,
		diagnostic: report.LastError,
	}
	if _, recordErr := s.cursors.RecordError(ctx, report); recordErr != nil {
		safeErr.cause = errors.Join(err, recordErr)
		return safeErr
	}
	if eventErr := s.recordDispatchFailureEvent(ctx, key, preset, event, report.LastError); eventErr != nil {
		safeErr.cause = errors.Join(err, eventErr)
		return safeErr
	}
	return safeErr
}

func (s *Service) recordPresetEnablementEvent(
	ctx context.Context,
	preset Preset,
	change EnablementChange,
) error {
	if s == nil || s.events == nil {
		return nil
	}
	payload := struct {
		Preset      string `json:"preset"`
		ProfileID   string `json:"profile_id"`
		ProfileName string `json:"profile_name"`
		ActorKind   string `json:"actor_kind"`
		Enabled     bool   `json:"enabled"`
	}{
		Preset: preset.Name, ProfileID: change.ProfileID, ProfileName: change.ProfileName,
		ActorKind: change.ActorKind, Enabled: preset.Enabled,
	}
	content, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("notifications: encode preset enablement event: %w", err)
	}
	event := store.EventSummary{
		ProfileID: change.ProfileID,
		Type:      eventspkg.NotificationPresetEnablementChanged,
		Outcome:   string(eventspkg.OutcomeFor(eventspkg.NotificationPresetEnablementChanged)),
		Summary: fmt.Sprintf(
			"notification preset %s enablement changed for profile %s",
			preset.Name,
			change.ProfileName,
		),
		Timestamp: s.now().UTC(),
		EventCorrelation: store.EventCorrelation{
			ActorKind: change.ActorKind, ActorID: change.ActorID,
		},
	}
	event.SetContent(content)
	if err := s.events.WriteEventSummary(detachedContext(ctx), event); err != nil {
		return fmt.Errorf("notifications: record preset enablement event: %w", err)
	}
	return nil
}

// dispatchDiagnosticError preserves the cause for errors.Is and errors.As
// while ensuring the surfaced error string is safe for logs and event paths.
type dispatchDiagnosticError struct {
	cause      error
	diagnostic string
}

func (e *dispatchDiagnosticError) Error() string {
	return e.diagnostic
}

func (e *dispatchDiagnosticError) Unwrap() error {
	return e.cause
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
		BuiltIn                bool     `json:"built_in"`
		UserModified           bool     `json:"user_modified"`
		DefaultUpdateAvailable bool     `json:"default_update_available"`
	}{
		Name:                   preset.Name,
		Events:                 append([]string(nil), preset.Events...),
		BuiltIn:                preset.BuiltIn,
		UserModified:           preset.UserModified,
		DefaultUpdateAvailable: preset.DefaultUpdateAvailable,
	}
	content, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("notifications: encode preset lifecycle event: %w", err)
	}
	event := store.EventSummary{
		ProfileID: store.DefaultProfileID,
		Type:      eventType,
		Outcome:   string(eventspkg.OutcomeFor(eventType)),
		Summary:   notificationPresetLifecycleSummary(eventType, preset.Name),
		Timestamp: s.now().UTC(),
	}
	event.SetContent(content)
	if err := s.events.WriteEventSummary(detachedContext(ctx), event); err != nil {
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
	summary := store.EventSummary{
		ProfileID: strings.TrimSpace(event.ProfileID),
		Type:      eventspkg.NotificationPresetDispatchFailed,
		Outcome:   string(eventspkg.OutcomeFor(eventspkg.NotificationPresetDispatchFailed)),
		Summary:   fmt.Sprintf("notification preset %s dispatch failed", preset.Name),
		Timestamp: s.now().UTC(),
	}
	summary.SetContent(content)
	if err := s.events.WriteEventSummary(detachedContext(ctx), summary); err != nil {
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
