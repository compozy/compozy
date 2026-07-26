package daemon

import (
	"context"
	"errors"

	"github.com/compozy/agh/internal/observe"
)

func (d *Daemon) applyObserverFactoryDefault() {
	if d.newObserver != nil {
		return
	}
	d.newObserver = func(ctx context.Context, deps RuntimeDeps) (Observer, error) {
		source, ok := deps.Sessions.(observe.SessionSource)
		if !ok {
			return nil, errors.New("daemon: session manager does not implement observe session source")
		}
		opts := []observe.Option{
			observe.WithRegistry(deps.Registry),
			observe.WithHomePaths(deps.HomePaths),
			observe.WithSessionSource(source),
			observe.WithCostCatalog(deps.ModelCatalog),
			observe.WithWorkspaceResolver(deps.WorkspaceResolver),
			observe.WithLogger(deps.Logger),
			observe.WithStartTime(deps.StartedAt),
			observe.WithBridgeSource(bridgeObserveSource(deps.Bridges)),
			observe.WithObservabilityConfig(deps.Config.Observability),
			observe.WithAgentProbeSource(
				agentProbeTargetSource(&deps.Config, deps.AgentCatalog, deps.Logger),
				deps.Config.Observability.AgentProbeTimeoutOrDefault(),
			),
		}
		if deps.MemoryStore != nil {
			opts = append(opts, observe.WithMemoryEventSource(deps.MemoryStore))
		}
		return observe.New(ctx, opts...)
	}
}
