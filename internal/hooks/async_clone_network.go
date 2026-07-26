package hooks

import "github.com/compozy/agh/internal/network/participation"

func cloneCoordinatorContext(ctx CoordinatorContext) CoordinatorContext {
	ctx.ResolvedNetworkParticipation = cloneParticipationSpec(ctx.ResolvedNetworkParticipation)
	return ctx
}

func cloneLoopContext(ctx LoopContext) LoopContext {
	ctx.ResolvedNetworkParticipation = cloneParticipationSpec(ctx.ResolvedNetworkParticipation)
	return ctx
}

func cloneSpawnContext(ctx SpawnContext) SpawnContext {
	ctx.ResolvedNetworkParticipation = cloneParticipationSpec(ctx.ResolvedNetworkParticipation)
	return ctx
}

func cloneParticipationSpec(spec *participation.Spec) *participation.Spec {
	if spec == nil {
		return nil
	}
	return participation.CloneSpec(*spec)
}
