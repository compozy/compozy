package spec

import (
	"reflect"

	"github.com/compozy/agh/internal/api/contract"
)

func withGoalSchemaEnumValues(values map[reflect.Type][]string) map[reflect.Type][]string {
	values[reflect.TypeFor[contract.GoalCommandOutcome]()] = contract.GoalCommandOutcomeValues()
	values[reflect.TypeFor[contract.GoalStatus]()] = contract.GoalStatusValues()
	values[reflect.TypeFor[contract.GoalTurnResultStatus]()] = contract.GoalTurnResultStatusValues()
	values[reflect.TypeFor[contract.GoalVerdictOutcome]()] = contract.GoalVerdictOutcomeValues()
	values[reflect.TypeFor[contract.ACPPromptStopReason]()] = contract.ACPPromptStopReasonValues()
	values[reflect.TypeFor[contract.GoalContextState]()] = contract.GoalContextStateValues()
	values[reflect.TypeFor[contract.GoalPromptKind]()] = contract.GoalPromptKindValues()
	values[reflect.TypeFor[contract.GoalSnapshotChangeCause]()] = contract.GoalSnapshotChangeCauseValues()
	values[reflect.TypeFor[contract.GoalReasonCode]()] = contract.GoalReasonCodeValues()
	return values
}
