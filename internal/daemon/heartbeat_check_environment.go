package daemon

import (
	"context"
	"sync"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"

	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/heartbeat/source"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type checkWorkspaceReader interface {
	ListWorkspaces(context.Context) ([]workspacepkg.Workspace, error)
}

type instanceCheckEnvironment struct {
	sourceExtensions  source.ExtensionPoller
	sourceConfig      compozyconfig.HeartbeatSourcesConfig
	sourcePollOnce    sync.Once
	sourcePollSem     chan struct{}
	sourceNow         func() time.Time
	sourceMaxInterval time.Duration
	instanceDigestCollector
	definitions instanceDefinitions
	policies    heartbeat.PolicyResolver
	hooks       *hooksNotifier
	workspaces  checkWorkspaceReader
}

var _ heartbeat.CheckEnvironment = (*instanceCheckEnvironment)(nil)

func (e *instanceCheckEnvironment) ResolveCheck(ctx context.Context, id string) (heartbeat.CheckTarget, error) {
	instance, err := e.instances.Get(ctx, id)
	if err != nil {
		return heartbeat.CheckTarget{}, err
	}
	target := heartbeat.CheckTarget{Instance: instance}
	err = resolveInstanceCheckPolicy(ctx, e.definitions, e.policies, &target)
	if err != nil || target.Gate != "" {
		return target, err
	}
	if target.Scope.WorkspaceID == "" && target.Scope.Unrestricted {
		workspaces, err := e.workspaces.ListWorkspaces(ctx)
		if err != nil {
			return target, err
		}
		for _, workspace := range workspaces {
			if workspace.ArchivedAt == nil && !workspace.RootMissing {
				target.Scope.ReachableWorkspaces = append(target.Scope.ReachableWorkspaces, workspace.ID)
			}
		}
	}
	return target, nil
}

func (e *instanceCheckEnvironment) BeforeCheck(
	ctx context.Context,
	target *heartbeat.CheckTarget,
	source string,
) (string, error) {
	reason, err := beforeInstanceCheck(ctx, e.hooks, target, source)
	if err != nil || reason != "" {
		return reason, err
	}
	return "", e.notifyCheckInAuthority(ctx, target)
}

func (e *instanceCheckEnvironment) ReconcileSourcePolicy(ctx context.Context, target *heartbeat.CheckTarget) error {
	return e.instances.ClearRemovedSources(ctx, target.Instance.ID)
}
