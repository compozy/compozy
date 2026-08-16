package session

import (
	"context"
	"errors"
	"strings"
)

func (m *Manager) parseSendPromptRequest(
	ctx context.Context,
	id string,
	opts SendPromptOpts,
) (promptRequest, error) {
	attachments := cloneAttachmentMeta(opts.Attachments)
	message := strings.TrimSpace(opts.Message)
	if message == "" && len(attachments) == 0 {
		return promptRequest{}, errors.New("session: prompt message is required")
	}
	if message != "" {
		req, err := m.parsePromptRequest(ctx, id, PromptOpts{
			Message:         opts.Message,
			TurnSource:      TurnSourceUser,
			DeliveryContext: opts.DeliveryContext,
		})
		if err != nil {
			return promptRequest{}, err
		}
		req.attachments = attachments
		return req, nil
	}
	return m.parseAttachmentOnlyPromptRequest(ctx, id, opts, attachments)
}

func (m *Manager) parseAttachmentOnlyPromptRequest(
	ctx context.Context,
	id string,
	opts SendPromptOpts,
	attachments []AttachmentMeta,
) (promptRequest, error) {
	req, err := m.parsePromptRequestWithMessagePolicy(ctx, id, PromptOpts{
		TurnSource:      TurnSourceUser,
		DeliveryContext: opts.DeliveryContext,
	}, true)
	if err != nil {
		return promptRequest{}, err
	}
	req.attachments = attachments
	return req, nil
}

func (m *Manager) canonicalizePromptRequestAttachments(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	req *promptRequest,
) error {
	if req == nil {
		return errors.New("session: prompt request is required")
	}
	attachments, err := m.canonicalPromptAttachments(ctx, workspaceID, sessionID, req.attachments)
	if err != nil {
		return err
	}
	req.attachments = attachments
	return nil
}
