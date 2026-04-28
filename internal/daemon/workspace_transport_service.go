package daemon

import (
	"context"
	"errors"
	"net/http"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/store/globaldb"
)

type transportWorkspaceService struct {
	globalDB *globaldb.GlobalDB
}

var _ apicore.WorkspaceService = (*transportWorkspaceService)(nil)

func newTransportWorkspaceService(globalDB *globaldb.GlobalDB) *transportWorkspaceService {
	return &transportWorkspaceService{globalDB: globalDB}
}

func (s *transportWorkspaceService) Register(
	ctx context.Context,
	path string,
	name string,
) (apicore.WorkspaceRegisterResult, error) {
	if s == nil || s.globalDB == nil {
		return apicore.WorkspaceRegisterResult{}, workspaceTransportUnavailable("workspace registration")
	}

	_, lookupErr := s.globalDB.Get(ctx, path)

	row, err := s.globalDB.Register(ctx, path, name)
	if err != nil {
		return apicore.WorkspaceRegisterResult{}, err
	}
	return apicore.WorkspaceRegisterResult{
		Workspace: transportWorkspace(row),
		Created:   errors.Is(lookupErr, globaldb.ErrWorkspaceNotFound),
	}, nil
}

func (s *transportWorkspaceService) List(ctx context.Context) ([]apicore.Workspace, error) {
	if s == nil || s.globalDB == nil {
		return nil, workspaceTransportUnavailable("workspace listing")
	}

	rows, err := s.globalDB.List(ctx)
	if err != nil {
		return nil, err
	}
	workspaces := make([]apicore.Workspace, 0, len(rows))
	for idx := range rows {
		workspaces = append(workspaces, transportWorkspace(rows[idx]))
	}
	return workspaces, nil
}

func (s *transportWorkspaceService) Get(ctx context.Context, ref string) (apicore.Workspace, error) {
	if s == nil || s.globalDB == nil {
		return apicore.Workspace{}, workspaceTransportUnavailable("workspace lookup")
	}

	row, err := s.globalDB.Get(ctx, ref)
	if err != nil {
		return apicore.Workspace{}, err
	}
	return transportWorkspace(row), nil
}

func (*transportWorkspaceService) Update(
	context.Context,
	string,
	apicore.WorkspaceUpdateInput,
) (apicore.Workspace, error) {
	return apicore.Workspace{}, workspaceTransportUnavailable("workspace updates")
}

func (s *transportWorkspaceService) Delete(ctx context.Context, ref string) error {
	if s == nil || s.globalDB == nil {
		return workspaceTransportUnavailable("workspace unregister")
	}
	return s.globalDB.Unregister(ctx, ref)
}

func (s *transportWorkspaceService) Resolve(ctx context.Context, path string) (apicore.Workspace, error) {
	if s == nil || s.globalDB == nil {
		return apicore.Workspace{}, workspaceTransportUnavailable("workspace resolution")
	}

	row, err := s.globalDB.ResolveOrRegister(ctx, path)
	if err != nil {
		return apicore.Workspace{}, err
	}
	return transportWorkspace(row), nil
}

func (s *transportWorkspaceService) Sync(ctx context.Context) (apicore.WorkspaceSyncResult, error) {
	if s == nil || s.globalDB == nil {
		return apicore.WorkspaceSyncResult{}, workspaceTransportUnavailable("workspace sync")
	}
	return refreshRegisteredWorkspaces(ctx, s.globalDB, workspaceRefreshOptions{SyncPresent: true})
}

func workspaceTransportUnavailable(action string) error {
	return apicore.NewProblem(
		http.StatusServiceUnavailable,
		"workspace_service_unavailable",
		action+" is not available in this daemon build",
		nil,
		nil,
	)
}
