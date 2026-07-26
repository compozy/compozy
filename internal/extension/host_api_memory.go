package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"strings"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/compozy/agh/internal/store"
)

func (h *HostAPIHandler) handleMemoryStore(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPIMemoryStoreParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.Key) == "" {
		return nil, invalidParamsRPCError(errors.New("key is required"))
	}
	if strings.TrimSpace(params.Content) == "" {
		return nil, invalidParamsRPCError(errors.New("content is required"))
	}

	storeHandle, scope, err := h.memoryStoreFor(ctx, string(params.Scope), params.Workspace)
	if err != nil {
		return nil, err
	}

	filename := normalizeMemoryFilename(params.Key)
	doc, err := renderMemoryDocument(hostAPIMemoryDocument{
		Key:     filename,
		Scope:   scope,
		Content: params.Content,
		Tags:    params.Tags,
	})
	if err != nil {
		return nil, err
	}
	result, err := storeHandle.ProposeWrite(ctx, scope, filename, []byte(doc), memcontract.OriginTool)
	if err != nil {
		return nil, err
	}
	if result.Decision.Op == memcontract.OpReject {
		return nil, invalidParamsRPCError(fmt.Errorf("memory write rejected: %s", result.Decision.Reason))
	}
	return struct{}{}, nil
}

func (h *HostAPIHandler) handleMemoryRecall(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPIMemoryRecallParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	query := strings.TrimSpace(params.Query)
	if query == "" {
		return nil, invalidParamsRPCError(errors.New("query is required"))
	}

	limit := params.Limit
	if limit <= 0 {
		limit = defaultHostAPIRecallLimit
	}

	packaged, err := h.recallMemory(ctx, query, hostAPIMemoryRecallSelection{
		Limit:     limit,
		Scope:     params.Scope,
		Workspace: params.Workspace,
	})
	if err != nil {
		return nil, err
	}

	return hostAPIMemoryRecallEntries(packaged, limit), nil
}

func (h *HostAPIHandler) handleMemoryForget(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPIMemoryForgetParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.Key) == "" {
		return nil, invalidParamsRPCError(errors.New("key is required"))
	}

	storeHandle, scope, err := h.memoryStoreFor(ctx, string(params.Scope), params.Workspace)
	if err != nil {
		return nil, err
	}
	if _, err := storeHandle.ProposeDelete(
		ctx,
		scope,
		normalizeMemoryFilename(params.Key),
		memcontract.OriginTool,
	); err != nil {
		return nil, err
	}
	return struct{}{}, nil
}

func (h *HostAPIHandler) handleObserveHealth(ctx context.Context, _ json.RawMessage) (any, error) {
	if h.observer == nil {
		return nil, errors.New("extension: observer is not configured")
	}
	return h.observer.Health(ctx)
}

func (h *HostAPIHandler) handleListLogs(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.observer == nil {
		return nil, errors.New("extension: observer is not configured")
	}

	var params hostAPIListLogsParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	workspaceID, err := h.hostAPINetworkWorkspaceID(ctx, params.WorkspaceID)
	if err != nil {
		return nil, err
	}

	events, err := h.observer.QueryEvents(ctx, store.EventSummaryQuery{
		WorkspaceID:   workspaceID,
		SessionID:     strings.TrimSpace(params.SessionID),
		AgentName:     strings.TrimSpace(params.AgentName),
		Type:          strings.TrimSpace(params.Type),
		RunID:         strings.TrimSpace(params.RunID),
		ActorKind:     strings.TrimSpace(params.ActorKind),
		ActorID:       strings.TrimSpace(params.ActorID),
		Provider:      strings.TrimSpace(params.Provider),
		Outcome:       strings.TrimSpace(params.Outcome),
		Component:     strings.TrimSpace(params.Component),
		ErrorOnly:     params.ErrorOnly,
		AfterSequence: params.AfterSequence,
		Since:         params.Since,
		Limit:         params.Limit,
	})
	if err != nil {
		return nil, err
	}

	result := make([]hostAPISessionEvent, 0, len(events))
	for _, event := range events {
		result = append(result, hostAPISessionEvent{
			Type:      event.Type,
			Timestamp: event.Timestamp,
			Data: map[string]any{
				hostAPIWorkspaceIDKey: event.WorkspaceID,
				hostAPISessionIDKey:   event.SessionID,
				hostAPIAgentNameKey:   event.AgentName,
				"summary":             event.Summary,
			},
		})
	}

	return result, nil
}
