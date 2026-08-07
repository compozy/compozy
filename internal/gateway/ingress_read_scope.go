package gateway

import (
	"context"
	"strings"
)

type ingressReadWorkspaceKey struct{}

// WithIngressReadWorkspace scopes binding projections to one validated agent workspace.
func WithIngressReadWorkspace(ctx context.Context, workspaceID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ingressReadWorkspaceKey{}, strings.TrimSpace(workspaceID))
}

func ingressReadWorkspace(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	workspaceID, ok := ctx.Value(ingressReadWorkspaceKey{}).(string)
	return strings.TrimSpace(workspaceID), ok
}
