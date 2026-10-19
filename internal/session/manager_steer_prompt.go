package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// SteerPrompt durably accepts guidance that replaces the fenced active turn.
func (m *Manager) SteerPrompt(ctx context.Context, id string, opts SteerPromptOpts) (SendPromptResult, error) {
	if m == nil {
		return SendPromptResult{}, errors.New("session: manager is required")
	}
	if ctx == nil {
		return SendPromptResult{}, errors.New("session: steer prompt context is required")
	}
	if err := validateCommandIngress(opts.AllowCommands, opts.Caller); err != nil {
		return SendPromptResult{}, err
	}
	req, err := m.parseSendPromptRequest(ctx, id, SendPromptOpts{Message: opts.Message, Attachments: opts.Attachments})
	if err != nil {
		return SendPromptResult{}, err
	}
	req.messageID = strings.TrimSpace(opts.MessageID)
	req.idempotencyKey = strings.TrimSpace(opts.IdempotencyKey)
	req.expectedTurnID = strings.TrimSpace(opts.ExpectedTurnID)
	req.runtime = cloneRuntimeSelection(opts.Runtime)
	if opts.AllowCommands {
		if err := m.preparePromptSkillInvocations(ctx, &req); err != nil {
			return SendPromptResult{}, err
		}
	}
	req.humanIngress = humanPromptCaller(opts.Caller)
	m.prepareMentions(ctx, &req)
	if err := req.validatePromptAdmissionIdentity(); err != nil {
		return SendPromptResult{}, err
	}
	session, err := m.lookupPromptSession(ctx, req.target)
	if err != nil {
		return SendPromptResult{}, err
	}
	if req.hasPromptAdmissionIdentity() {
		req.runtime, err = m.resolvePromptRuntimeAtAdmission(ctx, session, nil)
		if err != nil {
			return SendPromptResult{}, err
		}
		return m.submitAdmittedSteerCommand(ctx, session, req)
	}
	if err := m.canonicalizePromptRequestAttachments(ctx, session.WorkspaceID, session.ID, &req); err != nil {
		return SendPromptResult{}, err
	}
	if !session.IsPrompting() {
		return SendPromptResult{}, fmt.Errorf("%w: %s", ErrPromptNotInProgress, session.ID)
	}
	return m.stageSteerPrompt(ctx, session, req)
}
