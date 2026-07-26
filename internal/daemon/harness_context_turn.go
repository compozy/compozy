package daemon

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/acp"

	"github.com/compozy/agh/internal/session"
)

func (r *HarnessContextResolver) normalizeHarnessTurnContext(
	sessionCtx HarnessSessionContext,
	request HarnessTurnRequest,
) (HarnessTurnContext, error) {
	normalizedMeta := request.PromptMeta.Normalize()
	origin, err := turnOriginFromSource(request.Source)
	if err != nil {
		return HarnessTurnContext{}, err
	}
	// Network turn origin is provenance only. Live participation remains gated by
	// session.PromptNetwork; bridge ingress uses PromptWithOpts with network meta
	// on Local sessions and must still resolve harness policy.

	if request.Synthetic != nil && origin != TurnOriginSynthetic {
		return HarnessTurnContext{}, errors.New(
			"daemon: synthetic harness metadata requires the synthetic turn origin",
		)
	}

	detached, err := normalizeDetachedRunMetadata(request.Detached)
	if err != nil {
		return HarnessTurnContext{}, err
	}
	if origin == TurnOriginSynthetic {
		return r.normalizeSyntheticHarnessTurnContext(sessionCtx, normalizedMeta, request.Synthetic, detached)
	}

	normalizedMeta, err = normalizePromptMetaForHarnessTurn(origin, normalizedMeta)
	if err != nil {
		return HarnessTurnContext{}, err
	}
	return HarnessTurnContext{
		Origin:     origin,
		PromptMeta: normalizedMeta,
		Detached:   detached,
	}, nil
}

func normalizePromptMetaForHarnessTurn(
	origin TurnOrigin,
	meta acp.PromptMeta,
) (acp.PromptMeta, error) {
	switch origin {
	case TurnOriginUser:
		return normalizePromptMetaForExpectedTurnSource(origin, meta, acp.PromptTurnSourceUser)
	case TurnOriginNetwork:
		return normalizePromptMetaForExpectedTurnSource(origin, meta, acp.PromptTurnSourceNetwork)
	default:
		return acp.PromptMeta{}, fmt.Errorf("daemon: invalid harness turn origin %q", origin)
	}
}

func normalizePromptMetaForExpectedTurnSource(
	origin TurnOrigin,
	meta acp.PromptMeta,
	expected string,
) (acp.PromptMeta, error) {
	if meta.TurnSource == "" {
		meta.TurnSource = expected
	}
	if meta.TurnSource != expected {
		return acp.PromptMeta{}, fmt.Errorf(
			"daemon: harness turn origin %q does not match prompt metadata turn_source %q",
			origin,
			meta.TurnSource,
		)
	}
	if err := meta.Validate(); err != nil {
		return acp.PromptMeta{}, err
	}
	return meta, nil
}

func (r *HarnessContextResolver) normalizeSyntheticHarnessTurnContext(
	sessionCtx HarnessSessionContext,
	meta acp.PromptMeta,
	syntheticInput *SyntheticTurnMetadata,
	detached *DetachedRunMetadata,
) (HarnessTurnContext, error) {
	if !r.runtime.SyntheticTurnsEnabled {
		return HarnessTurnContext{}, errors.New("daemon: synthetic harness turns are not enabled")
	}
	if sessionCtx.Type != session.SessionTypeSystem {
		return HarnessTurnContext{}, fmt.Errorf(
			"daemon: synthetic harness turns require a system session, got %q",
			sessionCtx.Type,
		)
	}
	if meta.Network != nil {
		return HarnessTurnContext{}, errors.New(
			"daemon: synthetic harness turns cannot include network prompt metadata",
		)
	}
	synthetic, err := normalizeSyntheticTurnMetadata(syntheticInput)
	if err != nil {
		return HarnessTurnContext{}, err
	}
	meta.TurnSource = acp.PromptTurnSourceSynthetic
	return HarnessTurnContext{
		Origin:     TurnOriginSynthetic,
		PromptMeta: meta,
		Synthetic:  synthetic,
		Detached:   detached,
	}, nil
}

func turnOriginFromSource(source session.TurnSource) (TurnOrigin, error) {
	switch session.TurnSource(strings.TrimSpace(string(source))) {
	case "", session.TurnSourceUser:
		return TurnOriginUser, nil
	case session.TurnSourceNetwork:
		return TurnOriginNetwork, nil
	case session.TurnSourceSynthetic:
		return TurnOriginSynthetic, nil
	default:
		return "", fmt.Errorf("daemon: invalid harness turn origin %q", source)
	}
}

func normalizeSyntheticTurnMetadata(input *SyntheticTurnMetadata) (*SyntheticTurnMetadata, error) {
	if input == nil {
		return nil, errors.New("daemon: synthetic harness turns require runtime metadata")
	}

	normalized := &SyntheticTurnMetadata{
		Reason:      strings.TrimSpace(input.Reason),
		Trigger:     strings.TrimSpace(input.Trigger),
		SourceTask:  strings.TrimSpace(input.SourceTask),
		SourceRunID: strings.TrimSpace(input.SourceRunID),
	}
	if normalized.Reason == "" {
		return nil, errors.New("daemon: synthetic harness turns require a reason")
	}
	if normalized.Trigger == "" {
		return nil, errors.New("daemon: synthetic harness turns require a trigger")
	}
	return normalized, nil
}

func normalizeDetachedRunMetadata(input *DetachedRunMetadata) (*DetachedRunMetadata, error) {
	if input == nil {
		return nil, nil
	}

	normalized := &DetachedRunMetadata{
		TaskID:    strings.TrimSpace(input.TaskID),
		TaskRunID: strings.TrimSpace(input.TaskRunID),
	}
	if normalized.TaskID == "" && normalized.TaskRunID == "" {
		return nil, errors.New("daemon: detached harness metadata requires a task id or task run id")
	}
	return normalized, nil
}
