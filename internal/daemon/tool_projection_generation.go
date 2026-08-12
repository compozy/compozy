package daemon

func bindToolProjectionCatalogInvalidation(state *bootState) {
	if state == nil || state.toolProjectionEpoch == nil {
		return
	}
	invalidate := func() { state.toolProjectionEpoch.Advance() }
	if state.agentCatalog != nil {
		state.agentCatalog.setOnChange(invalidate)
	}
	if state.toolCatalog != nil {
		state.toolCatalog.setOnChange(invalidate)
	}
	if state.mcpServerCatalog != nil {
		state.mcpServerCatalog.setOnChange(invalidate)
	}
}
