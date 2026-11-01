package core

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/agentmemory"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/gin-gonic/gin"
)

// SearchAgentMemorySessions checks access before reading any transcript.
func SearchAgentMemorySessions(
	ctx context.Context,
	sessions any,
	policy workspaceaccess.Policy,
	actor workspaceaccess.ActorRef,
	profileID, query string,
) ([]agentmemory.SearchResult, error) {
	pager, ok := sessions.(SessionPageManager)
	if !ok {
		return nil, errors.New("agent memory session catalog unavailable")
	}
	reader, ok := sessions.(SessionNavigationReader)
	if !ok {
		return nil, errors.New("agent memory session navigation unavailable")
	}
	search, err := (transcript.SearchQuery{Query: query}).Normalize()
	if err != nil {
		return nil, err
	}
	q := session.ListQuery{
		ReadScope:     store.ReadScope{ProfileID: profileID},
		AllWorkspaces: true,
		Archive:       store.SessionArchiveInclude,
		Later:         store.SessionLaterInclude,
		SkipTotal:     true,
	}
	results := []agentmemory.SearchResult{}
	for {
		page, err := pager.ListPage(ctx, q)
		if err != nil {
			return nil, err
		}
		for _, info := range page.Sessions {
			if info.ProfileID != profileID {
				continue
			}
			allowed, err := authorizeMemoryTranscript(ctx, policy, actor, profileID, info.WorkspaceID)
			if err != nil {
				return nil, err
			}
			if !allowed {
				continue
			}
			found, err := reader.TranscriptSearch(ctx, info.ID, search)
			if err != nil {
				return nil, err
			}
			for _, match := range found.Matches {
				results = append(
					results,
					agentmemory.SearchResult{
						Source:    "session",
						SessionID: info.ID,
						Text:      redact.String(match.Snippet),
						Score:     1,
					},
				)
			}
		}
		if !page.HasMore {
			return results, nil
		}
		if page.NextCursor == "" || page.NextCursor == q.Cursor {
			return nil, errors.New("session catalog did not advance")
		}
		q.Cursor = page.NextCursor
	}
}

func (h *BaseHandlers) searchAgentMemory(
	c *gin.Context,
	key agentmemory.Key,
	req contract.AgentMemorySearchRequest,
) ([]agentmemory.SearchResult, error) {
	results, err := h.AgentMemory.Search(key, req.Query)
	if err != nil || !req.IncludeSessions {
		return results, err
	}
	actor := workspaceaccess.ActorRef{Kind: workspaceaccess.ActorHuman, Operator: true}
	credentials := agentCallerCredentialsFromRequest(c)
	if hasAgentCallerIdentityCredentials(credentials) {
		caller, err := h.resolveAgentCallerForWorkspace(c.Request.Context(), credentials, "agent.memory.search", "")
		if err != nil {
			return results, err
		}
		actor = workspaceAccessRequestForCaller(caller, "").Actor
	}
	matches, err := SearchAgentMemorySessions(
		c.Request.Context(),
		h.Sessions,
		h.WorkspaceAccess,
		actor,
		key.ProfileID,
		req.Query,
	)
	return append(results, matches...), err
}

func authorizeMemoryTranscript(
	ctx context.Context,
	policy workspaceaccess.Policy,
	actor workspaceaccess.ActorRef,
	profileID, workspaceID string,
) (bool, error) {
	if actor.Operator {
		return true, nil
	}
	if policy == nil {
		return false, errors.New("workspace access policy unavailable")
	}
	if workspaceID == "" {
		workspaceID = workspaceaccess.UserSelector
	}
	decision, err := policy.Authorize(ctx, workspaceaccess.Request{
		Actor:    actor,
		Target:   workspaceaccess.ScopeTarget(workspaceID, profileID),
		ReadOnly: true,
		Seam:     workspaceaccess.SeamCatalog,
	})
	return decision.Allowed, err
}
