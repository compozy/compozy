package task

import (
	"context"
	"errors"
	"strings"
	"time"
)

type ActivationCancelOutcome struct {
	Won     bool
	Claimed bool
}

type ActivationRunCancelStore interface {
	CancelCallActivationRun(context.Context, string, string, time.Time) (ActivationCancelOutcome, error)
}

func (m *Service) CancelActivationRun(
	ctx context.Context,
	runID string,
	reason string,
) (ActivationCancelOutcome, error) {
	store, ok := m.store.(ActivationRunCancelStore)
	if !ok || store == nil {
		return ActivationCancelOutcome{}, errors.New("task: store does not support call activation cancellation")
	}
	return store.CancelCallActivationRun(ctx, strings.TrimSpace(runID), strings.TrimSpace(reason), m.now().UTC())
}
