package daemon

import (
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
)

type compactSessionListRecord struct {
	compactSessionRecord
	FolderID string                         `json:"folder_id,omitempty"`
	Health   *contract.SessionHealthPayload `json:"health,omitzero"`
}

var sessionListViews = resultViews[contract.SessionCatalogResponse]{
	Compact: func(p contract.SessionCatalogResponse) any {
		rows := make([]compactSessionListRecord, 0, len(p.Sessions))
		for _, item := range p.Sessions {
			rows = append(rows, compactSessionListRecord{
				compactSessionRecord: compactSession(&item), FolderID: item.FolderID, Health: item.Health,
			})
		}
		return struct {
			Sessions []compactSessionListRecord         `json:"sessions"`
			Page     contract.SessionCatalogPagePayload `json:"page"`
		}{Sessions: rows, Page: p.Page}
	},
	Preview: func(p contract.SessionCatalogResponse) string {
		return fmt.Sprintf("%d of %d sessions", len(p.Sessions), *p.Page.Total)
	},
}

type sessionDescribeResult struct {
	Session     contract.SessionPayload `json:"session"`
	Events      []any                   `json:"events"`
	EventsPage  nativeSessionPage       `json:"events_page"`
	History     []any                   `json:"history"`
	HistoryPage nativeSessionPage       `json:"history_page"`
}

var sessionDescribeViews = resultViews[sessionDescribeResult]{
	Compact: func(p sessionDescribeResult) any {
		return map[string]any{
			nativeToolsSessionKey: compactSession(&p.Session),
			nativeToolsEventsKey:  p.Events, nativeToolsEventsPageKey: p.EventsPage,
			nativeToolsHistoryKey: p.History, nativeToolsHistoryPageKey: p.HistoryPage,
		}
	},
	Full: func(p sessionDescribeResult) any {
		return map[string]any{
			nativeToolsSessionKey: p.Session,
			nativeToolsEventsKey:  p.Events, nativeToolsEventsPageKey: p.EventsPage,
			nativeToolsHistoryKey: p.History, nativeToolsHistoryPageKey: p.HistoryPage,
		}
	},
	Preview: func(p sessionDescribeResult) string { return p.Session.ID },
}

var sessionDeriveViews = resultViews[contract.SessionDeriveResponse]{
	Compact: func(p contract.SessionDeriveResponse) any {
		result := map[string]any{"derived": p.Derived}
		if p.Session != nil {
			result[nativeToolsSessionKey] = compactSession(p.Session)
		}
		return result
	},
	Preview: func(p contract.SessionDeriveResponse) string { return p.Derived.ChildSessionID },
}

var sessionRewindViews = resultViews[contract.SessionConversationRewindResponse]{
	Compact: func(p contract.SessionConversationRewindResponse) any {
		return map[string]any{nativeToolsSessionKey: compactSession(&p.Session), "rewind": p.Rewind}
	},
	Preview: func(p contract.SessionConversationRewindResponse) string { return p.Rewind.TargetMessageID },
}

type sessionMoveToProfileResult struct {
	Session    contract.SessionPayload `json:"session"`
	Original   contract.SessionPayload `json:"original"`
	WorktreeID string                  `json:"worktree_id,omitempty"`
	Branch     string                  `json:"branch,omitempty"`
}

var sessionMoveToProfileViews = resultViews[sessionMoveToProfileResult]{
	Compact: func(p sessionMoveToProfileResult) any {
		result := map[string]any{
			nativeToolsSessionKey: compactSession(&p.Session), "original": compactSession(&p.Original),
		}
		if p.WorktreeID != "" {
			result["worktree_id"], result["branch"] = p.WorktreeID, p.Branch
		}
		return result
	},
	Full: func(p sessionMoveToProfileResult) any {
		result := map[string]any{nativeToolsSessionKey: p.Session, "original": p.Original}
		if p.WorktreeID != "" {
			result["worktree_id"], result["branch"] = p.WorktreeID, p.Branch
		}
		return result
	},
	Preview: func(p sessionMoveToProfileResult) string { return p.Session.ID },
}
