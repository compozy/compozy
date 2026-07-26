package core

import (
	"context"
	"net/http"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/store"
)

func (h *BaseHandlers) provisionNetworkChannel(
	ctx context.Context,
	service NetworkService,
	networkStore NetworkStore,
	entry store.NetworkChannelEntry,
	agentNames []string,
) (contract.NetworkChannelDetailPayload, int, error) {
	if err := networkStore.CreateNetworkChannel(ctx, entry); err != nil {
		return contract.NetworkChannelDetailPayload{}, statusForCreateNetworkChannelError(err), err
	}
	ref := store.NetworkChannelRef{WorkspaceID: entry.WorkspaceID, Channel: entry.Channel}

	createdIDs, err := h.createNetworkChannelSessions(
		ctx,
		entry.Channel,
		entry.WorkspaceID,
		agentNames,
	)
	if err != nil {
		err = rollbackCreatedNetworkChannel(ctx, h.Sessions, networkStore, ref, createdIDs, err)
		return contract.NetworkChannelDetailPayload{}, StatusForSessionError(err), err
	}

	detail, err := h.networkChannelDetailPayload(ctx, service, entry.WorkspaceID, entry.Channel)
	if err != nil {
		err = rollbackCreatedNetworkChannel(ctx, h.Sessions, networkStore, ref, createdIDs, err)
		return contract.NetworkChannelDetailPayload{}, http.StatusInternalServerError, err
	}
	return detail, http.StatusCreated, nil
}
