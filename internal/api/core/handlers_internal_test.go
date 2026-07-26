package core

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
	workspacepkg "github.com/compozy/agh/internal/workspace"
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

func TestWorkspaceResolveServiceStub(t *testing.T) {
	t.Parallel()

	t.Run("Should return workspace not found when resolve callback is unset", func(t *testing.T) {
		t.Parallel()

		_, err := workspaceResolveServiceStub{}.Resolve(context.Background(), "alpha")
		if !errors.Is(err, workspacepkg.ErrWorkspaceNotFound) {
			t.Fatalf("Resolve() error = %v, want ErrWorkspaceNotFound", err)
		}
	})
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
						Workspace: workspacepkg.Workspace{
							ID:      "ws-empty-root",
							Name:    "alpha",
							RootDir: "",
						},
						WorkspaceID: "ws-empty-root",
					}, nil
				},
			},
		}

		_, err := handlers.createAgentDefinitionPath(context.Background(), contract.CreateAgentRequest{
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
