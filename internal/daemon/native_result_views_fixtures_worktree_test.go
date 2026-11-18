package daemon

import (
	"encoding/json"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func nativeResultViewFixturesWorktree(t *testing.T) []nativeResultViewFixture {
	t.Helper()
	var rows []nativeResultViewFixture
	{
		var payload contract.WorktreesResponse
		if err := json.Unmarshal(
			[]byte(
				`{"worktrees":[{"id":"wt-a","workspace_id":"ws-a","name":"sample","branch":"feat/sample","path":"/tmp/sample","state":"failed","pending_phase":"setup","origin":"managed","setup_state":"failed","setup_error":"setup failed","create_error":"create failed","created_branch":true,"dirty":true,"ahead":1,"behind":2,"agent_activity":"active","run_id":"run-a","profile_name":"Default","created_at":"2026-11-10T00:00:00Z","updated_at":"2026-11-10T00:00:00Z","profile_id":"profile-a","profile_archived":true}],"discovered":[{"name":"extra","branch":"extra","path":"/tmp/extra","stale":true,"unavailable":true,"reason":"missing"}],"repo":{"git_backed":true,"git_available":false,"diagnostic":"git unavailable"}}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__worktree_list",
				Name:   "failure",
				MustKeep: []string{
					"worktrees.0.id", "worktrees.0.profile_id", "worktrees.0.profile_archived",
					"worktrees.0.path",
					"worktrees.0.setup_error",
					"discovered.0.reason",
					"repo.diagnostic",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.WorktreesResponse]{Compact: compactWorktrees}).result(view, payload)
				},
			},
		)
	}
	{
		var payload contract.WorktreesResponse
		if err := json.Unmarshal(
			[]byte(
				`{"worktrees":[],"discovered":[],"repo":{"git_backed":true,"git_available":false,"diagnostic":"git unavailable"}}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__worktree_list",
				Name:     "empty",
				MustKeep: []string{"worktrees", "discovered", "repo.diagnostic"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[contract.WorktreesResponse]{Compact: compactWorktrees}).result(view, payload)
				},
			},
		)
	}
	return rows
}
