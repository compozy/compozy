package demoseed

import (
	"fmt"
	"time"
)

func ratchetLoopRun(clock timeline) loopRunStory {
	createdAt := clock.daysHoursAgo(3, 4)
	generations := make([]loopGenerationStory, 0, 3)
	events := make([]loopEventStory, 0, 24)
	scores := []float64{0.62, 0.81, 0.94}
	for index, score := range scores {
		number := index + 1
		origin := originInitial
		parent := 0
		if number > 1 {
			origin = originGateNext
			parent = number - 1
		}
		at := createdAt.Add(time.Duration(index) * 40 * time.Minute)
		generations = append(generations, loopGenerationStory{
			Number: number, Parent: parent, Origin: origin, CreatedAt: at,
			Cells: append(
				succeededCells(nodeMarketsInput, nodeSplitMarkets, nodeReviewFanout),
				marketReviewCells(number)...,
			),
			Verdicts: []loopVerdictStory{{
				GateID: nodePromotionGate, Outcome: gateOutcomeFor(number, len(scores)),
				Score: score, Scored: true, DecidedAt: at.Add(32 * time.Minute),
			}},
		})
		events = append(
			events,
			nodeLifecycleEvents(at, number, nodeSplitMarkets, nodeReviewMarket)...)
		events = append(events, loopEventStory{
			Kind: evtGateVerdict, At: at.Add(32 * time.Minute),
			Payload: map[string]any{
				keyGeneration: number, keyNodeID: nodePromotionGate,
				keyVerdict: gateOutcomeFor(number, len(scores)), keyValue: score,
			},
		})
	}
	endedAt := createdAt.Add(2*time.Hour + 5*time.Minute)
	events = append(events, terminalStatusEvent(runDone, causeContract, endedAt))
	return loopRunStory{
		ID: "loopr_northstar_market_rollout_done", LoopName: loopMarketRollout,
		WorkspaceKey: workspaceKeyLaunch, Status: runDone, Generation: 3,
		IterationCap: 3, BudgetTokens: 120000, TokensUsed: 84210,
		CreatedAt: createdAt, LastProgressAt: endedAt,
		BestGeneration: 3, BestScore: 0.94,
		Generations: generations, Events: events,
	}
}

func gateOutcomeFor(number int, total int) string {
	if number == total {
		return "approved"
	}
	return "rejected"
}

func marketReviewCells(generation int) []loopCellStory {
	markets := []string{"BR", "MX", "CO", "CL"}
	cells := make([]loopCellStory, 0, len(markets)+2)
	for index, market := range markets {
		cells = append(cells, loopCellStory{
			NodeID: nodeReviewMarket, ItemIndex: index, Status: cellSucceeded, Attempt: 1,
			Payload: fmt.Sprintf(
				`{"market":%q,"verdict":%q,"generation":%d}`,
				market,
				marketVerdict(market),
				generation,
			),
		})
	}
	return append(cells, succeededCells(nodeCollectReviews, nodePromotionGate)...)
}

func marketVerdict(market string) string {
	if market == "BR" {
		return "promote"
	}
	return "hold"
}

func releaseTrainLoopRuns(clock timeline) []loopRunStory {
	createdAt := clock.daysHoursAgo(3, 9)
	childCreated := createdAt.Add(4 * time.Minute)
	child := releaseTrainReadinessChild(childCreated)
	settlementCreated := child.LastProgressAt.Add(time.Minute)
	settlement := releaseTrainSettlementChild(settlementCreated)
	parent := releaseTrainParent(createdAt, settlement.LastProgressAt.Add(time.Minute))
	return []loopRunStory{child, settlement, parent, queuedReleaseTrainRun(clock)}
}

func releaseTrainReadinessChild(createdAt time.Time) loopRunStory {
	childEvents := nodeLifecycleEvents(
		createdAt,
		1,
		nodeMarketInput,
		nodeAssess,
		nodeDecisionGate,
	)
	childEnded := childEvents[len(childEvents)-1].At.Add(time.Minute)
	childEvents = append(childEvents, terminalStatusEvent(runDone, causeContract, childEnded))
	return loopRunStory{
		ID: loopReleaseChildRunID, LoopName: loopLaunchReadiness, WorkspaceKey: workspaceKeyPlatform,
		Status: runDone, Generation: 1, IterationCap: 3, BudgetTokens: 40000, TokensUsed: 15900,
		CreatedAt: createdAt, LastProgressAt: childEnded, ParentRunID: loopReleaseParentRun,
		Generations: []loopGenerationStory{{
			Number: 1, Origin: originInitial, CreatedAt: createdAt,
			Cells: succeededCells(nodeMarketInput, nodeAssess, nodeHasBlockers, nodeDecisionGate),
		}},
		Events: childEvents,
	}
}

