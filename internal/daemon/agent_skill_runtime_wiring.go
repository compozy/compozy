package daemon

func wireAgentSkillResources(state *bootState, publisher agentSkillPublisher) {
	state.agentSkillResources = publisher
	state.deps.SkillResources = publisher
	state.deps.AgentDefinitionSync = publisher
}
