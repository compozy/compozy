package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/tools"
	"github.com/spf13/cobra"
)

type completionStubClient struct {
	*stubClient
	listSessionFoldersFn func(context.Context, string) (contract.SessionFoldersResponse, error)
	workspaceLayoutFn    func(context.Context) (contract.WorkspaceLayoutResponse, error)
	listComputersFn      func(context.Context) (contract.ComputerListResponse, error)
}

func (c *completionStubClient) ListSessionFolders(
	ctx context.Context,
	workspace string,
) (contract.SessionFoldersResponse, error) {
	if c.listSessionFoldersFn != nil {
		return c.listSessionFoldersFn(ctx, workspace)
	}
	return contract.SessionFoldersResponse{}, errors.New("unexpected ListSessionFolders call")
}

func (c *completionStubClient) CreateSessionFolder(
	context.Context,
	string,
	string,
) (contract.SessionFolderResponse, error) {
	return contract.SessionFolderResponse{}, errors.New("unexpected CreateSessionFolder call")
}

func (c *completionStubClient) RenameSessionFolder(
	context.Context,
	string,
	string,
	string,
) (contract.SessionFolderResponse, error) {
	return contract.SessionFolderResponse{}, errors.New("unexpected RenameSessionFolder call")
}

func (c *completionStubClient) DeleteSessionFolder(
	context.Context,
	string,
	string,
) (contract.SessionFoldersResponse, error) {
	return contract.SessionFoldersResponse{}, errors.New("unexpected DeleteSessionFolder call")
}

func (c *completionStubClient) ReorderSessionFolders(
	context.Context,
	string,
	[]string,
) (contract.SessionFoldersResponse, error) {
	return contract.SessionFoldersResponse{}, errors.New("unexpected ReorderSessionFolders call")
}

func (c *completionStubClient) MoveSession(context.Context, string, string) (contract.SessionFoldersResponse, error) {
	return contract.SessionFoldersResponse{}, errors.New("unexpected MoveSession call")
}

func (c *completionStubClient) ReorderSessions(
	context.Context,
	string,
	contract.ReorderSessionsRequest,
) (contract.SessionFoldersResponse, error) {
	return contract.SessionFoldersResponse{}, errors.New("unexpected ReorderSessions call")
}

func (c *completionStubClient) WorkspaceLayout(ctx context.Context) (contract.WorkspaceLayoutResponse, error) {
	if c.workspaceLayoutFn != nil {
		return c.workspaceLayoutFn(ctx)
	}
	return contract.WorkspaceLayoutResponse{}, errors.New("unexpected WorkspaceLayout call")
}

func (c *completionStubClient) CreateWorkspaceFolder(
	context.Context,
	string,
) (contract.WorkspaceFolderResponse, error) {
	return contract.WorkspaceFolderResponse{}, errors.New("unexpected CreateWorkspaceFolder call")
}

func (c *completionStubClient) RenameWorkspaceFolder(
	context.Context,
	string,
	string,
) (contract.WorkspaceFolderResponse, error) {
	return contract.WorkspaceFolderResponse{}, errors.New("unexpected RenameWorkspaceFolder call")
}

func (c *completionStubClient) DeleteWorkspaceFolder(
	context.Context,
	string,
) (contract.WorkspaceLayoutResponse, error) {
	return contract.WorkspaceLayoutResponse{}, errors.New("unexpected DeleteWorkspaceFolder call")
}

func (c *completionStubClient) ReorderWorkspaceFolders(
	context.Context,
	[]string,
) (contract.WorkspaceLayoutResponse, error) {
	return contract.WorkspaceLayoutResponse{}, errors.New("unexpected ReorderWorkspaceFolders call")
}

func (c *completionStubClient) MoveWorkspace(
	context.Context,
	contract.MoveWorkspaceRequest,
) (contract.WorkspaceLayoutResponse, error) {
	return contract.WorkspaceLayoutResponse{}, errors.New("unexpected MoveWorkspace call")
}

func (c *completionStubClient) ReorderWorkspaces(
	context.Context,
	contract.ReorderWorkspacesRequest,
) (contract.WorkspaceLayoutResponse, error) {
	return contract.WorkspaceLayoutResponse{}, errors.New("unexpected ReorderWorkspaces call")
}

