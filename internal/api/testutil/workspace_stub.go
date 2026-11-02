package testutil

import (
	"context"
	"errors"

	core "github.com/compozy/compozy/internal/api/core"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

var ErrStubWorkspaceServiceNotImplemented = errors.New("stub workspace service method not implemented")

type StubWorkspaceService struct {
	ResolveUserFn       func(context.Context, string) (workspacepkg.ResolvedWorkspace, error)
	SetArchivedFn       func(context.Context, string, bool) (workspacepkg.Workspace, error)
	RegisterFn          func(context.Context, workspacepkg.RegisterOptions) (workspacepkg.Workspace, error)
	UnregisterFn        func(context.Context, string) error
	UpdateFn            func(context.Context, string, workspacepkg.UpdateOptions) error
	ListFn              func(context.Context) ([]workspacepkg.Workspace, error)
	GetFn               func(context.Context, string) (workspacepkg.Workspace, error)
	ResolveFn           func(context.Context, string) (workspacepkg.ResolvedWorkspace, error)
	ResolveForProfileFn func(context.Context, string, string) (workspacepkg.ResolvedWorkspace, error)
	ResolveOrRegisterFn func(context.Context, string) (workspacepkg.ResolvedWorkspace, error)
}

func (s StubWorkspaceService) Register(
	ctx context.Context,
	opts workspacepkg.RegisterOptions,
) (workspacepkg.Workspace, error) {
	if s.RegisterFn != nil {
		return s.RegisterFn(ctx, opts)
	}
	return workspacepkg.Workspace{}, ErrStubWorkspaceServiceNotImplemented
}

func (s StubWorkspaceService) Unregister(ctx context.Context, id string) error {
	if s.UnregisterFn != nil {
		return s.UnregisterFn(ctx, id)
	}
	return workspacepkg.ErrWorkspaceNotFound
}

func (s StubWorkspaceService) Update(ctx context.Context, id string, opts workspacepkg.UpdateOptions) error {
	if s.UpdateFn != nil {
		return s.UpdateFn(ctx, id, opts)
	}
	return workspacepkg.ErrWorkspaceNotFound
}

func (s StubWorkspaceService) List(ctx context.Context) ([]workspacepkg.Workspace, error) {
	if s.ListFn != nil {
		return s.ListFn(ctx)
	}
	return nil, nil
}

func (s StubWorkspaceService) Get(ctx context.Context, ref string) (workspacepkg.Workspace, error) {
	if s.GetFn != nil {
		return s.GetFn(ctx, ref)
	}
	return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceNotFound
}

func (s StubWorkspaceService) Resolve(ctx context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
	if s.ResolveFn != nil {
		return s.ResolveFn(ctx, ref)
	}
	return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
}

func (s StubWorkspaceService) ResolveUser(
	ctx context.Context, profileID string,
) (workspacepkg.ResolvedWorkspace, error) {
	if s.ResolveUserFn != nil {
		return s.ResolveUserFn(ctx, profileID)
	}
	return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrUserScopeUnsupported
}

func (s StubWorkspaceService) ResolveForProfile(
	ctx context.Context,
	ref string,
	profileName string,
) (workspacepkg.ResolvedWorkspace, error) {
	if s.ResolveForProfileFn != nil {
		return s.ResolveForProfileFn(ctx, ref, profileName)
	}
	resolved, err := s.Resolve(ctx, ref)
	if err != nil {
		return workspacepkg.ResolvedWorkspace{}, err
	}
	resolved.ProfileName = profileName
	return resolved, nil
}

func (s StubWorkspaceService) ResolveOrRegister(
	ctx context.Context,
	path string,
) (workspacepkg.ResolvedWorkspace, error) {
	if s.ResolveOrRegisterFn != nil {
		return s.ResolveOrRegisterFn(ctx, path)
	}
	return workspacepkg.ResolvedWorkspace{}, ErrStubWorkspaceServiceNotImplemented
}

var _ core.WorkspaceService = (*StubWorkspaceService)(nil)
var _ workspacepkg.ProfileRuntimeResolver = (*StubWorkspaceService)(nil)
var _ workspacepkg.UserRuntimeResolver = (*StubWorkspaceService)(nil)

func (s StubWorkspaceService) SetArchived(
	ctx context.Context,
	ref string,
	archived bool,
) (workspacepkg.Workspace, error) {
	if s.SetArchivedFn != nil {
		return s.SetArchivedFn(ctx, ref, archived)
	}
	return workspacepkg.Workspace{}, ErrStubWorkspaceServiceNotImplemented
}
