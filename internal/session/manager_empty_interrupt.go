package session

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) sendEmptyInterrupt(ctx context.Context, req promptRequest) (SendPromptResult, error) {
	if !req.hasPromptAdmissionIdentity() {
		return m.cancelEmptyInterrupt(ctx, req)
	}
	info, err := m.Status(ctx, req.target)
	if err != nil {
		return SendPromptResult{}, err
	}
	request, err := m.newPromptAdmissionRequest(
		info.WorkspaceID,
		req,
		store.SessionPromptOperationPrompt,
		BusyInputModeInterrupt,
	)
	if err != nil {
		return SendPromptResult{}, err
	}
	unlock := m.lockPromptAdmission(info.WorkspaceID, req.target, req.idempotencyKey)
	defer unlock()
	if replayed, _, err := m.replayPromptAdmission(ctx, request); err != nil || replayed != nil {
		if err != nil {
			return SendPromptResult{}, err
		}
		return *replayed, nil
	}
	if active, ok := m.Get(req.target); ok && req.expectedTurnID != "" {
		if _, err := requireExpectedActiveTurn(active, req.expectedTurnID); err != nil {
			return SendPromptResult{}, err
		}
	}
	admission, replayed, err := m.claimPromptAdmission(ctx, request)
	if err != nil {
		return SendPromptResult{}, err
	}
	if replayed != nil {
		return *replayed, nil
	}
	if err := m.commitPromptAdmissionDispatch(ctx, admission); err != nil {
		return SendPromptResult{}, err
	}
	result, err := m.cancelEmptyInterrupt(ctx, req)
	if err != nil {
		return SendPromptResult{}, m.promptDispatchIndeterminate(ctx, admission, err)
	}
	return m.completePromptAdmission(context.WithoutCancel(ctx), admission, result)
}

func (m *Manager) cancelEmptyInterrupt(ctx context.Context, req promptRequest) (SendPromptResult, error) {
	result := SendPromptResult{
		Status: store.SessionPromptResultStatusCanceled,
		Mode:   BusyInputModeInterrupt, Delivery: store.SessionInputDeliveryNone,
	}
	if _, ok := m.Get(req.target); !ok {
		if _, err := m.Status(ctx, req.target); err != nil {
			return SendPromptResult{}, err
		}
		return result, nil
	}
	run, err := m.requestTurnStop(ctx, req.target, req.expectedTurnID, CauseUserRequested)
	if errors.Is(err, ErrPromptNotInProgress) {
		return result, nil
	}
	if err != nil {
		return SendPromptResult{}, err
	}
	result.PreviousTurnID = run.turnID
	return result, nil
}
