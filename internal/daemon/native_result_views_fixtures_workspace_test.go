package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	apitest "github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/modelcatalog"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/worktree"
)

func nativeResultViewFixturesWorkspace(t *testing.T) []nativeResultViewFixture {
	t.Helper()
	var rows []nativeResultViewFixture
	{
		var payload workspaceDescribeResult
		if err := json.Unmarshal(
			[]byte(
				`{"workspace":{"id":"ws-a","root_dir":"/tmp/ws","name":"Workspace","add_dirs":[],"created_at":"2026-11-10T00:00:00Z","updated_at":"2026-11-10T00:00:00Z"},"sessions":[{"id":"sess-a","profile_id":"prof_default","agent_name":"general","state":"failed","badge":"failed","board_column":"needs_you","runtime":{"status":"failed","selection_revision":1,"failure":"runtime failed"},"failure":{"kind":"runtime"},"stop_reason":"failed"}],"agents":[{"name":"agent-a","provider":"claude","origin":"user","prompt":"large definition","permissions":"*","diagnostics":[{"message":"Invalid definition"}]}],"skills":[{"name":"skill-a","source":"workspace","dir":"/tmp/skill-a"}],"providers":[{"name":"claude","display_name":"Claude","harness":"acp","env_policy":"inherit"}]}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID: "compozy__workspace_describe",
				Name:   "failure",
				MustKeep: []string{
					"workspace.id", "sessions.0.failure.kind", "sessions.0.runtime.failure",
					"agents.0.name",
					"agents.0.diagnostics",
					"skills.0.name",
					"providers.0.name",
				},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[workspaceDescribeResult]{Compact: compactWorkspaceDescribe}).result(
						view,
						payload,
					)
				},
			},
		)
	}
	{
		var payload workspaceDescribeResult
		if err := json.Unmarshal(
			[]byte(
				`{"workspace":{"id":"ws-a","root_dir":"/tmp/ws","name":"Workspace","add_dirs":[],"created_at":"2026-11-10T00:00:00Z","updated_at":"2026-11-10T00:00:00Z"},"sessions":[],"agents":[],"skills":[],"providers":[]}`,
			),
			&payload,
		); err != nil {
			t.Fatal(err)
		}
		rows = append(
			rows,
			nativeResultViewFixture{
				ToolID:   "compozy__workspace_describe",
				Name:     "empty",
				MustKeep: []string{"workspace.id", "sessions", "agents", "skills", "providers"},
				Result: func(view toolspkg.ResultView) (toolspkg.ToolResult, error) {
					return (resultViews[workspaceDescribeResult]{Compact: compactWorkspaceDescribe}).result(
						view,
						payload,
					)
				},
			},
		)
	}
	return rows
}

func TestNativePlatformFailureResultViews(t *testing.T) {
	t.Parallel()
	workspaceService := apitest.StubWorkspaceService{
		SetArchivedFn: func(context.Context, string, bool) (workspacepkg.Workspace, error) {
			return workspacepkg.Workspace{}, &workspacepkg.BusyError{
				Blockers: []workspacepkg.ArchiveBlocker{{Kind: "session", ID: "sess-a", Name: "Active"}},
			}
		},
		ResolveFn: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
			return workspacepkg.ResolvedWorkspace{
				ID:          "ws-a",
				WorkspaceID: "ws-a",
			}, nil
		},
	}
	cases := []struct {
		name    string
		id      toolspkg.ToolID
		input   string
		deps    daemonNativeToolsDeps
		field   string
		want    string
		wantErr bool
	}{
		{
			name:    "Should preserve workspace archive blockers",
			id:      toolspkg.ToolIDWorkspaceArchive,
			input:   `{"workspace":"ws-a"}`,
			deps:    daemonNativeToolsDeps{Workspaces: workspaceService},
			field:   "blockers",
			want:    `{"blockers":[{"kind":"session","id":"sess-a","name":"Active"}]}`,
			wantErr: true,
		},
		{
			name:  "Should preserve worktree removal refusal",
			id:    toolspkg.ToolIDWorktreeRemove,
			input: `{"workspace":"ws-a","ref":"wt-a"}`,
			deps: daemonNativeToolsDeps{Workspaces: workspaceService, Worktrees: &nativeWorktreeServiceStub{
				resolve: func(context.Context, string, string) (*worktree.Worktree, error) {
					return &worktree.Worktree{ID: "wt-a"}, nil
				},
				remove: func(context.Context, string, string, bool) (*worktree.RemovalRefusal, error) {
					return &worktree.RemovalRefusal{
						Code: "worktree_dirty",
						Risk: worktree.RemovalRisk{ChangedFiles: 2},
					}, worktree.ErrDirtyRequiresForce
				},
			}},
			field:   "risk",
			want:    `{"code":"worktree_dirty","risk":{"changed_files":2}}`,
			wantErr: true,
		},
		{
			name:  "Should preserve provider refresh partial failure",
			id:    toolspkg.ToolIDProviderModelsRefresh,
			input: `{"provider_id":"claude"}`,
			deps: daemonNativeToolsDeps{
				ModelCatalog: &nativeModelCatalogService{
					statuses:   []modelcatalog.SourceStatus{{ProviderID: "claude", SourceID: "config"}},
					refreshErr: errors.New("refresh failed"),
				},
			},
			field: "error",
			want:  `{"error":"refresh failed","sources":[{"provider_id":"claude","source_id":"config"}]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			n := &daemonNativeTools{deps: &tc.deps}
			var previous []byte
			for _, view := range []toolspkg.ResultView{toolspkg.ResultViewCompact, toolspkg.ResultViewFull} {
				req := toolspkg.CallRequest{ToolID: tc.id, Input: json.RawMessage(tc.input), View: view}
				var result toolspkg.ToolResult
				var err error
				switch tc.id {
				case toolspkg.ToolIDWorkspaceArchive:
					result, err = n.workspaceArchive(t.Context(), toolspkg.Scope{}, req)
				case toolspkg.ToolIDWorktreeRemove:
					result, err = n.worktreeRemove(t.Context(), toolspkg.Scope{}, req)
				default:
					result, err = n.providerModelsRefresh(t.Context(), toolspkg.Scope{}, req)
				}
				if (err != nil) != tc.wantErr {
					t.Fatalf("error = %v, want error %t", err, tc.wantErr)
				}
				if err != nil {
					failure, ok := errors.AsType[*toolspkg.ToolError](err)
					if !ok || failure.PartialResult == nil {
						t.Fatalf("missing partial result: %v", err)
					}
					result = *failure.PartialResult
				}
				var body map[string]json.RawMessage
				if err := json.Unmarshal(result.Structured, &body); err != nil {
					t.Fatal(err)
				}
				if len(body[tc.field]) == 0 {
					t.Fatalf("missing %s in %s", tc.field, result.Structured)
				}
				var want, actual any
				if err := json.Unmarshal([]byte(tc.want), &want); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(result.Structured, &actual); err != nil {
					t.Fatal(err)
				}
				requireNativeResultProjection(t, "failure", want, actual)
				if previous != nil && !bytes.Equal(previous, result.Structured) {
					t.Fatalf("partial result changed: %s vs %s", previous, result.Structured)
				}
				previous = result.Structured
			}
		})
	}
}