func releaseTrainSettlementChild(createdAt time.Time) loopRunStory {
	settlementEvents := nodeLifecycleEvents(
		createdAt,
		1,
		nodePartnerInput,
		nodeAudit,
		nodeOperatorSignoff,
	)
	settlementEnded := settlementEvents[len(settlementEvents)-1].At.Add(time.Minute)
	settlementEvents = append(
		settlementEvents,
		terminalStatusEvent(runDone, causeContract, settlementEnded),
	)
	return loopRunStory{
		ID: loopReleaseSettlementChildRunID, LoopName: loopSettlementAudit,
		WorkspaceKey: workspaceKeyPlatform, Status: runDone, Generation: 1,
		IterationCap: 2, BudgetTokens: 60000, TokensUsed: 12980,
		CreatedAt: createdAt, LastProgressAt: settlementEnded,
		ParentRunID: loopReleaseParentRun,
		Generations: []loopGenerationStory{{
			Number: 1, Origin: originInitial, CreatedAt: createdAt,
			Cells: succeededCells(nodePartnerInput, nodeAudit, nodeOperatorSignoff),
		}},
		Events: settlementEvents,
	}
}

func releaseTrainParent(createdAt time.Time, endedAt time.Time) loopRunStory {
	events := nodeLifecycleEvents(createdAt, 1, nodeMarketInput, nodeReadiness, nodeSettlementAudit)
	events = append(events, terminalStatusEvent(runDone, causeContract, endedAt))
	return loopRunStory{
		ID: loopReleaseParentRun, LoopName: loopReleaseTrain, WorkspaceKey: workspaceKeyPlatform,
		Status: runDone, Generation: 1, IterationCap: 1, BudgetTokens: 0, TokensUsed: 31200,
		CreatedAt: createdAt, LastProgressAt: endedAt,
		Generations: []loopGenerationStory{{
			Number: 1, Origin: originInitial, CreatedAt: createdAt,
			Cells: []loopCellStory{
				{
					NodeID:  nodeMarketInput,
					Status:  cellSucceeded,
					Attempt: 1,
					Payload: `{"market":"BR"}`,
				},
				{
					NodeID:     nodeReadiness,
					Status:     cellSucceeded,
					Attempt:    1,
					ChildRunID: loopReleaseChildRunID,
				},
				{
					NodeID:     nodeSettlementAudit,
					Status:     cellSucceeded,
					Attempt:    1,
					ChildRunID: loopReleaseSettlementChildRunID,
				},
			},
		}},
		Events: events,
	}
}

func queuedReleaseTrainRun(clock timeline) loopRunStory {
	createdAt := clock.minutesAgo(12)
	return loopRunStory{
		ID: "loopr_northstar_release_train_queued", LoopName: loopReleaseTrain,
		WorkspaceKey: workspaceKeyPlatform, Status: runQueued, Generation: 1,
		IterationCap: 1, BudgetTokens: 0, TokensUsed: 0,
		CreatedAt: createdAt, LastProgressAt: createdAt,
		Generations: []loopGenerationStory{{
			Number: 1, Origin: originInitial, CreatedAt: createdAt,
			Cells: []loopCellStory{{NodeID: nodeMarketInput, Status: cellPending, Attempt: 1}},
		}},
	}
}

func quarantineLoopRun(clock timeline) loopRunStory {
	createdAt := clock.hoursMinutesAgo(5, 30)
	attemptOne := createdAt.Add(6 * time.Minute)
	attemptTwo := attemptOne.Add(9 * time.Minute)
	attemptThree := attemptTwo.Add(14 * time.Minute)
	quarantinedAt := attemptThree.Add(3 * time.Minute)
	cells := quarantineCells()
	events := quarantineEvents(createdAt, attemptOne, attemptTwo, attemptThree, quarantinedAt)
	return loopRunStory{
		ID: loopFailedRunID, LoopName: loopDisputeSweep, WorkspaceKey: workspaceKeyPlatform,
		Status: runFailed, Generation: 1, IterationCap: 2, BudgetTokens: 30000, TokensUsed: 21740,
		CreatedAt: createdAt, LastProgressAt: quarantinedAt.Add(2 * time.Minute),
		Generations: []loopGenerationStory{{
			Number: 1, Origin: originInitial, CreatedAt: createdAt, Cells: cells,
			Attempts: quarantineAttempts(attemptOne, attemptTwo, attemptThree),
			Quarantine: &loopQuarantineStory{
				NodeID: nodeBuildBundle,
				Reason: quarantineLaneReason,
				At:     quarantinedAt,
			},
		}},
		Events: events,
	}
}

