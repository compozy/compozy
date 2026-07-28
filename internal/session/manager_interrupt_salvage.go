package session

import (
	"context"
	"fmt"
	"strings"
)

const interruptedPromptCorrectionPrefix = "User correction/guidance after interrupt: "

type interruptedPromptSalvage struct {
	generation int64
	message    string
}

func (m *Manager) stageInterruptedPromptSalvage(sessionID string, generation int64, message string) {
	if m == nil {
		return
	}
	target := strings.TrimSpace(sessionID)
	message = strings.TrimSpace(message)
	m.interruptSalvageMu.Lock()
	defer m.interruptSalvageMu.Unlock()
	if target == "" || message == "" {
		delete(m.interruptSalvages, target)
		return
	}
	if m.interruptSalvages == nil {
		m.interruptSalvages = make(map[string]interruptedPromptSalvage)
	}
	m.interruptSalvages[target] = interruptedPromptSalvage{generation: generation, message: message}
}

func (m *Manager) pendingInterruptedPromptSalvage(sessionID string) (interruptedPromptSalvage, bool) {
	if m == nil {
		return interruptedPromptSalvage{}, false
	}
	m.interruptSalvageMu.Lock()
	defer m.interruptSalvageMu.Unlock()
	salvage, ok := m.interruptSalvages[strings.TrimSpace(sessionID)]
	return salvage, ok
}

func (m *Manager) consumeInterruptedPromptSalvage(
	sessionID string,
	generation int64,
) (interruptedPromptSalvage, bool) {
	if m == nil {
		return interruptedPromptSalvage{}, false
	}
	target := strings.TrimSpace(sessionID)
	m.interruptSalvageMu.Lock()
	defer m.interruptSalvageMu.Unlock()
	salvage, ok := m.interruptSalvages[target]
	if !ok || salvage.generation != generation {
		return interruptedPromptSalvage{}, false
	}
	delete(m.interruptSalvages, target)
	return salvage, true
}

func (m *Manager) discardInterruptedPromptSalvage(sessionID string) {
	if m == nil {
		return
	}
	m.interruptSalvageMu.Lock()
	defer m.interruptSalvageMu.Unlock()
	delete(m.interruptSalvages, strings.TrimSpace(sessionID))
}

func (m *Manager) submitInterruptedPromptSalvage(
	ctx context.Context,
	session *Session,
	req promptRequest,
	pending interruptedPromptSalvage,
) (SendPromptResult, error) {
	waitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), m.supervision.TimeoutCancelGrace)
	defer cancel()
	if err := waitForPromptIdle(waitCtx, session); err != nil {
		return SendPromptResult{}, err
	}
	generation, err := m.currentInputGeneration(ctx, session.ID)
	if err != nil {
		return SendPromptResult{}, err
	}
	if generation != pending.generation {
		m.discardInterruptedPromptSalvage(session.ID)
		return SendPromptResult{}, fmt.Errorf("%w: %s", ErrPromptNotInProgress, session.ID)
	}
	salvage, ok := m.consumeInterruptedPromptSalvage(session.ID, generation)
	if !ok {
		return SendPromptResult{}, fmt.Errorf("%w: %s", ErrPromptNotInProgress, session.ID)
	}
	composed := composeInterruptedPromptSalvage(salvage.message, req.authoredMessage)
	req.message = composed
	req.authoredMessage = composed
	events, err := m.submitPromptRequest(ctx, req)
	if err != nil {
		return SendPromptResult{}, err
	}
	return SendPromptResult{
		Status:          promptStatusAccepted,
		Mode:            BusyInputModeSteer,
		Events:          events,
		NewTurnID:       req.turnID,
		QueueGeneration: generation,
	}, nil
}

func composeInterruptedPromptSalvage(interrupted string, correction string) string {
	return strings.TrimSpace(interrupted) + "\n\n" + interruptedPromptCorrectionPrefix + strings.TrimSpace(correction)
}
