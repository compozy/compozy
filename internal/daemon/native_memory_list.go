package daemon

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	memorypkg "github.com/compozy/agh/internal/memory"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type memoryListInput struct {
	Scope         string `json:"scope,omitempty"`
	Workspace     string `json:"workspace,omitempty"`
	AgentName     string `json:"agent_name,omitempty"`
	AgentTier     string `json:"agent_tier,omitempty"`
	Type          string `json:"type,omitempty"`
	Sort          string `json:"sort,omitempty"`
	Cursor        string `json:"cursor,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	IncludeSystem bool   `json:"include_system,omitempty"`
}

func (n *daemonNativeTools) memoryList(
	ctx context.Context,
	callerScope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryListInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	page, err := n.memoryHeaderPage(ctx, callerScope, input)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryToolError(req.ToolID, err)
	}
	memories := make([]contract.MemoryEntrySummaryPayload, 0, len(page.Headers))
	for _, header := range page.Headers {
		payload := core.MemorySummaryPayloadFromHeader(header)
		payload.Name = redactNativeMemoryHeaderText(payload.Name)
		payload.Description = redactNativeMemoryHeaderText(payload.Description)
		memories = append(memories, payload)
	}
	response := contract.MemoryListResponse{
		Memories: memories,
		Page: contract.CountedCursorPagePayload{
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
			Total:      page.Total,
			Limit:      page.Limit,
		},
	}
	return structuredResult(response, fmt.Sprintf("%d of %d memories", len(memories), page.Total))
}

func (n *daemonNativeTools) memoryHeaderPage(
	ctx context.Context,
	callerScope toolspkg.Scope,
	input memoryListInput,
) (memorypkg.HeaderListPage, error) {
	scope, err := core.ParseOptionalMemoryScope(input.Scope)
	if err != nil {
		return memorypkg.HeaderListPage{}, err
	}
	selector := memoryToolSelector{
		Scope:     input.Scope,
		Workspace: input.Workspace,
		AgentName: input.AgentName,
		AgentTier: input.AgentTier,
	}
	locations, err := n.memoryListLocations(ctx, callerScope, scope, selector)
	if err != nil {
		return memorypkg.HeaderListPage{}, err
	}

	headers := make([]memcontract.Header, 0)
	workspaceID := ""
	agentName := strings.TrimSpace(input.AgentName)
	agentTier := memcontract.AgentTier(input.AgentTier).Normalize()
	for _, location := range locations {
		items, scanErr := location.Store.CatalogHeaders(ctx, location.Scope)
		if scanErr != nil {
			return memorypkg.HeaderListPage{}, scanErr
		}
		for index := range items {
			items[index].Scope = location.Scope
			items[index].WorkspaceID = location.WorkspaceID
			if location.Scope == memcontract.ScopeAgent {
				items[index].AgentName = location.AgentName
				items[index].AgentTier = location.AgentTier
			}
		}
		if location.WorkspaceID != "" {
			workspaceID = location.WorkspaceID
		}
		if location.AgentName != "" {
			agentName = location.AgentName
			agentTier = location.AgentTier
		}
		headers = append(headers, items...)
	}

	page, err := memorypkg.BuildHeaderListPage(headers, memorypkg.HeaderListQuery{
		Scope:         scope,
		WorkspaceID:   workspaceID,
		AgentName:     agentName,
		AgentTier:     agentTier,
		Type:          memcontract.Type(input.Type),
		IncludeSystem: input.IncludeSystem,
		Sort:          input.Sort,
		Cursor:        input.Cursor,
		Limit:         input.Limit,
	})
	if err != nil {
		return memorypkg.HeaderListPage{}, core.NewMemoryValidationError(err)
	}
	return page, nil
}

func (n *daemonNativeTools) memoryListLocations(
	ctx context.Context,
	callerScope toolspkg.Scope,
	scope memcontract.Scope,
	selector memoryToolSelector,
) ([]memoryToolLocation, error) {
	locations := []memoryToolLocation{{Store: n.deps.MemoryStore, Scope: memcontract.ScopeGlobal}}
	switch scope {
	case memcontract.ScopeGlobal:
		return locations[:1], nil
	case memcontract.ScopeWorkspace, memcontract.ScopeAgent:
		location, err := n.memoryStoreFor(ctx, callerScope, toolspkg.ToolIDMemoryList, selector, scope)
		if err != nil {
			return nil, err
		}
		return []memoryToolLocation{location}, nil
	}
	if strings.TrimSpace(firstNonEmpty(selector.Workspace, callerScope.WorkspaceID)) != "" {
		workspaceSelector := selector
		workspaceSelector.Scope = string(memcontract.ScopeWorkspace)
		location, err := n.memoryStoreFor(
			ctx,
			callerScope,
			toolspkg.ToolIDMemoryList,
			workspaceSelector,
			memcontract.ScopeWorkspace,
		)
		if err != nil {
			return nil, err
		}
		locations = append(locations, location)
	}
	if strings.TrimSpace(firstNonEmpty(selector.AgentName, callerScope.AgentName)) != "" {
		agentSelector := selector
		agentSelector.Scope = string(memcontract.ScopeAgent)
		location, err := n.memoryStoreFor(
			ctx,
			callerScope,
			toolspkg.ToolIDMemoryList,
			agentSelector,
			memcontract.ScopeAgent,
		)
		if err != nil {
			return nil, err
		}
		locations = append(locations, location)
	}
	return locations, nil
}

func redactNativeMemoryHeaderText(value string) string {
	return taskpkg.RedactClaimTokens(strings.TrimSpace(value))
}
