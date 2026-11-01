package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/listcursor"

	compozyconfig "github.com/compozy/compozy/internal/config"

	"github.com/compozy/compozy/internal/api/contract"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type workspaceResolveServiceStub struct {
	resolve func(context.Context, string) (workspacepkg.ResolvedWorkspace, error)
}

func (s workspaceResolveServiceStub) Register(
	context.Context,
	workspacepkg.RegisterOptions,
) (workspacepkg.Workspace, error) {
	return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceNotFound
}

func (s workspaceResolveServiceStub) Unregister(context.Context, string) error {
	return workspacepkg.ErrWorkspaceNotFound
}

func (s workspaceResolveServiceStub) Update(context.Context, string, workspacepkg.UpdateOptions) error {
	return workspacepkg.ErrWorkspaceNotFound
}

func (s workspaceResolveServiceStub) List(context.Context) ([]workspacepkg.Workspace, error) {
	return nil, nil
}

func (s workspaceResolveServiceStub) Get(context.Context, string) (workspacepkg.Workspace, error) {
	return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceNotFound
}

func (s workspaceResolveServiceStub) Resolve(ctx context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
	if s.resolve == nil {
		return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
	}
	return s.resolve(ctx, ref)
}

func (s workspaceResolveServiceStub) ResolveOrRegister(
	context.Context,
	string,
) (workspacepkg.ResolvedWorkspace, error) {
	return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
}

func TestCreateAgentDefinitionPath(t *testing.T) {
	t.Parallel()

	t.Run("Should return workspace root missing for empty resolved roots", func(t *testing.T) {
		t.Parallel()

		handlers := &BaseHandlers{
			TransportName: "api-core-test",
			Workspaces: workspaceResolveServiceStub{
				resolve: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
					return workspacepkg.ResolvedWorkspace{
						ID:          "ws-empty-root",
						Name:        "alpha",
						RootDir:     "",
						WorkspaceID: "ws-empty-root",
					}, nil
				},
			},
		}

		_, err := handlers.createAgentDefinitionPath(t.Context(), contract.CreateAgentRequest{
			Scope:     contract.AgentCreateScopeWorkspace,
			Workspace: "alpha",
			Agent: contract.CreateAgentPayload{
				Name: "operator",
			},
		})
		if !errors.Is(err, workspacepkg.ErrWorkspaceRootMissing) {
			t.Fatalf("createAgentDefinitionPath() error = %v, want ErrWorkspaceRootMissing", err)
		}
	})
}

// Invariant: deprecated palette IDs emit one structured warning per process history; handler internals own warning deduplication.
func TestPaletteDeprecationWarnings(t *testing.T) {
	t.Parallel()
	t.Run("Should warn once per retired command id [IT-013]", func(t *testing.T) {
		t.Parallel()
		var logs bytes.Buffer
		var seen sync.Map
		logger := slog.New(slog.NewJSONHandler(&logs, nil))
		for range 2 {
			resolvePaletteCommandID(t.Context(), logger, &seen, "app.open.jobs")
		}
		if strings.Count(logs.String(), `"id":"app.open.jobs"`) != 1 ||
			!strings.Contains(logs.String(), `"event":"cmdpalette.command_id_deprecated"`) ||
			!strings.Contains(logs.String(), `"replacement":"app.open.automations"`) ||
			!strings.Contains(logs.String(), `"removal":"v0.5.0"`) {
			t.Fatalf("warnings=%s", logs.String())
		}
	})
}

func (s workspaceResolveServiceStub) SetArchived(context.Context, string, bool) (workspacepkg.Workspace, error) {
	return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceNotFound
}

func TestAgentIdentityPreservation(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve omitted fields and unchanged future appearance on PUT", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "AGENT.md")
		raw := []byte(
			"---\nname: triage\ndisplay_name: Triage Lead\ndescription: Watches issues\nkind: stays_on\nappearance:\n  version: 99\n  future: true\n---\nTriage issues.\n",
		)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		a, err := compozyconfig.LoadAgentDefFile(path)
		if err != nil {
			t.Fatal(err)
		}
		p := contract.CreateAgentPayload{Name: a.Name, Prompt: a.Prompt}
		draft, err := updateAgentDraft(p, a)
		if err != nil {
			t.Fatal(err)
		}
		if draft.DisplayName != a.DisplayName || draft.Description != a.Description || draft.Kind != a.Kind ||
			!compozyconfig.EqualAgentAppearance(draft.Appearance, a.Appearance) {
			t.Fatalf("identity lost: %#v", draft)
		}
		p.Kind = "on_demand"
		if _, err := updateAgentDraft(p, a); !errors.Is(err, compozyconfig.ErrAgentKindImmutable) {
			t.Fatalf("kind: %v", err)
		}
		clone, err := duplicateAgentDraft(a, contract.DuplicateAgentRequest{Name: "triage-lite", Kind: "on_demand"})
		if err != nil || clone.Kind != "on_demand" || clone.DisplayName != "Triage Lead copy" {
			t.Fatalf("duplicate: %#v %v", clone, err)
		}
	})
	t.Run("Should group identical digests and retain distinct definitions", func(t *testing.T) {
		t.Parallel()
		rows := []contract.AgentCatalogItemPayload{
			{Agent: contract.AgentPayload{Name: "triage", DefinitionDigest: "one", WorkspaceID: "a"}},
			{Agent: contract.AgentPayload{Name: "triage", DefinitionDigest: "one", WorkspaceID: "b"}},
			{Agent: contract.AgentPayload{Name: "triage", DefinitionDigest: "two", WorkspaceID: "c"}},
		}
		grouped := groupedAgentRows(rows)
		if len(grouped) != 2 || len(grouped[0].Scopes) != 2 {
			t.Fatalf("grouped: %#v", grouped)
		}
	})
}

func TestAgentIdentityGroupedPaging(t *testing.T) {
	t.Parallel()
	t.Run("Should page five hundred grouped definitions without dropping scopes", func(t *testing.T) {
		t.Parallel()
		rows := make([]contract.AgentCatalogItemPayload, 0, 1000)
		for i := range 500 {
			for _, scope := range []string{"a", "b"} {
				rows = append(
					rows,
					contract.AgentCatalogItemPayload{
						Agent: contract.AgentPayload{
							Name:             fmt.Sprintf("agent-%03d", i),
							DefinitionDigest: "digest",
							WorkspaceID:      scope,
						},
					},
				)
			}
		}
		grouped := groupedAgentRows(rows)
		seen := make(map[string]bool)
		paging := aggregateAgentPaging{fingerprint: "identity-paging", limit: 50}
		for {
			response := contract.AgentCatalogResponse{
				Agents: append([]contract.AgentCatalogItemPayload(nil), grouped...),
			}
			if err := paging.finish(&response); err != nil {
				t.Fatal(err)
			}
			if response.Page.Total != 500 {
				t.Fatalf("total=%d", response.Page.Total)
			}
			for _, row := range response.Agents {
				if seen[row.Agent.Name] || len(row.Scopes) != 2 {
					t.Fatalf("invalid page row: %#v", row)
				}
				seen[row.Agent.Name] = true
			}
			if !response.Page.HasMore {
				break
			}
			paging.cursor = response.Page.NextCursor
			after, err := listcursor.Decode[aggregateAgentPosition](
				paging.cursor,
				1,
				"agents_aggregate",
				paging.fingerprint,
				listcursor.DefaultMaxEncodedSize,
			)
			if err != nil {
				t.Fatal(err)
			}
			paging.after = after
		}
		if len(seen) != 500 {
			t.Fatalf("visited %d agents", len(seen))
		}
	})
}
