package core

import (
	"context"
	"fmt"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/session"
)

func (h *BaseHandlers) sessionPayloadsWithOptionalHealth(
	ctx context.Context,
	infos []*session.Info,
	includeHealth bool,
) ([]contract.SessionPayload, error) {
	if !includeHealth {
		return SessionPayloadsFromInfos(infos), nil
	}
	if h.SessionHealth == nil {
		return nil, errSessionHealthMissing
	}
	pageReader, ok := h.SessionHealth.(SessionHealthPageReader)
	if !ok {
		return nil, errSessionHealthMissing
	}
	return SessionPayloadsWithPageHealth(ctx, infos, pageReader)
}

// SessionPayloadsWithPageHealth decorates one bounded session page from one
// batch health read. Callers must not fall back to per-session reads.
func SessionPayloadsWithPageHealth(
	ctx context.Context,
	infos []*session.Info,
	pageReader SessionHealthPageReader,
) ([]contract.SessionPayload, error) {
	if pageReader == nil {
		return nil, errSessionHealthMissing
	}
	healthByID, err := pageReader.SessionHealthForPage(ctx, infos)
	if err != nil {
		return nil, err
	}
	payloads := make([]contract.SessionPayload, 0, len(infos))
	for _, info := range infos {
		if info == nil {
			continue
		}
		payload := SessionPayloadFromInfo(info)
		health, exists := healthByID[payload.ID]
		if !exists {
			return nil, fmt.Errorf(
				"api: session health %q: %w",
				payload.ID,
				heartbeat.ErrSessionHealthNotFound,
			)
		}
		payload.Badge = session.BadgeForHealth(info, health)
		converted, err := contract.SessionHealthPayloadFromDomain(health)
		if err != nil {
			return nil, err
		}
		payload.Health = &converted
		payloads = append(payloads, payload)
	}
	return payloads, nil
}
