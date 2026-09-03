package spec

import (
	"github.com/compozy/compozy/internal/api/contract"

	"github.com/compozy/compozy/internal/hooks"
)

func agentCreateScopeValues() []string {
	return []string{
		string(contract.AgentCreateScopeWorkspace),
		string(contract.AgentCreateScopeGlobal),
	}
}

func coordinatorConfigSourceValues() []string {
	return []string{
		string(contract.CoordinatorConfigSourceWorkspace),
		string(contract.CoordinatorConfigSourceGlobal),
		string(contract.CoordinatorConfigSourceDefault),
	}
}

func hookEventValues() []string {
	events := hooks.AllHookEvents()
	values := make([]string, 0, len(events))
	for _, event := range events {
		values = append(values, string(event))
	}
	return values
}

func hookEventFamilyValues() []string {
	return []string{
		string(hooks.HookEventFamilySession),
		string(hooks.HookEventFamilyInput),
		string(hooks.HookEventFamilyPrompt),
		string(hooks.HookEventFamilyEvent),
		string(hooks.HookEventFamilyAgent),
		string(hooks.HookEventFamilyTurn),
		string(hooks.HookEventFamilyMessage),
		string(hooks.HookEventFamilyTool),
		string(hooks.HookEventFamilyPermission),
		string(hooks.HookEventFamilyContext),
		string(hooks.HookEventFamilyTerminal),
	}
}

func hookModeValues() []string {
	return []string{string(hooks.HookModeSync), string(hooks.HookModeAsync)}
}

func hookOutcomeValues() []string {
	return []string{
		string(hooks.HookRunOutcomeApplied),
		string(hooks.HookRunOutcomeDenied),
		string(hooks.HookRunOutcomeFailed),
		string(hooks.HookRunOutcomeSkipped),
		string(hooks.HookRunOutcomeDropped),
		string(hooks.HookRunOutcomeRejected),
	}
}

func hookSkillSourceValues() []string {
	return []string{
		hooks.HookSkillSourceBundled.String(),
		hooks.HookSkillSourceMarketplace.String(),
		hooks.HookSkillSourceUser.String(),
		hooks.HookSkillSourceAdditional.String(),
		hooks.HookSkillSourceWorkspace.String(),
	}
}

func hookExecutorKindValues() []string {
	return []string{
		string(hooks.HookExecutorNative),
		string(hooks.HookExecutorSubprocess),
		string(hooks.HookExecutorWASM),
	}
}

func hookSourceValues() []string {
	return []string{"native", "config", "agent_definition", specSkillKey}
}
