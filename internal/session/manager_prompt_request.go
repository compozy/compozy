package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/acp"
)

func (m *Manager) parsePromptRequest(ctx context.Context, id string, opts PromptOpts) (promptRequest, error) {
	if ctx == nil {
		return promptRequest{}, errors.New("session: prompt context is required")
	}
	target := strings.TrimSpace(id)
	if target == "" {
		return promptRequest{}, errors.New("session: session id is required")
	}
	message := strings.TrimSpace(opts.Message)
	if message == "" {
		return promptRequest{}, errors.New("session: prompt message is required")
	}
	turnSource := normalizeTurnSource(opts.TurnSource)
	if turnSource == "" {
		return promptRequest{}, fmt.Errorf(
			"session: invalid turn source %q",
			strings.TrimSpace(string(opts.TurnSource)),
		)
	}
	meta, err := normalizePromptMeta(turnSource, opts.PromptMeta, promptSubmissionPathUserFacing)
	if err != nil {
		return promptRequest{}, err
	}
	return promptRequest{
		turnID:          m.newPromptTurnID(),
		target:          target,
		message:         message,
		authoredMessage: message,
		turnSource:      turnSource,
		meta:            meta,
		deliveryCtx:     opts.DeliveryContext,
		prepareDelivery: opts.PrepareDelivery,
	}, nil
}

func normalizePromptMeta(
	turnSource TurnSource,
	meta acp.PromptMeta,
	path promptSubmissionPath,
) (acp.PromptMeta, error) {
	normalized := meta.Normalize()
	if normalized.TurnSource == "" {
		normalized.TurnSource = string(turnSource)
	}
	if normalized.TurnSource != string(turnSource) {
		return acp.PromptMeta{}, fmt.Errorf(
			"session: prompt turn source %q does not match metadata turn_source %q",
			turnSource,
			normalized.TurnSource,
		)
	}
	if turnSource == TurnSourceSynthetic {
		if path != promptSubmissionPathSynthetic {
			return acp.PromptMeta{}, errors.New(
				"session: synthetic prompt turns require the dedicated synthetic submission path",
			)
		}
		if normalized.Synthetic == nil {
			return acp.PromptMeta{}, errors.New(
				"session: synthetic prompt turns require synthetic metadata",
			)
		}
	}
	if turnSource == TurnSourceUser && normalized.Network != nil {
		return acp.PromptMeta{}, errors.New("session: user prompt metadata cannot include network fields")
	}
	if err := normalized.Validate(); err != nil {
		return acp.PromptMeta{}, err
	}
	return normalized, nil
}

func (m *Manager) newPromptTurnID() string {
	if m == nil || m.newTurnID == nil {
		return newID("turn")
	}
	turnID := strings.TrimSpace(m.newTurnID())
	if turnID == "" {
		return newID("turn")
	}
	return turnID
}

func (m *Manager) lookupPromptSession(ctx context.Context, target string) (*Session, error) {
	session, err := m.lookup(target)
	if err == nil {
		if m.sessionStartRun(session.ID) != nil {
			return nil, fmt.Errorf("%w: %s (%s)", ErrSessionNotActive, target, StateStarting)
		}
		return session, nil
	}
	if !errors.Is(err, ErrSessionNotFound) {
		return nil, err
	}
	meta, metaErr := m.readMetaWithContext(ctx, target)
	switch {
	case metaErr == nil:
		return nil, fmt.Errorf("%w: %s (%s)", ErrSessionNotActive, target, meta.State)
	case errors.Is(metaErr, ErrSessionNotFound):
		return nil, err
	default:
		return nil, metaErr
	}
}
