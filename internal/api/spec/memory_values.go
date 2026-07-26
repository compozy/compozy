package spec

import (
	"github.com/compozy/agh/internal/api/contract"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

func memoryTypeValues() []string {
	return []string{
		string(memcontract.TypeUser),
		string(memcontract.TypeFeedback),
		string(memcontract.TypeProject),
		string(memcontract.TypeReference),
	}
}

func memoryScopeValues() []string {
	return []string{string(memcontract.ScopeGlobal), string(memcontract.ScopeWorkspace), string(memcontract.ScopeAgent)}
}

func memoryAgentTierValues() []string {
	return []string{string(memcontract.AgentTierWorkspace), string(memcontract.AgentTierGlobal)}
}

func memoryOriginValues() []string {
	return []string{
		string(memcontract.OriginCLI),
		string(memcontract.OriginHTTP),
		string(memcontract.OriginUDS),
		string(memcontract.OriginTool),
		string(memcontract.OriginExtractor),
		string(memcontract.OriginDreaming),
		string(memcontract.OriginFile),
		string(memcontract.OriginProvider),
	}
}

func memoryOperationValues() []string {
	return []string{
		string(memcontract.OperationWrite),
		string(memcontract.OperationDelete),
		string(memcontract.OperationSearch),
		string(memcontract.OperationReindex),
	}
}

func memoryDecisionOpValues() []string {
	return []string{
		string(contract.MemoryDecisionOpNoop),
		string(contract.MemoryDecisionOpAdd),
		string(contract.MemoryDecisionOpUpdate),
		string(contract.MemoryDecisionOpDelete),
		string(contract.MemoryDecisionOpReject),
	}
}

func memoryDecisionSourceValues() []string {
	return []string{string(memcontract.SourceRule), string(memcontract.SourceLLM)}
}

func memoryTriggerValues() []string {
	return []string{string(memcontract.TriggerPostMessage), string(memcontract.TriggerCompactionFlush)}
}

func memoryProviderStateValues() []string {
	return []string{
		string(contract.MemoryProviderStateActive),
		string(contract.MemoryProviderStateStandby),
		string(contract.MemoryProviderStateCoolingDown),
		string(contract.MemoryProviderStateFailed),
	}
}

func memoryDreamStateValues() []string {
	return []string{
		string(contract.MemoryDreamStateIdle),
		string(contract.MemoryDreamStateRunning),
		string(contract.MemoryDreamStatePromoted),
		string(contract.MemoryDreamStateSkipped),
		string(contract.MemoryDreamStateFailed),
	}
}

func memoryExtractorStateValues() []string {
	return []string{
		string(contract.MemoryExtractorStateIdle),
		string(contract.MemoryExtractorStateRunning),
		string(contract.MemoryExtractorStateDraining),
		string(contract.MemoryExtractorStateStopped),
	}
}