func (c *completionStubClient) ListComputers(ctx context.Context) (contract.ComputerListResponse, error) {
	if c.listComputersFn != nil {
		return c.listComputersFn(ctx)
	}
	return contract.ComputerListResponse{}, errors.New("unexpected ListComputers call")
}

func TestCompletionSourceMappings(t *testing.T) {
	t.Parallel()

	t.Run("Should fall back to workspace ID for blank, duplicate, whitespace, or path-like names", func(t *testing.T) {
		t.Parallel()

		stub := &completionStubClient{stubClient: &stubClient{
			listWorkspacesFn: func(context.Context) ([]WorkspaceRecord, error) {
				return []WorkspaceRecord{
					{ID: "ws_ok", Name: "checkout-api", RootDir: "/repo/checkout"},
					{ID: "ws_blank", Name: "   ", RootDir: "/repo/blank"},
					{ID: "ws_dup_1", Name: "dup", RootDir: "/repo/dup1"},
					{ID: "ws_dup_2", Name: "dup", RootDir: "/repo/dup2"},
					{ID: "ws_space", Name: "my app", RootDir: "/repo/space"},
					{ID: "ws_path", Name: "/tmp/local", RootDir: "/tmp/local"},
					{ID: "ws_dot", Name: "./rel", RootDir: "/repo/rel"},
				}, nil
			},
		}}
		rt := newCompletionRuntime(newTestDeps(t, stub))
		got := renderCompletionCandidates(rt.complete(&cobra.Command{Use: "test"}, kindWorkspace, "", ""), "")
		want := []string{
			"checkout-api\t/repo/checkout",
			"ws_blank\t/repo/blank",
			"ws_dot\t./rel",
			"ws_dup_1\tdup",
			"ws_dup_2\tdup",
			"ws_path\t/tmp/local",
			"ws_space\tmy app",
		}
		if !slices.Equal(got, want) {
			t.Fatalf("workspaces = %#v, want %#v", got, want)
		}
	})

	t.Run("Should map all entity kinds cleanly", func(t *testing.T) {
		t.Parallel()

		var seenSessionQuery SessionListQuery
		stub := &completionStubClient{
			stubClient: &stubClient{
				listProfilesFn: func(context.Context) ([]contract.Profile, error) {
					return []contract.Profile{{Name: "work", State: "active"}}, nil
				},
				listAgentsFn: func(_ context.Context, q AgentQuery) ([]AgentRecord, error) {
					if q.Workspace != "ws_1" {
						return nil, errors.New("missing workspace")
					}
					return []AgentRecord{{Name: "coder", DisplayName: "Coder"}}, nil
				},
				listProvidersFn: func(context.Context) (contract.ProviderListResponse, error) {
					return contract.ProviderListResponse{
						Providers: []contract.ProviderSummaryPayload{{Name: "claude", DisplayName: "Claude"}},
					}, nil
				},
				listProviderModelsFn: func(_ context.Context, q ProviderModelListQuery) (ProviderModelListRecord, error) {
					if q.ProviderID != "claude" {
						return ProviderModelListRecord{}, errors.New("wrong provider")
					}
					return ProviderModelListRecord{
						Models: []ProviderModelRecord{{ModelID: "claude-sonnet-5", DisplayName: "Claude Sonnet 5"}},
					}, nil
				},
				listSessionPageFn: func(_ context.Context, q SessionListQuery) (SessionListPage, error) {
					seenSessionQuery = q
					return SessionListPage{
						Sessions: []SessionRecord{{ID: "sess-1", Name: "Debug"}},
					}, nil
				},
				listTasksFn: func(_ context.Context, q TaskListQuery) ([]TaskCatalogItemRecord, error) {
					if q.Workspace != "ws_1" {
						return nil, errors.New("missing workspace")
					}
					return []TaskCatalogItemRecord{{ID: "task-1", Title: "Add completions"}}, nil
				},
				listLoopsFn: func(_ context.Context, ws string, _ LoopListQuery) (contract.LoopsResponse, error) {
					if ws != "ws_1" {
						return contract.LoopsResponse{}, errors.New("missing workspace")
					}
					return contract.LoopsResponse{
						Loops: []contract.LoopCatalogEntryPayload{{Name: "spec-loop", Description: "Run specs"}},
					}, nil
				},
				listAutomationJobsFn: func(_ context.Context, q AutomationJobQuery) (AutomationJobListRecord, error) {
					if q.WorkspaceID != "ws_1" {
						return AutomationJobListRecord{}, errors.New("missing workspace")
					}
					return AutomationJobListRecord{Jobs: []JobRecord{{ID: "job-1", Name: "Nightly"}}}, nil
				},
				listWorktreesFn: func(_ context.Context, ws string, _, _ bool) (WorktreeListRecord, error) {
					if ws != "ws_1" {
						return WorktreeListRecord{}, errors.New("missing workspace")
					}
					return WorktreeListRecord{
						Worktrees: []WorktreeRecord{{ID: "wt-1", Name: "feat", Branch: "feat/comp"}},
					}, nil
				},
				listExtensionsFn: func(context.Context) ([]ExtensionRecord, error) {
					return []ExtensionRecord{{Name: "ext-alpha", Description: "Alpha ext"}}, nil
				},
				listVaultSecretsFn: func(context.Context, VaultListQuery) ([]VaultRecord, error) {
					return []VaultRecord{
						{Ref: "vault:providers/claude/api_key", Namespace: "providers", Kind: "token"},
					}, nil
				},
				listToolsetsFn: func(_ context.Context, q ToolQuery) (ToolsetsResponseRecord, error) {
					if q.WorkspaceID != "ws_1" {
						return ToolsetsResponseRecord{}, errors.New("missing workspace")
					}
					return ToolsetsResponseRecord{
						Toolsets: []ToolsetRecord{{ID: tools.ToolsetID("compozy__core"), Status: "available"}},
					}, nil
				},
			},
			listSessionFoldersFn: func(_ context.Context, ws string) (contract.SessionFoldersResponse, error) {
				if ws != "ws_1" {
					return contract.SessionFoldersResponse{}, errors.New("missing workspace")
				}
				return contract.SessionFoldersResponse{
					Folders: []contract.SessionFolderPayload{{ID: "sfold-1", Name: "Sprint"}},
				}, nil
			},
			workspaceLayoutFn: func(context.Context) (contract.WorkspaceLayoutResponse, error) {
				return contract.WorkspaceLayoutResponse{
					Folders: []contract.WorkspaceFolderPayload{{ID: "wfold-1", Name: "Clients"}},
				}, nil
			},
			listComputersFn: func(context.Context) (contract.ComputerListResponse, error) {
				return contract.ComputerListResponse{
					Computers: []contract.Computer{
						{ID: remoteproto.ComputerID("cmp-1"), Name: "macbook", Status: "online"},
					},
				}, nil
			},
		}

		deps := newTestDeps(t, stub)
		deps.getenv = func(k string) string {
			if k == workspaceEnvName {
				return "ws_1"
			}
			return ""
		}
		rt := newCompletionRuntime(deps)
		cmd := &cobra.Command{Use: "test"}

		checks := []struct {
			kind     completionKind
			provider string
			want     []string
		}{
			{kind: kindProfile, want: []string{"work\tactive"}},
			{kind: kindAgent, want: []string{"coder\tCoder"}},
			{kind: kindProvider, want: []string{"claude\tClaude"}},
			{kind: kindModel, provider: "claude", want: []string{"claude-sonnet-5\tClaude Sonnet 5"}},
			{kind: kindSession, want: []string{"sess-1\tDebug"}},
			{kind: kindTask, want: []string{"task-1\tAdd completions"}},
			{kind: kindLoop, want: []string{"spec-loop\tRun specs"}},
			{kind: kindAutomation, want: []string{"job-1\tNightly"}},
			{kind: kindWorktree, want: []string{"wt-1\tfeat (feat/comp)"}},
			{kind: kindSessionFolder, want: []string{"sfold-1\tSprint"}},
			{kind: kindWorkspaceFolder, want: []string{"wfold-1\tClients"}},
			{kind: kindExtension, want: []string{"ext-alpha\tAlpha ext"}},
			{kind: kindComputer, want: []string{"macbook\tonline"}},
			{kind: kindVault, want: []string{"vault:providers/claude/api_key\tproviders (token)"}},
			{kind: kindToolset, want: []string{"compozy__core\tavailable"}},
			{kind: kindOutput, want: []string{
				"human\tHuman-readable output",
				"json\tJSON output",
				"jsonl\tJSON Lines output",
				"toon\tTOON output",
			}},
		}
		for _, tc := range checks {
			got := renderCompletionCandidates(rt.complete(cmd, tc.kind, "", tc.provider), "")
			if !slices.Equal(got, tc.want) {
				t.Fatalf("kind %v = %#v, want %#v", tc.kind, got, tc.want)
			}
		}
		if seenSessionQuery.Limit != completionSessionCap || seenSessionQuery.Sort != session.ListSortLastActivity {
			t.Fatalf(
				"session query = %#v, want Limit=%d Sort=%q",
				seenSessionQuery,
				completionSessionCap,
				session.ListSortLastActivity,
			)
		}
	})

	t.Run("Should return empty for model without provider without calling daemon", func(t *testing.T) {
		t.Parallel()

		called := false
		stub := &stubClient{
			listProviderModelsFn: func(context.Context, ProviderModelListQuery) (ProviderModelListRecord, error) {
				called = true
				return ProviderModelListRecord{}, nil
			},
		}
		rt := newCompletionRuntime(newTestDeps(t, stub))
		got := rt.complete(&cobra.Command{Use: "test"}, kindModel, "", "")
		if len(got) != 0 {
			t.Fatalf("models without provider = %#v, want empty", got)
		}
		if called {
			t.Fatal("ListProviderModels must not be called when provider is empty")
		}
	})

	t.Run("Should return empty on timeout or client error", func(t *testing.T) {
		t.Parallel()

		stub := &stubClient{
			listSessionsFn: func(ctx context.Context, _ SessionListQuery) ([]SessionRecord, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			},
		}
		rt := newCompletionRuntime(newTestDeps(t, stub))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		cmd := &cobra.Command{Use: "test"}
		cmd.SetContext(ctx)

		got := rt.complete(cmd, kindSession, "", "")
		if len(got) != 0 {
			t.Fatalf("canceled completion = %#v, want empty", got)
		}
	})

	t.Run("Should never expose vault secret values", func(t *testing.T) {
		t.Parallel()

		stub := &stubClient{
			listVaultSecretsFn: func(context.Context, VaultListQuery) ([]VaultRecord, error) {
				return []VaultRecord{{Ref: "vault:sessions/s1/tok", Namespace: "sessions", Kind: "api_key"}}, nil
			},
		}
		rt := newCompletionRuntime(newTestDeps(t, stub))
		got := renderCompletionCandidates(rt.complete(&cobra.Command{Use: "test"}, kindVault, "", ""), "")
		if len(got) != 1 || !strings.Contains(got[0], "vault:sessions/s1/tok\tsessions (api_key)") {
			t.Fatalf("vault completion = %#v, want ref + namespace/kind", got)
		}
	})
}

