package demoseed

import (
	"fmt"
	"time"
)

func liveLoopRuns(clock timeline) []loopRunStory {
	return []loopRunStory{
		approvalLoopRun(clock),
		watchingLoopRun(clock),
		runningFanOutLoopRun(clock),
		pausedLoopRun(clock),
	}
}

func approvalLoopRun(clock timeline) loopRunStory {
	createdAt := clock.hoursMinutesAgo(1, 40)
	auditedAt := createdAt.Add(52 * time.Minute)
	openedAt := auditedAt.Add(2 * time.Minute)
	events := nodeLifecycleEvents(createdAt, 1, nodePartnerInput, nodeAudit)
	events = append(
		events,
		loopEventStory{Kind: evtGateVerdict, At: openedAt, Payload: map[string]any{
			keyGeneration: 1, keyNodeID: nodeOperatorSignoff, keyVerdict: "awaiting_approval",
		}},
		loopEventStory{
			Kind: evtNeedsApproval,
			At:   openedAt.Add(time.Second),
			Payload: map[string]any{
				keyGeneration: 1, keyNodeID: nodeOperatorSignoff,
			},
		},
		loopEventStory{
			Kind: evtStatusChanged, At: openedAt.Add(2 * time.Second),
			Payload: map[string]any{
				keyFrom: cellRunning, keyTo: runNeedsApproval,
				keyStatus: runNeedsApproval, keyCause: causeApproval,
			},
		},
	)
	return loopRunStory{
		ID: loopApprovalRunID, LoopName: loopSettlementAudit, WorkspaceKey: workspaceKeyPlatform,
		Status: runNeedsApproval, Generation: 1, IterationCap: 2,
		BudgetTokens: 60000, TokensUsed: 23880,
		CreatedAt: createdAt, LastProgressAt: openedAt, ActiveGateID: nodeOperatorSignoff,
		Generations: []loopGenerationStory{{
			Number: 1, Origin: originInitial, CreatedAt: createdAt,
			Cells: []loopCellStory{
				{
					NodeID:  nodePartnerInput,
					Status:  cellSucceeded,
					Attempt: 1,
					Payload: `{"partner":"mercadox"}`,
				},
				{
					NodeID: nodeAudit, Status: cellSucceeded, Attempt: 1,
					Payload: `{"batches":47,"drift":0.0,"gap":"14:08-14:30 UTC"}`,
				},
				{NodeID: nodeOperatorSignoff, Status: cellWaiting, Attempt: 1},
			},
			Verdicts: []loopVerdictStory{{
				GateID: nodeOperatorSignoff, Outcome: "awaiting_approval", DecidedAt: openedAt,
			}},
		}},
		Events: events,
	}
}

func watchingLoopRun(clock timeline) loopRunStory {
	createdAt := clock.hoursAgo(9)
	parkedAt := createdAt.Add(3 * time.Minute)
	return loopRunStory{
		ID: loopWatchingRunID, LoopName: loopChargebackTriage, WorkspaceKey: workspaceKeyLaunch,
		Status: runWatching, Generation: 1, IterationCap: 0, BudgetTokens: 0, TokensUsed: 1260,
		CreatedAt: createdAt, LastProgressAt: parkedAt,
		Generations: []loopGenerationStory{{
			Number: 1, Origin: originInitial, CreatedAt: createdAt,
			Cells: []loopCellStory{{
				NodeID: nodeSettlementBatch, Status: cellWaiting, Attempt: 1,
				OutputRef: `{"kind":"watch_events_pending","subscriptions":` +
					`[{"kind":"automation.run.completed"}],"cursors":{"automation_runs":2},` +
					`"cursor_version":1}`,
			}},
		}},
		Events: []loopEventStory{
			{Kind: evtGenerationStarted, At: createdAt, Payload: map[string]any{keyGeneration: 1}},
			{Kind: evtNodeWaitStarted, At: parkedAt, Payload: map[string]any{
				keyGeneration: 1, keyNodeID: nodeSettlementBatch, keyWaitKind: "event",
				keyKind: "automation.run.completed",
			}},
			{Kind: evtStatusChanged, At: parkedAt.Add(time.Second), Payload: map[string]any{
				keyFrom: cellRunning, keyTo: runWatching, keyStatus: runWatching, keyCause: causeWatchEvents,
			}},
		},
	}
}

