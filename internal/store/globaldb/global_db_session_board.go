package globaldb

import (
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// sessionCatalogBoardColumnSQL mirrors session.BoardColumnFor; keeping the badge
// argument explicit lets the owning equivalence suite exercise every badge.
func sessionCatalogBoardColumnSQL(badge, prState string) string {
	return `(CASE WHEN ` + badge + ` IN ('needs-attention','waiting-for-auth','waiting-for-input','failed')
 THEN 'needs_you' WHEN ` + badge + ` IN ('running','hung','unhealthy')
 THEN 'working' WHEN ` + badge + ` = 'waiting' THEN 'waiting' WHEN ` + prState + ` = 'open' THEN 'in_review' ELSE 'done' END)`
}

var sessionCatalogBoardColumnExpression = `(CASE WHEN ` + sessionCatalogBadgeExpression + ` IN ('needs-attention','waiting-for-auth','waiting-for-input','failed') THEN 'needs_you' WHEN board_pending_input = 1 THEN 'working' WHEN board_link_open = 1 AND ` + sessionCatalogBadgeExpression + ` IN ('idle','done','stopped') THEN 'in_review' ELSE ` + sessionCatalogBoardColumnSQL(sessionCatalogBadgeExpression, "board_pr_state") + ` END)`

// The derived relation preserves the existing unqualified session filter/sort
// expressions while joining display facts with an explicit current repository key.
func sessionCatalogBoardSource(
	repositoryKeys map[string]string,
	defaultBranches map[string][]string,
) (string, []any, error) {
	if len(repositoryKeys) == 0 {
		return "(SELECT sessions.*, NULL AS board_pr_state, 0 AS board_link_open, EXISTS (SELECT 1 FROM session_input_queue q WHERE q.session_id = sessions.id AND q.status IN ('queued','dispatching')) AS board_pending_input FROM sessions) AS sessions", nil, nil
	}
	encoded, err := json.Marshal(repositoryKeys)
	if err != nil {
		return "", nil, fmt.Errorf("store: encode session repository keys: %w", err)
	}
	defaults, err := json.Marshal(defaultBranches)
	if err != nil {
		return "", nil, fmt.Errorf("store: encode default branches: %w", err)
	}
	return `(SELECT sessions.*, board_pr.pr_state AS board_pr_state,
 EXISTS (SELECT 1 FROM session_input_queue q WHERE q.session_id = sessions.id AND q.status IN ('queued','dispatching')) AS board_pending_input,
 EXISTS (SELECT 1 FROM session_work_links l JOIN forge_pull_requests f
 ON f.workspace_id = l.workspace_id AND f.repo_key = l.repo_key AND f.branch = l.branch
 WHERE l.session_id = sessions.id AND l.workspace_id = sessions.workspace_id
 AND l.unlinked = 0 AND f.pr_state = 'open' AND (l.pr_number IS NULL OR l.pr_number = f.pr_number) AND l.repo_key = json_extract(?, '$.' || json_quote(sessions.workspace_id))) AS board_link_open
 FROM sessions
 LEFT JOIN worktrees AS board_worktree ON board_worktree.id = sessions.worktree_id
 AND board_worktree.workspace_id = sessions.workspace_id
 LEFT JOIN worktree_status AS board_status ON board_status.worktree_id = board_worktree.id
 LEFT JOIN forge_pull_requests AS board_pr ON board_pr.workspace_id = sessions.workspace_id
 AND board_pr.repo_key = json_extract(?, '$.' || json_quote(sessions.workspace_id))
 AND board_pr.branch = CASE WHEN COALESCE(sessions.worktree_id, '') <> ''
 THEN CASE WHEN board_worktree.state IN ('pending','ready')
 THEN CASE WHEN board_status.detached = 1 THEN NULL
 ELSE NULLIF(COALESCE(board_status.branch, board_worktree.branch), '') END END
 ELSE NULLIF(sessions.branch, '') END
 AND NOT EXISTS (SELECT 1 FROM json_each(?, '$.' || json_quote(sessions.workspace_id)) AS default_branch
 WHERE default_branch.value = board_pr.branch)
) AS sessions`, []any{string(encoded), string(encoded), string(defaults)}, nil
}

func sessionCatalogPageSource(query store.SessionCatalogPageQuery) (string, []any, error) {
	if len(query.Columns) == 0 {
		return "sessions", nil, nil
	}
	return sessionCatalogBoardSource(query.RepositoryKeys, query.DefaultBranches)
}
