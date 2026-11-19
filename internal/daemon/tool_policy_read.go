package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/store"

	compozyconfig "github.com/compozy/compozy/internal/config"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type toolPolicyReadKey struct{}
type toolPolicyRead struct {
	fence       sessionMCPReadFence
	mu          sync.Mutex
	workspaces  map[toolPolicyWorkspaceKey]toolPolicyWorkspace
	authorityMu sync.Mutex
	authorities map[toolPolicyAuthorityKey]toolPolicyAuthority
}
type toolPolicyAuthorityKey struct {
	resolver *nativeToolPolicyResolver
	id       string
	row      string
	read     bool
}
type toolPolicyAuthority struct {
	policy store.SessionPermissionPolicy
	err    error
}

type toolPolicyWorkspaceKey struct {
	resolver           *nativeToolPolicyResolver
	profile, workspace string
}
type toolPolicyWorkspace struct {
	resolved *workspacepkg.ResolvedAgentConfig
	cfg      *compozyconfig.Config
	err      error
}

func (r *nativeToolPolicyResolver) PolicyReadContext(ctx context.Context, fresh bool) context.Context {
	if !fresh && ctx.Value(toolPolicyReadKey{}) != nil {
		return ctx
	}
	if fresh {
		ctx = newMCPOwnerRead(ctx)
		if mcpAuthorityRead(ctx) {
			ctx = context.WithValue(ctx, mcpAuthorityReadKey{}, make(mcpAuthorityReadCache))
		}
	} else {
		ctx = withMCPOwnerRead(ctx)
	}
	var fence sessionMCPReadFence
	if r.readFence != nil {
		fence = r.readFence()
	}
	return context.WithValue(ctx, toolPolicyReadKey{}, &toolPolicyRead{
		fence:      fence,
		workspaces: make(map[toolPolicyWorkspaceKey]toolPolicyWorkspace),
	})
}

func (r *nativeToolPolicyResolver) resolveWorkspaceConfig(
	ctx context.Context, scope toolspkg.Scope,
) (*workspacepkg.ResolvedAgentConfig, *compozyconfig.Config, error) {
	read, ok := ctx.Value(toolPolicyReadKey{}).(*toolPolicyRead)
	if !ok {
		return r.loadWorkspaceConfig(ctx, scope)
	}
	read.mu.Lock()
	defer read.mu.Unlock()
	key := toolPolicyWorkspaceKey{r, scope.ProfileID, scope.WorkspaceID}
	if value, ok := read.workspaces[key]; ok {
		return value.resolved, value.cfg, value.err
	}
	resolved, cfg, err := r.loadWorkspaceConfig(ctx, scope)
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		read.workspaces[key] = toolPolicyWorkspace{resolved, cfg, err}
	}
	return resolved, cfg, err
}

// resolveWorkspaceConfig loads policy inputs without skill discovery and retains the session profile identity.
func (r *nativeToolPolicyResolver) loadWorkspaceConfig(
	ctx context.Context,
	scope toolspkg.Scope,
) (*workspacepkg.ResolvedAgentConfig, *compozyconfig.Config, error) {
	cfg := r.cfg
	workspaceID := strings.TrimSpace(scope.WorkspaceID)
	if workspaceID == "" {
		if r.workspaceResolver == nil {
			return nil, cfg, nil
		}
		workspaceID = workspaceaccess.UserSelector
	}
	if r.workspaceResolver == nil {
		return nil, cfg, nil
	}
	profileName := ""
	if scope.ProfileID != "" {
		var profileErr error
		profileName, profileErr = resolvePromptSkillsProfileName(ctx, r.profileNames, scope.ProfileID)
		if profileErr != nil {
			return nil, nil, fmt.Errorf("daemon: resolve tool policy profile %q: %w", scope.ProfileID, profileErr)
		}
	}
	resolved, err := r.workspaceResolver.ResolveAgentConfig(ctx, workspaceID, profileName)
	if err != nil {
		return nil, nil, fmt.Errorf("daemon: resolve workspace tool policy %q: %w", workspaceID, err)
	}
	if scope.ProfileID != "" {
		resolved.ProfileID = scope.ProfileID
	}
	return &resolved, &resolved.Config, nil
}
