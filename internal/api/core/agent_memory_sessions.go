package core

import (
	"context"
	"errors"
	"github.com/compozy/compozy/internal/agentmemory"
	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

// SearchAgentMemorySessions checks access before reading any transcript.
func SearchAgentMemorySessions(ctx context.Context, sessions any, policy workspaceaccess.Policy, actor workspaceaccess.ActorRef, profileID, query string) ([]agentmemory.SearchResult, error) {
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
	q := session.ListQuery{ReadScope: store.ReadScope{ProfileID: profileID}, AllWorkspaces: true, Archive: store.SessionArchiveInclude, Later: store.SessionLaterInclude, SkipTotal: true}
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
			if !actor.Operator {
				if policy == nil {
					return nil, errors.New("workspace access policy unavailable")
				}
				target := info.WorkspaceID
				if target == "" {
					target = workspaceaccess.UserSelector
				}
				decision, err := policy.Authorize(ctx, workspaceaccess.Request{Actor: actor, Target: workspaceaccess.ScopeTarget(target, profileID), ReadOnly: true, Seam: workspaceaccess.SeamCatalog})
				if err != nil {
					return nil, err
				}
				if !decision.Allowed {
					continue
				}
			}
			found, err := reader.TranscriptSearch(ctx, info.ID, search)
			if err != nil {
				return nil, err
			}
			for _, match := range found.Matches {
				results = append(results, agentmemory.SearchResult{Source: "session", SessionID: info.ID, Text: redact.String(match.Snippet), Score: 1})
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
