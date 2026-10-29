package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/compozy/compozy/internal/heartbeat"

	"github.com/compozy/compozy/internal/api/contract"

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
	if err := DecorateSessionPullRequests(ctx, h.Sessions, infos, payloads); err != nil {
		return nil, err
	}
	if err := h.decorateSubagentSummaries(ctx, payloads); err != nil {
		return nil, err
	}
	h.decorateMissingSessionWorkspaces(ctx, payloads)
	h.decorateMonitorSummaries(payloads)
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
		payload.BoardColumn = session.BoardColumnFor(payload.Badge, "")
		payload.WaitingOn = sessionPayloadWaitingOn(info, payload.Badge)
		converted, err := contract.SessionHealthPayloadFromDomain(health)
		if err != nil {
			return nil, err
		}
		payload.Health = &converted
		payloads = append(payloads, payload)
	}
	return payloads, nil
}

// decorateMissingSessionWorkspaces uses one registration read for the entire page.
func (h *BaseHandlers) decorateMissingSessionWorkspaces(ctx context.Context, payloads []contract.SessionPayload) {
	if len(payloads) == 0 || h.Workspaces == nil {
		return
	}
	workspaces, err := h.Workspaces.List(ctx)
	if err != nil {
		slog.WarnContext(ctx, "session catalog workspace diagnostics unavailable", "error", err)
		return
	}
	missing := make(map[string]bool)
	for _, workspace := range workspaces {
		if workspace.RootMissing {
			missing[workspace.ID] = true
		}
	}
	for index := range payloads {
		if missing[payloads[index].WorkspaceID] {
			payloads[index].Diagnostics = MissingWorkspaceDiagnostics(payloads[index].WorkspaceID)
		}
	}
}