func runningFanOutLoopRun(clock timeline) loopRunStory {
	createdAt := clock.minutesAgo(18)
	cells := []loopCellStory{
		{
			NodeID:  nodeMarketsInput,
			Status:  cellSucceeded,
			Attempt: 1,
			Payload: `{"markets":"BR,MX,CO,CL"}`,
		},
		{NodeID: nodeSplitMarkets, Status: cellSucceeded, Attempt: 1, Payload: `{"count":12}`},
		{NodeID: nodeReviewFanout, Status: cellSucceeded, Attempt: 1, Payload: `{"lanes":12}`},
	}
	events := nodeLifecycleEvents(
		createdAt,
		1,
		nodeMarketsInput,
		nodeSplitMarkets,
		nodeReviewFanout,
	)
	for index := range 12 {
		cells = append(cells, fanOutLaneCell(index))
	}
	cells = append(
		cells,
		loopCellStory{NodeID: nodeCollectReviews, Status: cellPending, Attempt: 1},
	)
	events = append(events, loopEventStory{
		Kind: evtTokenTick, At: clock.minutesAgo(2),
		Payload: map[string]any{keyTokensUsed: int64(38400), keyTerminal: false},
	})
	return loopRunStory{
		ID: loopRunningRunID, LoopName: loopMarketRollout, WorkspaceKey: workspaceKeyLaunch,
		Status: cellRunning, Generation: 1, IterationCap: 2,
		BudgetTokens: 120000, TokensUsed: 38400,
		CreatedAt: createdAt, LastProgressAt: clock.minutesAgo(2),
		Generations: []loopGenerationStory{{
			Number: 1, Origin: originInitial, CreatedAt: createdAt, Cells: cells,
		}},
		Events: events,
	}
}

func fanOutLaneCell(index int) loopCellStory {
	switch {
	case index < 7:
		return loopCellStory{
			NodeID: nodeReviewMarket, ItemIndex: index, Status: cellSucceeded, Attempt: 1,
			Payload: fmt.Sprintf(`{"lane":%d,"verdict":"hold"}`, index),
		}
	case index < 10:
		return loopCellStory{
			NodeID:    nodeReviewMarket,
			ItemIndex: index,
			Status:    cellRunning,
			Attempt:   1,
		}
	default:
		return loopCellStory{
			NodeID:    nodeReviewMarket,
			ItemIndex: index,
			Status:    cellPending,
			Attempt:   1,
		}
	}
}

func pausedLoopRun(clock timeline) loopRunStory {
	createdAt := clock.hoursMinutesAgo(2, 15)
	pausedAt := createdAt.Add(24 * time.Minute)
	events := nodeLifecycleEvents(createdAt, 1, nodeDisputeFiles)
	events = append(
		events,
		loopEventStory{Kind: evtNodePaused, At: pausedAt, Payload: map[string]any{
			keyGeneration: 1, keyNodeID: nodeBuildBundle,
			keyReason: "partner evidence endpoint is under maintenance",
		}},
		loopEventStory{
			Kind: evtStatusChanged,
			At:   pausedAt.Add(time.Second),
			Payload: map[string]any{
				keyFrom: cellRunning, keyTo: runPaused, keyStatus: runPaused, keyCause: causePauseBoundary,
			},
		},
	)
	return loopRunStory{
		ID: "loopr_northstar_dispute_sweep_paused", LoopName: loopDisputeSweep,
		WorkspaceKey: workspaceKeyPlatform, Status: cellPaused, Generation: 1,
		IterationCap: 2, BudgetTokens: 30000, TokensUsed: 6100,
		CreatedAt: createdAt, LastProgressAt: pausedAt,
		Generations: []loopGenerationStory{{
			Number: 1, Origin: originInitial, CreatedAt: createdAt,
			Cells: []loopCellStory{
				{
					NodeID:  nodeDisputeFiles,
					Status:  cellSucceeded,
					Attempt: 1,
					Payload: `{"files":4}`,
				},
				{
					NodeID:  nodeBundleFanout,
					Status:  cellSucceeded,
					Attempt: 1,
					Payload: `{"lanes":4}`,
				},
				{NodeID: nodeBuildBundle, ItemIndex: 0, Status: cellPaused, Attempt: 1},
			},
			Quarantine: nil,
		}},
		Events: events,
	}
}