func TestCompletionWorkspaceRefReadOnly(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve workspace flag, env, and cwd without mutating command context", func(t *testing.T) {
		t.Parallel()

		rootDir := t.TempDir()
		subDir := filepath.Join(rootDir, "sub")
		if err := os.MkdirAll(subDir, 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}

		stub := &stubClient{
			listWorkspacesFn: func(context.Context) ([]WorkspaceRecord, error) {
				return []WorkspaceRecord{{ID: "ws_cwd", Name: "cwd-ws", RootDir: rootDir}}, nil
			},
		}
		deps := newTestDeps(t, stub)
		deps.getwd = func() (string, error) { return subDir, nil }

		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String(workspaceFlagName, "", "")
		baseCtx := context.Background()
		cmd.SetContext(baseCtx)
		rt := newCompletionRuntime(deps)

		if got := completionWorkspaceRef(baseCtx, cmd, deps, rt); got != "ws_cwd" {
			t.Fatalf("cwd completionWorkspaceRef() = %q, want %q", got, "ws_cwd")
		}
		if _, cached := commandWorkspaceResolution(cmd); cached {
			t.Fatal("completionWorkspaceRef mutated command context")
		}

		deps.getenv = func(k string) string {
			if k == workspaceEnvName {
				return "ws_env"
			}
			return ""
		}
		if got := completionWorkspaceRef(baseCtx, cmd, deps, rt); got != "ws_env" {
			t.Fatalf("env completionWorkspaceRef() = %q, want %q", got, "ws_env")
		}

		if err := cmd.Flags().Set(workspaceFlagName, "ws_flag"); err != nil {
			t.Fatalf("Set(workspace) error = %v", err)
		}
		if got := completionWorkspaceRef(baseCtx, cmd, deps, rt); got != "ws_flag" {
			t.Fatalf("flag completionWorkspaceRef() = %q, want %q", got, "ws_flag")
		}
	})
}
