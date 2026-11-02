package daemon

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/workspaceaccess"

	"github.com/compozy/compozy/internal/agentmemory"
	compozyconfig "github.com/compozy/compozy/internal/config"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type nativeAgentMemoryInput struct {
	Text            string `json:"text"`
	Until           string `json:"until"`
	Match           string `json:"match"`
	Query           string `json:"query"`
	IncludeSessions bool   `json:"include_sessions"`
}

const agentMemoryDigestKey = "digest"

func (n *daemonNativeTools) agentMemoryToolBindings() map[toolspkg.ToolID]nativeToolBinding {
	available := n.dependencyAvailability(
		func() bool { return n.deps.AgentMemory != nil && n.deps.AgentCatalog != nil && n.deps.Sessions != nil },
	)
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDAgentMemoryAdd:     {call: n.agentMemoryCall, availability: available},
		toolspkg.ToolIDAgentMemoryReplace: {call: n.agentMemoryCall, availability: available},
		toolspkg.ToolIDAgentMemoryRemove:  {call: n.agentMemoryCall, availability: available},
		toolspkg.ToolIDAgentMemorySearch:  {call: n.agentMemoryCall, availability: available},
	}
}

func (n *daemonNativeTools) agentMemoryCaller(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
) (agentmemory.Key, string, error) {
	if scope.SessionID == "" {
		return agentmemory.Key{}, "", nativeRequiredInputError(id, "session_id")
	}
	info, err := n.deps.Sessions.Status(ctx, scope.SessionID)
	if err != nil {
		return agentmemory.Key{}, "", err
	}
	if info == nil || info.ProfileID == "" || info.ProfileID != scope.ProfileID || info.AgentName != scope.AgentName ||
		info.WorkspaceID != scope.WorkspaceID {
		return agentmemory.Key{}, "", nativeWorkspaceAccessDeniedError(id)
	}
	if n.deps.AgentInstances == nil {
		return agentmemory.Key{}, "", nativeWorkspaceAccessDeniedError(id)
	}
	instances := n.deps.AgentInstances()
	if instances == nil {
		return agentmemory.Key{}, "", nativeWorkspaceAccessDeniedError(id)
	}
	if err := instances.Authority.AuthorizeMemory(ctx, info.ID, info.AgentName, info.ProfileID); err != nil {
		return agentmemory.Key{}, "", nativeWorkspaceAccessDeniedError(id)
	}
	resolved, err := n.nativeSkillWorkspace(ctx, info.ProfileID, info.WorkspaceID)
	if err != nil {
		return agentmemory.Key{}, "", err
	}
	entries, err := n.deps.AgentCatalog.ListAgentsForWorkspace(ctx, resolved)
	if err != nil {
		return agentmemory.Key{}, "", err
	}
	for i := range entries {
		entry := &entries[i]
		if entry.Def.Name != info.AgentName {
			continue
		}
		if entry.Def.EffectiveKind() != compozyconfig.AgentKindStaysOn {
			return agentmemory.Key{}, "", nativeInputError(id, errors.New("agent_not_stays_on"))
		}
		if entry.MemoryOrigin == "" {
			return agentmemory.Key{}, "", nativeUnavailableError(id, "agent memory identity unavailable")
		}
		source := resolved.Name
		if source == "" {
			source = daemonGlobalLabel
		}
		return agentmemory.Key{
			ProfileID: info.ProfileID,
			Origin:    entry.MemoryOrigin,
			Agent:     info.AgentName,
		}, source, nil
	}
	return agentmemory.Key{}, "", nativeUnavailableError(id, "agent definition unavailable")
}

func (n *daemonNativeTools) agentMemoryCall(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeAgentMemoryInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	key, source, err := n.agentMemoryCaller(ctx, scope, req.ToolID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	var result any
	switch req.ToolID {
	case toolspkg.ToolIDAgentMemoryAdd:
		var doc agentmemory.Document
		doc, err = n.deps.AgentMemory.Add(key, time.Now(), source, input.Text, input.Until)
		result = map[string]any{agentMemoryDigestKey: doc.Digest, "budget": doc.Budget}
	case toolspkg.ToolIDAgentMemoryReplace:
		var doc agentmemory.Document
		doc, err = n.deps.AgentMemory.Replace(key, input.Match, input.Text)
		result = map[string]any{agentMemoryDigestKey: doc.Digest}
	case toolspkg.ToolIDAgentMemoryRemove:
		var doc agentmemory.Document
		var removed int
		doc, removed, err = n.deps.AgentMemory.Remove(key, input.Match)
		result = map[string]any{agentMemoryDigestKey: doc.Digest, "removed": removed}
	case toolspkg.ToolIDAgentMemorySearch:

		var results []agentmemory.SearchResult
		results, err = n.deps.AgentMemory.Search(key, input.Query)
		if err == nil && input.IncludeSessions {
			var matches []agentmemory.SearchResult
			matches, err = core.SearchAgentMemorySessions(
				ctx,
				n.deps.Sessions,
				n.deps.WorkspaceAccess,
				workspaceaccess.ActorRef{
					Kind:        workspaceaccess.ActorAgentSession,
					SessionID:   scope.SessionID,
					WorkspaceID: scope.WorkspaceID,
					AgentName:   scope.AgentName,
				},
				key.ProfileID,
				input.Query,
			)
			results = append(results, matches...)
		}
		result = map[string]any{"results": results}
	default:
		return toolspkg.ToolResult{}, nativeInputError(req.ToolID, errors.New("unknown agent memory operation"))
	}
	if err != nil {
		return toolspkg.ToolResult{}, nativeInputError(req.ToolID, err)
	}
	return structuredResult(result, "Agent memory operation completed.")
}
