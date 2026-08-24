package core

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/session"
)

func (h *BaseHandlers) sessionPayloadsWithOptionalHealth(
	ctx context.Context,
	infos []*session.Info,
	includeHealth bool,
) ([]contract.SessionPayload, error) {
	var payloads []contract.SessionPayload
	var err error
	if !includeHealth {
		payloads = SessionPayloadsFromInfos(infos)
	} else {
		if h.SessionHealth == nil {
			return nil, errSessionHealthMissing
		}
		pageReader, ok := h.SessionHealth.(SessionHealthPageReader)
		if !ok {
			return nil, errSessionHealthMissing
		}
		payloads, err = SessionPayloadsWithPageHealth(ctx, infos, pageReader)
		if err != nil {
			return nil, err
		}
	}
	return h.decorateSessionOwners(ctx, payloads)
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
