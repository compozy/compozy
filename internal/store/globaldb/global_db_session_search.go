package globaldb

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

func appendSessionCatalogSearch(
	where []string,
	args []any,
	query store.SessionCatalogPageQuery,
) ([]string, []any, error) {
	if search := strings.ToLower(strings.TrimSpace(query.Search)); search != "" {
		repos, err := json.Marshal(query.RepositoryKeys)
		if err != nil {
			return nil, nil, fmt.Errorf("store: encode search repositories: %w", err)
		}
		text := strings.TrimSpace(query.Search)
		branchTerm := store.LowerSessionCatalogSearchText(text)
		number, numberArgs := sessionCatalogNumberMatch("COALESCE(search_link.pr_number, search_pr.pr_number)", text)
		linked := ` OR EXISTS (SELECT 1 FROM session_work_links AS search_link
 LEFT JOIN forge_pull_requests AS search_pr ON search_pr.workspace_id = search_link.workspace_id
 AND search_pr.repo_key = search_link.repo_key AND search_pr.branch = search_link.branch
 AND search_pr.repo_key = json_extract(?, '$.' || json_quote(search_link.workspace_id))
 WHERE search_link.session_id = sessions.id AND search_link.workspace_id = sessions.workspace_id
 AND search_link.unlinked = 0 AND ((instr(compozy_unicode_lower(search_link.branch), ?) > 0
 AND search_link.repo_key = json_extract(?, '$.' || json_quote(search_link.workspace_id)))` + number + `))`
		// A number the link stored still names that PR after a move; a branch name no longer does.
		linkedArgs := append([]any{string(repos), branchTerm, string(repos)}, numberArgs...)
		own, ownArgs, err := sessionCatalogOwnWorkSearch(query)
		if err != nil {
			return nil, nil, err
		}
		linked += own
		if query.SearchFields == "title_agent" {
			search = store.LowerSessionCatalogSearchText(strings.TrimSpace(query.Search))
			where = append(
				where,
				"(instr(compozy_unicode_lower("+sessionCatalogDisplayTitleExpression+"), ?) > 0 OR "+
					"instr(compozy_unicode_lower(agent_name), ?) > 0"+linked+")",
			)
			args = append(args, search, search)
		} else {
			where = append(where, `(instr(lower(id), ?) > 0 OR
			instr(lower(COALESCE(name, '')), ?) > 0 OR
			instr(lower(agent_name), ?) > 0 OR
			instr(lower(provider), ?) > 0`+linked+`)`)
			for range 4 {
				args = append(args, search)
			}
		}
		args = append(append(args, linkedArgs...), ownArgs...)
	}
	return where, args, nil
}

// sessionCatalogNumberMatch skips an empty term: instr matches "" everywhere, so `#` alone would too.
func sessionCatalogNumberMatch(column, text string) (string, []any) {
	digits := strings.TrimPrefix(text, "#")
	if digits == "" {
		return "", nil
	}
	return " OR instr(CAST(" + column + " AS TEXT), ?) > 0", []any{digits}
}

func sessionCatalogOwnWorkSearch(query store.SessionCatalogPageQuery) (string, []any, error) {
	repos, err := json.Marshal(query.RepositoryKeys)
	if err != nil {
		return "", nil, fmt.Errorf("store: encode search repositories: %w", err)
	}
	defaults, err := json.Marshal(query.DefaultBranches)
	if err != nil {
		return "", nil, fmt.Errorf("store: encode search default branches: %w", err)
	}
	text := strings.TrimSpace(query.Search)
	number, numberArgs := sessionCatalogNumberMatch("pr.pr_number", text)
	return ` OR EXISTS (SELECT 1 FROM (
 SELECT own.id, own.workspace_id, own.worktree_id,
 CASE WHEN COALESCE(own.worktree_id,'') <> '' THEN
 CASE WHEN w.state IN ('pending','ready') THEN
 CASE WHEN ws.detached = 1 THEN '' ELSE COALESCE(ws.branch,w.branch,'') END ELSE '' END
 ELSE own.branch END AS branch
 FROM sessions AS own
 LEFT JOIN worktrees w ON w.id = own.worktree_id AND w.workspace_id = own.workspace_id
 LEFT JOIN worktree_status ws ON ws.worktree_id = w.id
 WHERE own.id = sessions.id
 ) AS own_link
 LEFT JOIN forge_pull_requests pr ON pr.workspace_id = own_link.workspace_id
 AND pr.branch = own_link.branch AND pr.repo_key = json_extract(?, '$.' || json_quote(own_link.workspace_id))
 WHERE own_link.branch <> ''
 AND (COALESCE(own_link.worktree_id,'') <> '' OR NOT EXISTS (
 SELECT 1 FROM json_each(?, '$.' || json_quote(own_link.workspace_id)) d WHERE d.value = own_link.branch))
 AND NOT EXISTS (SELECT 1 FROM session_work_links l WHERE l.session_id = own_link.id
 AND l.branch = own_link.branch AND l.repo_key = json_extract(?, '$.' || json_quote(own_link.workspace_id)))
 AND (instr(compozy_unicode_lower(own_link.branch),?) > 0` + number + `))`,
		append(
			[]any{string(repos), string(defaults), string(repos), store.LowerSessionCatalogSearchText(text)},
			numberArgs...), nil
}
