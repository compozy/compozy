package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
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
	turnID, err := m.newPromptTurnID()
	if err != nil {
		return promptRequest{}, err
	}
	return promptRequest{
		turnID:          turnID,
		target:          target,
		message:         message,
		authoredMessage: message,
		turnSource:      turnSource,
		meta:            meta,
		resumeStopped:   true,
		deliveryCtx:     opts.DeliveryContext,
		prepareDelivery: opts.PrepareDelivery,
	}, nil
}

func (m *Manager) lookupOrResumePromptSession(ctx context.Context, target string) (*Session, error) {
	session, err := m.lookup(target)
	if err == nil {
		state := session.Info().State
		if run := m.sessionStartRun(session.ID); run != nil {
			if err := waitForSessionStartRun(ctx, run); err != nil {
				return nil, err
			}
			return m.lookupPromptSession(ctx, target)
		}
		if state == StateActive {
			return session, nil
		}
		if state == StateStopping || state == StateStopped {
			waited, waitErr := m.waitForSessionFinalization(ctx, target)
			if waitErr != nil {
				return nil, waitErr
			}
			if waited {
				return m.lookupOrResumePromptSession(ctx, target)
			}
		}
		return nil, fmt.Errorf("%w: %s (%s)", ErrSessionNotActive, target, state)
	}
	if !errors.Is(err, ErrSessionNotFound) {
		return nil, err
	}
	meta, metaErr := m.readMetaWithContext(ctx, target)
	if metaErr != nil {
		if errors.Is(metaErr, ErrSessionNotFound) {
			return nil, err
		}
		return nil, metaErr
	}
	if State(meta.State) != StateStopped {
		return nil, fmt.Errorf("%w: %s (%s)", ErrSessionNotActive, target, meta.State)
	}
	resumed, err := m.Resume(ctx, target)
	if err != nil {
		return nil, err
	}
	if run := m.sessionStartRun(resumed.ID); run != nil {
		if err := waitForSessionStartRun(ctx, run); err != nil {
			return nil, err
		}
	}
	return m.lookupPromptSession(ctx, target)
}

func (m *Manager) lookupPromptRequestSession(
	ctx context.Context,
	req promptRequest,
) (*Session, error) {
	if req.resumeStopped {
		return m.lookupOrResumePromptSession(ctx, req.target)
	}
	return m.lookupPromptSession(ctx, req.target)
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

func (m *Manager) newPromptTurnID() (string, error) {
	if m == nil || m.newTurnID == nil {
		return "", errors.New("session: turn id generator is required")
	}
	turnID, err := m.newTurnID()
	if err != nil {
		return "", fmt.Errorf("session: generate prompt turn id: %w", err)
	}
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		return "", errors.New("session: turn id generator returned empty id")
	}
	return turnID, nil
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
