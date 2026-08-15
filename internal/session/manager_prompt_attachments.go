package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
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
	if ctx == nil {
		return promptRequest{}, errors.New("session: prompt context is required")
	}
	target := strings.TrimSpace(id)
	if target == "" {
		return promptRequest{}, errors.New("session: session id is required")
	}
	turnSource := normalizeTurnSource(TurnSourceUser)
	if turnSource == "" {
		return promptRequest{}, fmt.Errorf(
			"session: invalid turn source %q",
			strings.TrimSpace(string(TurnSourceUser)),
		)
	}
	meta, err := normalizePromptMeta(turnSource, acp.PromptMeta{}, promptSubmissionPathUserFacing)
	if err != nil {
		return promptRequest{}, err
	}
	turnID, err := m.newPromptTurnID()
	if err != nil {
		return promptRequest{}, err
	}
	return promptRequest{
		turnID:          turnID,
		target:          target,
		message:         "",
		authoredMessage: "",
		turnSource:      turnSource,
		meta:            meta,
		resumeStopped:   true,
		deliveryCtx:     opts.DeliveryContext,
		attachments:     attachments,
	}, nil
}
