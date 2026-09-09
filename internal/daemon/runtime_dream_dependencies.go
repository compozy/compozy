package daemon

import (
	"context"
	"fmt"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/memory"
	"github.com/compozy/compozy/internal/memory/consolidation"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (d *Daemon) initializeDreamRuntime(state *bootState, sessions SessionManager) {
	if state == nil || state.dreamSvc == nil {
		return
	}
	lockPath := memory.ConsolidationLockPath(state.globalMemoryDir)
	roles := roleResolverForState(state)
	state.dreamRuntime = consolidation.NewRuntime(
		func() bool {
			d.mu.Lock()
			defer d.mu.Unlock()
			return state.cfg.Memory.Enabled
		},
		state.dreamSvc,
		consolidation.NewSessionSpawner(
			sessions,
			state.workspaceResolver,
			true,
			dreamSessionRouteResolver(roles),
		),
		state.cfg.Memory.Dream.CheckInterval,
		state.logger,
		func() (time.Time, error) {
			return memory.NewConsolidationLock(lockPath).LastConsolidatedAt()
		},
		consolidation.WithEligibility(dreamRoleEligibility(sessions, roles)),
	)
}

func dreamSessionRouteResolver(roles RoleResolver) consolidation.SessionRouteResolver {
	return func(ctx context.Context, workspaceID string) (consolidation.SessionRoute, error) {
		correlation := roleInvocationCorrelation{
			WorkspaceID: strings.TrimSpace(workspaceID),
			Event: store.EventCorrelation{
				SchedulerReason: "dream-consolidation",
				ActorKind:       string(taskpkg.ActorKindDaemon),
				ActorID:         "dream-runtime",
			},
		}
		resolved, err := roles.Resolve(
			withRoleInvocationCorrelation(ctx, correlation),
			workspaceID,
			compozyconfig.RoleDream,
		)
		if err != nil {
			return consolidation.SessionRoute{}, fmt.Errorf("resolve dream role: %w", err)
		}
		return consolidation.SessionRoute{
			Enabled:         resolved.Enabled,
			AgentName:       resolved.AgentName,
			Provider:        resolved.Provider,
			Model:           resolved.Model,
			ReasoningEffort: resolved.ReasoningEffort,
			Speed:           resolved.speedValue(),
			ACPOptions:      session.ACPOptionSelectionsFromConfig(resolved.acpOptionsValue()),
			Fallbacks:       append([]compozyconfig.RoleFallback(nil), resolved.Fallbacks...),
			BeforeFallback: func(fallbackCtx context.Context, attempt int, fallback compozyconfig.RoleFallback) error {
				return recordRoleFallbackEvent(fallbackCtx, resolved, correlation, attempt, roleAttemptRoute{
					AgentName:       resolved.AgentName,
					Provider:        fallback.Provider,
					Model:           fallback.Model,
					ReasoningEffort: fallback.ReasoningEffort,
					Speed:           fallback.Speed,
					ACPOptions:      compozyconfig.CloneACPOptionSelections(fallback.ACPOptions),
				})
			},
		}, nil
	}
}

func dreamRoleEligibility(sessions SessionManager, roles RoleResolver) func(context.Context, string) (bool, error) {
	return func(ctx context.Context, workspace string) (bool, error) {
		role, err := roles.Resolve(ctx, workspace, compozyconfig.RoleDream)
		if err != nil {
			return false, err
		}
		if strings.TrimSpace(workspace) != "" || role.Enabled {
			return role.Enabled, nil
		}
		infos, err := sessions.ListAll(ctx)
		if err != nil {
			return false, err
		}
		seen := make(map[string]bool)
		for _, info := range infos {
			if info == nil || info.Type == session.SessionTypeDream || info.WorkspaceID == "" ||
				seen[info.WorkspaceID] {
				continue
			}
			seen[info.WorkspaceID] = true
			role, err := roles.Resolve(ctx, info.WorkspaceID, compozyconfig.RoleDream)
			if err != nil {
				return false, err
			}
			if role.Enabled {
				return true, nil
			}
		}
		return false, nil
	}
}
