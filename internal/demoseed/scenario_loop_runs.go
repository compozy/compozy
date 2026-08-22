package demoseed

import (
	"fmt"
	"time"
)

type loopCellStory struct {
	NodeID     string
	ItemIndex  int
	Status     string
	Attempt    int
	Payload    string
	OutputRef  string
	ChildRunID string
}

type loopAttemptStory struct {
	NodeID       string
	ItemIndex    int
	Attempt      int
	FailureClass string
	FailureCode  string
	Cause        string
	Hint         string
	Target       string
	Disposition  string
	StartedAt    time.Time
	EndedAt      time.Time
	RetryAt      time.Time
}

type loopVerdictStory struct {
	GateID    string
	Outcome   string
	Score     float64
	Scored    bool
	DecidedAt time.Time
}

type loopQuarantineStory struct {
	NodeID string
	Reason string
	At     time.Time
}

type loopGenerationStory struct {
	Number     int
	Parent     int
	Origin     string
	CreatedAt  time.Time
	Cells      []loopCellStory
	Attempts   []loopAttemptStory
	Verdicts   []loopVerdictStory
	Quarantine *loopQuarantineStory
}

type loopEventStory struct {
	Kind    string
	Payload map[string]any
	At      time.Time
}

type loopRunStory struct {
	ID             string
	LoopName       string
	WorkspaceKey   string
	Status         string
	Generation     int
	IterationCap   int
	BudgetTokens   int
	TokensUsed     int64
	CreatedAt      time.Time
	LastProgressAt time.Time
	ActiveGateID   string
	ParentRunID    string
	BestGeneration int
	BestScore      float64
	Inputs         map[string]any
	Generations    []loopGenerationStory
	Events         []loopEventStory
}

func scenarioLoopRuns(clock timeline) []loopRunStory {
	runs := terminalLoopRuns(clock)
	runs = append(runs, ratchetLoopRun(clock))
	runs = append(runs, releaseTrainLoopRuns(clock)...)
	runs = append(runs, quarantineLoopRun(clock))
	return append(runs, liveLoopRuns(clock)...)
}

func scenarioLoopRunIDs() []string {
	runs := scenarioLoopRuns(newTimeline(time.Unix(0, 0)))
	ids := make([]string, 0, len(runs))
	for _, run := range runs {
		ids = append(ids, run.ID)
	}
	return ids
}

// succeededCells marks a straight-line pass across the named nodes.
func succeededCells(nodeIDs ...string) []loopCellStory {
	cells := make([]loopCellStory, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		cells = append(cells, loopCellStory{
			NodeID: nodeID, Status: cellSucceeded, Attempt: 1,
			Payload: fmt.Sprintf(`{"node":%q,"status":%q}`, nodeID, cellSucceeded),
		})
	}
	return cells
}

func nodeLifecycleEvents(at time.Time, generation int, nodeIDs ...string) []loopEventStory {
	events := []loopEventStory{{
		Kind: evtGenerationStarted, At: at,
		Payload: map[string]any{keyGeneration: generation},
	}}
	cursor := at
	for _, nodeID := range nodeIDs {
		cursor = cursor.Add(30 * time.Second)
		events = append(events, loopEventStory{
			Kind: evtNodeRunning, At: cursor,
			Payload: map[string]any{keyNodeID: nodeID, keyGeneration: generation},
		})
		cursor = cursor.Add(90 * time.Second)
		events = append(events, loopEventStory{
			Kind: evtNodeSucceeded, At: cursor,
			Payload: map[string]any{keyNodeID: nodeID, keyGeneration: generation},
		})
	}
	return events
}

func terminalStatusEvent(status string, cause string, at time.Time) loopEventStory {
	return loopEventStory{
		Kind: evtStatusChanged, At: at,
		Payload: map[string]any{
			keyFrom:   cellRunning,
			keyTo:     status,
			keyStatus: status,
			keyCause:  cause,
		},
	}
}

