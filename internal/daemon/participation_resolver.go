package daemon

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	aghconfig "github.com/compozy/agh/internal/config"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func newDaemonParticipationResolver(
	dependencies any,
	config aghconfig.NetworkConfig,
) (participation.Resolver, error) {
	availability, ok := dependencies.(store.NetworkAvailabilityStore)
	if !ok {
		return nil, fmt.Errorf("daemon: participation resolver requires network availability store")
	}
	channels, ok := dependencies.(store.NetworkChannelStore)
	if !ok {
		return nil, fmt.Errorf("daemon: participation resolver requires network channel store")
	}
	coordination, ok := dependencies.(workspacepkg.CoordinationSettings)
	if !ok {
		return nil, fmt.Errorf("daemon: participation resolver requires workspace coordination settings")
	}
	resolver, err := participation.NewResolver(participation.ResolverOptions{
		Defaults: config.Live.Defaults.ParticipationBounds(),
		Limits:   config.Live.Limits.ParticipationLimits(),
		Availability: func(ctx context.Context) (bool, error) {
			state, readErr := availability.GetNetworkAvailability(ctx)
			if readErr != nil {
				return false, readErr
			}
			return state.Enabled, nil
		},
		ChannelExists: func(ctx context.Context, workspaceID, channelID string) (bool, error) {
			_, readErr := channels.GetNetworkChannel(ctx, store.NetworkChannelRef{
				WorkspaceID: workspaceID,
				Channel:     channelID,
			})
			switch {
			case errors.Is(readErr, sql.ErrNoRows):
				return false, nil
			case readErr != nil:
				return false, readErr
			default:
				return true, nil
			}
		},
		WorkspaceCoordination: func(ctx context.Context, workspaceID string) (bool, error) {
			setting, readErr := coordination.Get(ctx, workspaceID)
			if readErr != nil {
				return false, readErr
			}
			return setting.Enabled, nil
		},
		LiveSupport: func(context.Context, participation.ResolveInput) (bool, error) {
			return true, nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("daemon: construct participation resolver: %w", err)
	}
	return resolver, nil
}

func ensureDaemonParticipationResolver(
	state *bootState,
	dependencies any,
) (participation.Resolver, error) {
	if state == nil {
		return nil, fmt.Errorf("daemon: participation resolver state is required")
	}
	if state.participationResolver != nil {
		var hooksRuntime *hookspkg.Hooks
		if hooks, ok := state.hooks.(*hookspkg.Hooks); ok {
			hooksRuntime = hooks
		}
		state.participationResolver = wrapParticipationResolverWithHooks(
			state.participationResolver,
			hooksRuntime,
		)
		return state.participationResolver, nil
	}
	resolver, err := newDaemonParticipationResolver(dependencies, state.cfg.Network)
	if err != nil {
		return nil, err
	}
	var hooksRuntime *hookspkg.Hooks
	if hooks, ok := state.hooks.(*hookspkg.Hooks); ok {
		hooksRuntime = hooks
	}
	state.participationResolver = wrapParticipationResolverWithHooks(resolver, hooksRuntime)
	return state.participationResolver, nil
}

func participationSnapshotPointer(spec participation.Spec) *participation.Spec {
	if spec == (participation.Spec{}) {
		spec = participation.LocalSpec()
	}
	return &spec
}

func participationSnapshotValue(spec *participation.Spec) participation.Spec {
	if spec == nil || *spec == (participation.Spec{}) {
		return participation.LocalSpec()
	}
	return *spec
}