func quarantineCells() []loopCellStory {
	return []loopCellStory{
		{NodeID: nodeDisputeFiles, Status: cellSucceeded, Attempt: 1, Payload: `{"files":6}`},
		{NodeID: nodeBundleFanout, Status: cellSucceeded, Attempt: 1, Payload: `{"lanes":6}`},
		{
			NodeID:    nodeBuildBundle,
			ItemIndex: 0,
			Status:    cellSucceeded,
			Attempt:   1,
			Payload:   `{"bundle":"d-1001"}`,
		},
		{
			NodeID:    nodeBuildBundle,
			ItemIndex: 1,
			Status:    cellSucceeded,
			Attempt:   1,
			Payload:   `{"bundle":"d-1002"}`,
		},
		{NodeID: nodeBuildBundle, ItemIndex: 2, Status: cellQuarantined, Attempt: 3},
		{
			NodeID:    nodeBuildBundle,
			ItemIndex: 3,
			Status:    cellSucceeded,
			Attempt:   1,
			Payload:   `{"bundle":"d-1004"}`,
		},
		{
			NodeID:    nodeBuildBundle,
			ItemIndex: 4,
			Status:    cellSucceeded,
			Attempt:   1,
			Payload:   `{"bundle":"d-1005"}`,
		},
		{
			NodeID:    nodeBuildBundle,
			ItemIndex: 5,
			Status:    cellSucceeded,
			Attempt:   1,
			Payload:   `{"bundle":"d-1006"}`,
		},
		{
			NodeID:  nodeCollectBundles,
			Status:  cellPartial,
			Attempt: 1,
			Payload: `{"packaged":5,"missing":1}`,
		},
	}
}

func quarantineEvents(
	createdAt time.Time,
	attemptOne time.Time,
	attemptTwo time.Time,
	attemptThree time.Time,
	quarantinedAt time.Time,
) []loopEventStory {
	events := nodeLifecycleEvents(createdAt, 1, nodeDisputeFiles, nodeBundleFanout)
	events = append(events,
		loopEventStory{Kind: evtNodeFailed, At: attemptOne, Payload: map[string]any{
			keyNodeID: nodeBuildBundle, keyItemIndex: 2, keyGeneration: 1,
			keyReason: partnerTimeoutCause,
		}},
		loopEventStory{Kind: evtNodeRetry, At: attemptOne.Add(time.Minute), Payload: map[string]any{
			keyNodeID: nodeBuildBundle, keyItemIndex: 2, keyGeneration: 1, keyScheduleKind: "retry",
		}},
		loopEventStory{Kind: evtNodeFailed, At: attemptTwo, Payload: map[string]any{
			keyNodeID: nodeBuildBundle, keyItemIndex: 2, keyGeneration: 1,
			keyReason: partnerTimeoutCause,
		}},
		loopEventStory{Kind: evtNodeFailed, At: attemptThree, Payload: map[string]any{
			keyNodeID: nodeBuildBundle, keyItemIndex: 2, keyGeneration: 1,
			keyReason: "evidence payload failed its declared schema",
		}},
		loopEventStory{Kind: evtNodeQuarantined, At: quarantinedAt, Payload: map[string]any{
			keyNodeID: nodeBuildBundle, keyItemIndex: 2, keyGeneration: 1,
			keyReason: quarantineLaneReason,
		}},
		terminalStatusEvent(runFailed, causeContract, quarantinedAt.Add(2*time.Minute)),
	)
	return events
}

func quarantineAttempts(first time.Time, second time.Time, third time.Time) []loopAttemptStory {
	return []loopAttemptStory{
		{
			NodeID: nodeBuildBundle, ItemIndex: 2, Attempt: 1,
			FailureClass: "transport", FailureCode: "partner_timeout",
			Cause:  partnerTimeoutCause + " after 30s",
			Hint:   "retry with backoff",
			Target: partnerEvidenceTgt, Disposition: "retried",
			StartedAt: first.Add(-4 * time.Minute), EndedAt: first,
			RetryAt: first.Add(time.Minute),
		},
		{
			NodeID: nodeBuildBundle, ItemIndex: 2, Attempt: 2,
			FailureClass: "transport", FailureCode: "partner_timeout",
			Cause:  partnerTimeoutCause + " after 30s",
			Hint:   "retry with backoff",
			Target: partnerEvidenceTgt, Disposition: "retried",
			StartedAt: second.Add(-5 * time.Minute), EndedAt: second,
			RetryAt: second.Add(2 * time.Minute),
		},
		{
			NodeID: nodeBuildBundle, ItemIndex: 2, Attempt: 3,
			FailureClass: "quality_rejection", FailureCode: "schema_mismatch",
			Cause:  "evidence payload failed its declared schema",
			Hint:   "requeue after the partner export is corrected",
			Target: partnerEvidenceTgt, Disposition: "quarantined",
			StartedAt: third.Add(-8 * time.Minute), EndedAt: third,
		},
	}
}