func terminalLoopRuns(clock timeline) []loopRunStory {
	return []loopRunStory{
		simpleTerminalRun(clock, simpleRunInput{
			id: "loopr_northstar_launch_readiness_done", loopName: loopLaunchReadiness,
			workspaceKey: workspaceKeyLaunch, status: runDone, cause: causeContract,
			startedAgo: 5 * time.Hour, tokens: 18420, budget: 40000, iterationCap: 3,
			nodes: []string{nodeMarketInput, nodeAssess, nodeHasBlockers, nodeDecisionGate},
		}),
		simpleTerminalRun(clock, simpleRunInput{
			id: "loopr_northstar_docs_freshness_done", loopName: loopDocsFreshness,
			workspaceKey: workspaceKeyPlatform, status: runDone, cause: causeContract,
			startedAgo: 6 * 24 * time.Hour, tokens: 4180, budget: 20000, iterationCap: 1,
			nodes: []string{nodeAreaInput, nodeFlagStale},
		}),
		simpleTerminalRun(clock, simpleRunInput{
			id: "loopr_northstar_docs_freshness_noop", loopName: loopDocsFreshness,
			workspaceKey: workspaceKeyPlatform, status: runNoOp, cause: causeContract,
			startedAgo: 2 * 24 * time.Hour, tokens: 910, budget: 20000, iterationCap: 1,
			nodes: []string{nodeAreaInput, nodeFlagStale},
		}),
		simpleTerminalRun(clock, simpleRunInput{
			id: "loopr_northstar_launch_readiness_stalled", loopName: loopLaunchReadiness,
			workspaceKey: workspaceKeyLaunch, status: runStalled, cause: causeNoProgress,
			startedAgo: 8 * 24 * time.Hour, tokens: 26100, budget: 40000, iterationCap: 3,
			nodes: []string{nodeMarketInput, nodeAssess},
		}),
		simpleTerminalRun(clock, simpleRunInput{
			id: "loopr_northstar_settlement_blocked", loopName: loopSettlementAudit,
			workspaceKey: workspaceKeyPlatform, status: runBlocked, cause: causeContract,
			startedAgo: 11 * 24 * time.Hour, tokens: 12300, budget: 60000, iterationCap: 2,
			nodes: []string{nodePartnerInput, nodeAudit},
		}),
		simpleTerminalRun(clock, simpleRunInput{
			id: loopGoalRunID, loopName: loopIncidentPostmort,
			workspaceKey: workspaceKeyLaunch, status: runExhausted, cause: causeBudget,
			startedAgo: 13 * 24 * time.Hour, tokens: 90000, budget: 90000, iterationCap: 4,
			nodes: []string{nodeIncidentInput, nodeWritePostmortem},
		}),
		simpleTerminalRun(clock, simpleRunInput{
			id: "loopr_northstar_market_rollout_canceled", loopName: loopMarketRollout,
			workspaceKey: workspaceKeyLaunch, status: runCanceled, cause: causeOperatorCancel,
			startedAgo: 9 * 24 * time.Hour, tokens: 7400, budget: 120000, iterationCap: 2,
			nodes: []string{nodeMarketsInput, nodeSplitMarkets},
		}),
	}
}

type simpleRunInput struct {
	id           string
	loopName     string
	workspaceKey string
	status       string
	cause        string
	startedAgo   time.Duration
	tokens       int64
	budget       int
	iterationCap int
	nodes        []string
}

func simpleTerminalRun(clock timeline, input simpleRunInput) loopRunStory {
	createdAt := clock.Now().Add(-input.startedAgo)
	events := nodeLifecycleEvents(createdAt, 1, input.nodes...)
	cells := succeededCells(input.nodes...)
	if input.status != runDone && input.status != runNoOp {
		last := len(cells) - 1
		cells[last].Status = cellFailed
		cells[last].Payload = fmt.Sprintf(
			`{"node":%q,"status":%q,"cause":%q}`,
			input.nodes[last],
			cellFailed,
			input.cause,
		)
		terminalKind := evtNodeFailed
		if input.status == runCanceled {
			cells[last].Status = cellCanceled
			terminalKind = evtNodeCanceled
		}
		events[len(events)-1].Kind = terminalKind
		events[len(events)-1].Payload[keyReason] = input.cause
	}
	endedAt := events[len(events)-1].At.Add(time.Minute)
	events = append(events,
		loopEventStory{
			Kind: evtTokenTick, At: endedAt.Add(-30 * time.Second),
			Payload: map[string]any{keyTokensUsed: input.tokens, keyTerminal: true},
		},
		terminalStatusEvent(input.status, input.cause, endedAt),
	)
	return loopRunStory{
		ID: input.id, LoopName: input.loopName, WorkspaceKey: input.workspaceKey,
		Status: input.status, Generation: 1, IterationCap: input.iterationCap,
		BudgetTokens: input.budget, TokensUsed: input.tokens,
		CreatedAt: createdAt, LastProgressAt: endedAt,
		Generations: []loopGenerationStory{{
			Number: 1, Origin: originInitial, CreatedAt: createdAt,
			Cells: cells,
		}},
		Events: events,
	}
}
