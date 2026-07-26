package extensionpkg

import (
	"context"

	"errors"
	"fmt"

	"strings"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/compozy/agh/internal/memory"
)

type hostAPIMemoryRecallSelection struct {
	Limit     int
	Scope     memcontract.Scope
	Workspace string
}

func (h *HostAPIHandler) recallMemory(
	ctx context.Context,
	query string,
	selection hostAPIMemoryRecallSelection,
) (memcontract.Packaged, error) {
	workspaceID, err := h.resolveStableWorkspaceID(ctx, selection.Workspace)
	if err != nil {
		return memcontract.Packaged{}, err
	}
	if selection.Scope.Normalize() == memcontract.ScopeGlobal {
		workspaceID = ""
	}
	if providerRecall, ok, err := h.recallMemoryFromProvider(
		ctx,
		query,
		workspaceID,
		selection.Limit,
	); ok ||
		err != nil {
		return providerRecall, err
	}
	return h.recallMemoryFromStore(ctx, query, workspaceID, selection)
}

func (h *HostAPIHandler) recallMemoryFromProvider(
	ctx context.Context,
	query string,
	workspaceID string,
	limit int,
) (memcontract.Packaged, bool, error) {
	if h.memoryProviders == nil {
		return memcontract.Packaged{}, false, nil
	}
	registration, err := h.memoryProviders.Select(ctx, workspaceID, "")
	if err != nil {
		if errors.Is(err, ErrMemoryProviderNotFound) {
			return memcontract.Packaged{}, false, nil
		}
		return memcontract.Packaged{}, true, err
	}
	recalled, err := registration.Provider.Recall(ctx, memcontract.RecallRequest{
		Query: memcontract.Query{
			WorkspaceID: workspaceID,
			QueryText:   query,
		},
		Options: memcontract.RecallOptions{TopK: limit},
	})
	if err != nil {
		if errors.Is(err, memcontract.ErrNotImplemented) {
			return memcontract.Packaged{}, false, nil
		}
		return memcontract.Packaged{}, true, err
	}
	return recalled.Packaged, true, nil
}

func (h *HostAPIHandler) recallMemoryFromStore(
	ctx context.Context,
	query string,
	workspaceID string,
	selection hostAPIMemoryRecallSelection,
) (memcontract.Packaged, error) {
	storeHandle, _, err := h.memoryStoreFor(ctx, string(selection.Scope), selection.Workspace)
	if err != nil {
		return memcontract.Packaged{}, err
	}
	return storeHandle.Recall(ctx, memcontract.Query{
		WorkspaceID: workspaceID,
		QueryText:   query,
	}, memcontract.RecallOptions{TopK: selection.Limit})
}

func (h *HostAPIHandler) memoryStoreFor(
	ctx context.Context,
	rawScope string,
	rawWorkspace string,
) (*memory.Store, memcontract.Scope, error) {
	if h.memory == nil {
		return nil, "", errors.New("extension: memory store is not configured")
	}

	scope := memcontract.Scope(strings.TrimSpace(rawScope)).Normalize()
	workspaceRoot, err := h.resolveWorkspaceRoot(ctx, rawWorkspace)
	if err != nil {
		return nil, "", err
	}
	if scope == "" {
		if workspaceRoot != "" {
			scope = memcontract.ScopeWorkspace
		} else {
			scope = memcontract.ScopeGlobal
		}
	}

	switch scope {
	case memcontract.ScopeGlobal:
		return h.memory, memcontract.ScopeGlobal, nil
	case memcontract.ScopeWorkspace:
		if workspaceRoot == "" {
			return nil, "", invalidParamsRPCError(errors.New("workspace is required for workspace memory scope"))
		}
		return h.memory.ForWorkspace(workspaceRoot), memcontract.ScopeWorkspace, nil
	default:
		return nil, "", invalidParamsRPCError(fmt.Errorf("memory scope must be one of global or workspace"))
	}
}
