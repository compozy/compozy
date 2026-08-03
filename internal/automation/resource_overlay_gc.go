package automation

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

func (m *Manager) pruneJobResourceOverlays(ctx context.Context, next map[string]Job) error {
	overlays, err := m.store.ListJobEnabledOverlays(ctx)
	if err != nil {
		return fmt.Errorf("automation: list job overlays for resource projection: %w", err)
	}
	slices.SortFunc(overlays, func(left, right JobEnabledOverlay) int {
		return strings.Compare(left.JobID, right.JobID)
	})
	deleted := make([]JobEnabledOverlay, 0, len(overlays))
	for _, overlay := range overlays {
		if _, exists := next[strings.TrimSpace(overlay.JobID)]; exists {
			continue
		}
		if err := m.store.DeleteJobEnabledOverlay(ctx, overlay.JobID); err != nil {
			return errors.Join(
				fmt.Errorf("automation: delete retired job overlay %q: %w", overlay.JobID, err),
				m.restoreJobResourceOverlays(ctx, deleted),
			)
		}
		deleted = append(deleted, overlay)
	}
	return nil
}

func (m *Manager) restoreJobResourceOverlays(ctx context.Context, overlays []JobEnabledOverlay) error {
	ctx = persistenceContext(ctx)
	var restoreErr error
	for _, overlay := range slices.Backward(overlays) {
		if _, err := m.store.SetJobEnabledOverlay(ctx, overlay); err != nil {
			restoreErr = errors.Join(
				restoreErr,
				fmt.Errorf("automation: restore retired job overlay %q: %w", overlay.JobID, err),
			)
		}
	}
	return restoreErr
}

func (m *Manager) pruneTriggerResourceOverlays(ctx context.Context, next map[string]Trigger) error {
	overlays, err := m.store.ListTriggerEnabledOverlays(ctx)
	if err != nil {
		return fmt.Errorf("automation: list trigger overlays for resource projection: %w", err)
	}
	slices.SortFunc(overlays, func(left, right TriggerEnabledOverlay) int {
		return strings.Compare(left.TriggerID, right.TriggerID)
	})
	deleted := make([]TriggerEnabledOverlay, 0, len(overlays))
	for _, overlay := range overlays {
		if _, exists := next[strings.TrimSpace(overlay.TriggerID)]; exists {
			continue
		}
		if err := m.store.DeleteTriggerEnabledOverlay(ctx, overlay.TriggerID); err != nil {
			return errors.Join(
				fmt.Errorf("automation: delete retired trigger overlay %q: %w", overlay.TriggerID, err),
				m.restoreTriggerResourceOverlays(ctx, deleted),
			)
		}
		deleted = append(deleted, overlay)
	}
	return nil
}

func (m *Manager) restoreTriggerResourceOverlays(ctx context.Context, overlays []TriggerEnabledOverlay) error {
	ctx = persistenceContext(ctx)
	var restoreErr error
	for _, overlay := range slices.Backward(overlays) {
		if _, err := m.store.SetTriggerEnabledOverlay(ctx, overlay); err != nil {
			restoreErr = errors.Join(
				restoreErr,
				fmt.Errorf("automation: restore retired trigger overlay %q: %w", overlay.TriggerID, err),
			)
		}
	}
	return restoreErr
}
