package loop

import (
	"reflect"
	"testing"

	"github.com/compozy/compozy/internal/loop/dsl"
)

// Invariant: join settlement is a deterministic, monotonic function of durable lane states.
// This suite owns the pure strategy arithmetic introduced by control_join.go.
func TestSettleJoinShouldApplyStrategySemantics(t *testing.T) {
	t.Parallel()

	percent66 := &dsl.StrategyThreshold{Kind: dsl.ThresholdPercent, Percent: 66}
	for _, tt := range []struct {
		name         string
		strategy     dsl.StrategySpec
		lanes        []joinLaneState
		wantState    joinSettlementState
		wantCancel   []int
		wantTrigger  *int
		wantWinner   *int
		wantCoverage CollectOutput
	}{
		{
			name: "Should preserve the wait-all barrier",
			lanes: []joinLaneState{
				{ItemIndex: 0, Status: generationOutputSucceeded, Definitive: true},
				{ItemIndex: 1, Status: generationOutputRunning},
			},
			wantState:    joinSettlementPending,
			wantCoverage: CollectOutput{Total: 2, Succeeded: 1, CoverageRate: 0.5},
		},
		{
			name:     "Should fail fast on the lowest definitive failure",
			strategy: dsl.StrategySpec{Kind: dsl.StrategyFailFast},
			lanes: []joinLaneState{
				{ItemIndex: 3, Status: generationOutputRunning},
				{ItemIndex: 2, Status: generationOutputFailed, Definitive: true},
				{ItemIndex: 1, Status: generationOutputRunning},
				{ItemIndex: 0, Status: generationOutputFailed, Definitive: true},
			},
			wantState: joinSettlementFailed, wantCancel: []int{1, 3}, wantTrigger: new(0),
			wantCoverage: CollectOutput{Total: 4, Failed: 2},
		},
		{
			name:         "Should ignore a retry-eligible failure",
			strategy:     dsl.StrategySpec{Kind: dsl.StrategyFailFast},
			lanes:        []joinLaneState{{ItemIndex: 0, Status: generationOutputFailed}},
			wantState:    joinSettlementPending,
			wantCoverage: CollectOutput{Total: 1},
		},
		{
			name: "Should admit a partial quorum and cancel unsettled lanes",
			strategy: dsl.StrategySpec{Kind: dsl.StrategyBestEffort, Threshold: percent66,
				Missing: dsl.MissingAcceptable},
			lanes: []joinLaneState{
				{ItemIndex: 0, Status: generationOutputSucceeded, Definitive: true},
				{ItemIndex: 1, Status: generationOutputSucceeded, Definitive: true},
				{ItemIndex: 2, Status: generationOutputRunning},
			},
			wantState: joinSettlementPartial, wantCancel: []int{2},
			wantCoverage: CollectOutput{Total: 3, Succeeded: 2, CoverageRate: 0.67, Partial: true},
		},
		{
			name:     "Should select the lowest race winner",
			strategy: dsl.StrategySpec{Kind: dsl.StrategyRace},
			lanes: []joinLaneState{
				{ItemIndex: 2, Status: generationOutputSucceeded, OutputRef: "two", Definitive: true},
				{ItemIndex: 0, Status: generationOutputSucceeded, OutputRef: "zero", Definitive: true},
				{ItemIndex: 1, Status: generationOutputRunning},
			},
			wantState: joinSettlementSucceeded, wantCancel: []int{1}, wantWinner: new(0),
			wantCoverage: CollectOutput{Total: 3, Succeeded: 2, CoverageRate: 0.67},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := settleJoin(tt.strategy, len(tt.lanes), tt.lanes, nil)
			if got.State != tt.wantState || !reflect.DeepEqual(got.CancelItems, tt.wantCancel) ||
				!reflect.DeepEqual(got.TriggerItem, tt.wantTrigger) ||
				!reflect.DeepEqual(got.WinnerItem, tt.wantWinner) ||
				!reflect.DeepEqual(got.Coverage, tt.wantCoverage) {
				t.Fatalf("settleJoin() = %#v, want state=%q cancel=%v trigger=%v winner=%v coverage=%#v",
					got, tt.wantState, tt.wantCancel, tt.wantTrigger, tt.wantWinner, tt.wantCoverage)
			}
		})
	}
}

func TestSettleJoinShouldKeepAnAdmittedDecision(t *testing.T) {
	t.Parallel()

	prior := joinSettlement{
		State:    joinSettlementPartial,
		Coverage: CollectOutput{Total: 3, Succeeded: 2, CoverageRate: 0.67, Partial: true},
	}
	got := settleJoin(dsl.StrategySpec{Kind: dsl.StrategyBestEffort}, 3, []joinLaneState{
		{ItemIndex: 0, Status: generationOutputSucceeded, Definitive: true},
		{ItemIndex: 1, Status: generationOutputSucceeded, Definitive: true},
		{ItemIndex: 2, Status: generationOutputFailed, Definitive: true},
	}, &prior)
	if !reflect.DeepEqual(got, prior) {
		t.Fatalf("settleJoin(prior) = %#v, want %#v", got, prior)
	}
}
