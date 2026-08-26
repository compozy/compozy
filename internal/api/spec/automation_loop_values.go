package spec

import (
	"github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
)

func automationScopeValues() []string {
	return []string{
		string(automationpkg.AutomationScopeGlobal),
		string(automationpkg.AutomationScopeWorkspace),
	}
}

func automationScheduleModeValues() []string {
	return []string{
		string(automationpkg.ScheduleModeCron),
		string(automationpkg.ScheduleModeEvery),
		string(automationpkg.ScheduleModeAt),
	}
}

func automationSchedulerCatchUpPolicyValues() []string {
	return []string{
		string(automationpkg.SchedulerCatchUpPolicySkipMissed),
		string(automationpkg.SchedulerCatchUpPolicyCoalesce),
		string(automationpkg.SchedulerCatchUpPolicyReplay),
		string(automationpkg.SchedulerCatchUpPolicyRunOnce),
	}
}

func automationRetryStrategyValues() []string {
	return []string{
		string(automationpkg.RetryStrategyNone),
		string(automationpkg.RetryStrategyBackoff),
	}
}

func automationRunStatusValues() []string {
	return []string{
		string(automationpkg.RunScheduled),
		string(automationpkg.RunRunning),
		string(automationpkg.RunDelegated),
		string(automationpkg.RunCompleted),
		string(automationpkg.RunFailed),
		string(automationpkg.RunCancelled),
	}
}

func loopSourceValues() []string {
	return []string{
		string(contract.LoopSourceMarketplace),
		string(contract.LoopSourceUser),
		string(contract.LoopSourceAdditional),
		string(contract.LoopSourceWorkspace),
	}
}

func loopRunStatusValues() []string {
	return contract.LoopRunStatusValues()
}

func loopRunEventKindValues() []string {
	return contract.LoopRunEventKindValues()
}

func loopNodeClassValues() []string {
	return []string{
		string(contract.LoopNodeClassAction),
		string(contract.LoopNodeClassControl),
		string(contract.LoopNodeClassSource),
	}
}

func loopReattemptStrategyValues() []string {
	return []string{
		string(contract.LoopReattemptFailedOnly),
		string(contract.LoopReattemptFullBody),
		string(contract.LoopReattemptHalt),
	}
}

func loopBudgetExceededValues() []string {
	return []string{
		string(contract.LoopBudgetExceededHalt),
		string(contract.LoopBudgetExceededEscalate),
	}
}

func loopEnvironmentModeValues() []string {
	return []string{
		string(contract.LoopEnvironmentModeRoot),
		string(contract.LoopEnvironmentModeWorktree),
		string(contract.LoopEnvironmentModePerRun),
		string(contract.LoopEnvironmentModeDirectory),
	}
}

func loopGateDecisionValues() []string {
	return []string{
		string(contract.LoopGateDecisionApprove),
		string(contract.LoopGateDecisionRequestChanges),
		string(contract.LoopGateDecisionReject),
	}
}

func loopLintSeverityValues() []string {
	return []string{
		string(contract.LoopLintSeverityError),
		string(contract.LoopLintSeverityWarning),
	}
}
