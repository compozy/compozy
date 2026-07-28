package daemon

func roleResolverForState(state *bootState) *roleResolver {
	if state == nil {
		return nil
	}
	return state.roleResolver
}

func initializeRoleResolver(state *bootState) {
	if state == nil {
		return
	}
	state.roleResolver = newRoleResolver(
		&state.cfg,
		state.workspaceResolver,
		agentCatalogDependency(state.agentCatalog, agentSidecarCatalogs{
			soul: state.soulCatalog, heartbeat: state.heartbeatCatalog,
		}),
		state.registry,
	)
}
