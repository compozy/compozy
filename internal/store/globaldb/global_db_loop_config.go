package globaldb

import looppkg "github.com/compozy/agh/internal/loop"

type loopConfigPatchFlags struct {
	HumanGate        bool
	Reattempt        bool
	EnabledChecks    bool
	IterationCap     bool
	BudgetTokens     bool
	BudgetWallSec    bool
	BudgetOnExceeded bool
	NoProgressWindow bool
	FanOutWidth      bool
	GateMaxRevisions bool
	ModelWorker      bool
	ModelJudge       bool
}

func loopConfigPatchFlagsForStore(original looppkg.LoopConfig, normalized looppkg.LoopConfig) loopConfigPatchFlags {
	return loopConfigPatchFlags{
		HumanGate:        normalized.HumanGateEnabled != nil,
		Reattempt:        normalized.ReattemptStrategy != nil,
		EnabledChecks:    len(original.EnabledChecks) > 0,
		IterationCap:     normalized.IterationCap != nil,
		BudgetTokens:     normalized.BudgetTokens != nil,
		BudgetWallSec:    normalized.BudgetWallSec != nil,
		BudgetOnExceeded: normalized.BudgetOnExceeded != nil,
		NoProgressWindow: normalized.NoProgressWindow != nil,
		FanOutWidth:      normalized.FanOutWidth != nil,
		GateMaxRevisions: normalized.GateMaxRevisions != nil,
		ModelWorker:      normalized.ModelDefaults != nil && normalized.ModelDefaults.Worker != nil,
		ModelJudge:       normalized.ModelDefaults != nil && normalized.ModelDefaults.Judge != nil,
	}
}
