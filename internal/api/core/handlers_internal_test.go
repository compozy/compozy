package core

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"

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
			resolvePaletteCommandID(logger, &seen, "app.open.jobs")
		}
		if strings.Count(logs.String(), `"id":"app.open.jobs"`) != 1 ||
			!strings.Contains(logs.String(), `"event":"cmdpalette.command_id_deprecated"`) ||
			!strings.Contains(logs.String(), `"replacement":"app.open.automations"`) ||
			!strings.Contains(logs.String(), `"removal":"v0.5.0"`) {
			t.Fatalf("warnings=%s", logs.String())
		}
	})
}
