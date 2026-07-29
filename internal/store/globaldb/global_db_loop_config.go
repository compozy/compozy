package globaldb

import (
	"context"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

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
	RuntimeDefaults  bool
	RuntimeRules     bool
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
		RuntimeDefaults:  normalized.RuntimeDefaults != nil,
		RuntimeRules:     original.RuntimeRules != nil,
	}
}

func upsertLoopConfigWithExecutor(
	ctx context.Context,
	exec globalSQLExecutor,
	workspaceID string,
	loopName string,
	original looppkg.LoopConfig,
	normalized looppkg.LoopConfig,
) error {
	runtimeDefaultsJSON, err := nullableLoopConfigJSON(normalized.RuntimeDefaults)
	if err != nil {
		return err
	}
	var runtimeRulesValue any
	if normalized.RuntimeRules != nil {
		runtimeRulesValue = normalized.RuntimeRules
	}
	runtimeRulesJSON, err := nullableLoopConfigJSON(runtimeRulesValue)
	if err != nil {
		return err
	}
	insert := sqlcgen.InsertLoopConfigIfMissingParams{
		WorkspaceID:         workspaceID,
		LoopName:            loopName,
		HumanGateEnabled:    int64(boolPtrToInt(normalized.HumanGateEnabled)),
		ReattemptStrategy:   nullStringPtr(normalized.ReattemptStrategy),
		EnabledChecksJson:   enabledChecksForStore(normalized.EnabledChecks),
		IterationCap:        nullIntPtr(normalized.IterationCap),
		BudgetTokens:        nullIntPtr(normalized.BudgetTokens),
		BudgetWallSec:       nullIntPtr(normalized.BudgetWallSec),
		BudgetOnExceeded:    nullStringPtr(normalized.BudgetOnExceeded),
		NoProgressWindow:    nullIntPtr(normalized.NoProgressWindow),
		FanOutWidth:         nullIntPtr(normalized.FanOutWidth),
		GateMaxRevisions:    nullIntPtr(normalized.GateMaxRevisions),
		RuntimeDefaultsJson: runtimeDefaultsJSON,
		RuntimeRulesJson:    runtimeRulesJSON,
	}
	queries := sqlcgen.New(exec)
	if err := queries.InsertLoopConfigIfMissing(ctx, insert); err != nil {
		return err
	}
	return queries.PatchLoopConfig(ctx, loopConfigPatchParams(
		insert,
		loopConfigPatchFlagsForStore(original, normalized),
	))
}
