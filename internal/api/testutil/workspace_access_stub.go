package testutil

import (
	"context"

	"github.com/compozy/compozy/internal/workspaceaccess"
)

// StubWorkspaceAccessPolicy exposes a focused policy seam for transport tests.
type StubWorkspaceAccessPolicy struct {
	AuthorizeFn func(context.Context, workspaceaccess.Request) (workspaceaccess.Decision, error)
}

// Authorize delegates to AuthorizeFn and denies when no behavior is configured.
func (s StubWorkspaceAccessPolicy) Authorize(
	ctx context.Context,
	req workspaceaccess.Request,
) (workspaceaccess.Decision, error) {
	if s.AuthorizeFn == nil {
		return workspaceaccess.Decision{Source: workspaceaccess.SourceDenied}, nil
	}
	return s.AuthorizeFn(ctx, req)
}
