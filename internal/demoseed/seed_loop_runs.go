package demoseed

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/task"
)

// loopSeedCounts reports what one loop-run seeding pass persisted.
type loopSeedCounts struct {
	runs        int
	generations int
	events      int
	goalTurns   int
}

func seedLoopRuns(
	ctx context.Context,
	db *globaldb.GlobalDB,
	state *scenario,
) (loopSeedCounts, error) {
	counts := loopSeedCounts{}
	snapshots := map[string]loopSnapshot{}
	for _, story := range scenarioLoopRuns(state.clock) {
		record, err := state.recordFor(story.WorkspaceKey)
		if err != nil {
			return loopSeedCounts{}, err
		}
		pinned, err := loopSnapshotFor(snapshots, story.LoopName)
		if err != nil {
			return loopSeedCounts{}, err
		}
		snapshot, err := runHistorySnapshot(story, record.ID, pinned)
		if err != nil {
			return loopSeedCounts{}, err
		}
		command, err := looppkg.NewRunHistoryImport(&snapshot)
		if err != nil {
			return loopSeedCounts{}, fmt.Errorf("demo seed: prepare Loop run %q: %w", story.ID, err)
		}
		if err := db.ImportRunHistory(ctx, &command); err != nil {
			return loopSeedCounts{}, fmt.Errorf("demo seed: import Loop run %q: %w", story.ID, err)
		}
		counts.runs++
		counts.generations += len(story.Generations)
		// The run insert appends its own opening status_changed event.
		counts.events += len(snapshot.Events) + 1
		counts.goalTurns += len(snapshot.GoalTurns)
	}
	return counts, nil
}

type loopSnapshot struct {
	raw     json.RawMessage
	digest  string
	version int
}

func loopSnapshotFor(cache map[string]loopSnapshot, loopName string) (loopSnapshot, error) {
	if cached, ok := cache[loopName]; ok {
		return cached, nil
	}
	raw, digest, version, err := compiledLoopSnapshot(loopName)
	if err != nil {
		return loopSnapshot{}, err
	}
	pinned := loopSnapshot{raw: raw, digest: digest, version: version}
	cache[loopName] = pinned
	return pinned, nil
}

func runHistorySnapshot(
	story loopRunStory,
	workspaceID string,
	pinned loopSnapshot,
) (looppkg.RunHistorySnapshot, error) {
	events, err := runHistoryEvents(story)
	if err != nil {
		return looppkg.RunHistorySnapshot{}, err
	}
	snapshot := looppkg.RunHistorySnapshot{
		Run:         historicalLoopRun(story, workspaceID, pinned),
		Generations: runHistoryGenerations(story),
		Events:      events,
		Actor:       operatorActor(workspaceID),
	}
	if story.ID == loopGoalRunID {
		snapshot.GoalTurns = incidentGoalTurns(story)
		snapshot.Events = append(snapshot.Events, incidentGoalTurnEvents(snapshot.GoalTurns)...)
		slices.SortFunc(snapshot.Events, func(left, right looppkg.RunHistoryEvent) int {
			return left.At.Compare(right.At)
		})
	}
	resolved, err := looppkg.LoadExecutedDefinitionSnapshot(pinned.raw, pinned.digest)
	if err != nil {
		return looppkg.RunHistorySnapshot{}, fmt.Errorf(
			"demo seed: load Loop snapshot %q: %w",
			story.LoopName,
			err,
		)
	}
	inputs, err := looppkg.ResolveInputs(
		resolved.Definition,
		looppkg.Inputs{Values: snapshot.Run.Inputs},
	)
	if err != nil {
		return looppkg.RunHistorySnapshot{}, fmt.Errorf(
			"demo seed: resolve Loop inputs for %q: %w",
			story.ID,
			err,
		)
	}
	snapshot.Run.Inputs = inputs
	if story.BestGeneration > 0 {
		snapshot.Best = &looppkg.RunHistoryBest{
			Generation: int64(story.BestGeneration), Score: story.BestScore,
		}
	}
	return snapshot, nil
}

func incidentGoalTurns(story loopRunStory) []looppkg.RunHistoryGoalTurn {
	firstStarted := story.CreatedAt.Add(2*time.Minute + 15*time.Second)
	secondStarted := story.CreatedAt.Add(4*time.Minute + 15*time.Second)
	return []looppkg.RunHistoryGoalTurn{
		{
			Seq: 1, Generation: 1, NodeID: looppkg.NodeID(nodeWritePostmortem), Turn: 1,
			SessionID: scenarioSessionIDs[11], BindingHandle: "goal:incident-postmortem",
			BindingEpoch: 1, PromptID: "prompt_northstar_postmortem_01", PromptAttempt: 1,
			UsageBaseTokens: 18420, StopReason: looppkg.ActionStopEndTurn,
			VerdictOutcome: gate.VerdictOutcomeRejected,
			BlockingIssues: json.RawMessage(
				`[{"code":"missing_timeline_evidence","message":"Attach the authorization latency timeline."}]`,
			),
			Criteria: json.RawMessage(
				`[{"id":"evidence","outcome":"rejected","message":"The first draft lacks the provider latency timeline."}]`,
			),
			Warnings: json.RawMessage(
				`[]`,
			), PromptRef: "demo://goal/incident-postmortem/turn-1",
			TokensUsed: 12780, ActorKind: string(task.ActorKindAgentSession), ActorID: scenarioSessionIDs[11],
			StartedAt: firstStarted, EndedAt: firstStarted.Add(75 * time.Second),
		},
		{
			Seq: 2, Generation: 1, NodeID: looppkg.NodeID(nodeWritePostmortem), Turn: 2,
			SessionID: scenarioSessionIDs[11], BindingHandle: "goal:incident-postmortem",
			BindingEpoch: 1, PromptID: "prompt_northstar_postmortem_02", PromptAttempt: 1,
			UsageBaseTokens: 31200, StopReason: looppkg.ActionStopMaxTurnRequests,
			VerdictOutcome: gate.VerdictOutcomeRejected,
			BlockingIssues: json.RawMessage(
				`[{"code":"owner_unconfirmed","message":"The provider mitigation owner is still unconfirmed."}]`,
			),
			Criteria: json.RawMessage(
				`[{"id":"ownership","outcome":"rejected","message":"Name the provider mitigation owner before publishing."}]`,
			),
			Warnings: json.RawMessage(`[]`), EvidenceRef: "demo://evidence/authorization-dip",
			PromptRef: "demo://goal/incident-postmortem/turn-2", TokensUsed: 15860,
			ActorKind: string(task.ActorKindAgentSession), ActorID: scenarioSessionIDs[11],
			StartedAt: secondStarted, EndedAt: secondStarted.Add(35 * time.Second),
		},
	}
}

func incidentGoalTurnEvents(turns []looppkg.RunHistoryGoalTurn) []looppkg.RunHistoryEvent {
	events := make([]looppkg.RunHistoryEvent, 0, len(turns)*2)
	for _, turn := range turns {
		base := map[string]any{
			keyGeneration: turn.Generation,
			keyNodeID:     string(turn.NodeID),
			"turn":        turn.Turn,
			"prompt_id":   turn.PromptID,
		}
		events = append(
			events,
			looppkg.RunHistoryEvent{
				Kind:    looppkg.RunEventGoalTurnStarted,
				Payload: base,
				At:      turn.StartedAt,
			},
			looppkg.RunHistoryEvent{
				Kind:    looppkg.RunEventGoalTurnCompleted,
				Payload: base,
				At:      turn.EndedAt,
			},
		)
	}
	return events
}

func historicalLoopRun(story loopRunStory, workspaceID string, pinned loopSnapshot) looppkg.Run {
	return looppkg.Run{
		ID: looppkg.RunID(story.ID), WorkspaceID: looppkg.WorkspaceID(workspaceID),
		LoopName: story.LoopName, Status: looppkg.Status(story.Status),
		Generation: story.Generation, ReattemptStrategy: looppkg.ReattemptFailedOnly,
		CreatedAt: story.CreatedAt, StartedAt: story.CreatedAt, LastProgressAt: story.LastProgressAt,
		StartedBy:         task.ActorIdentity{Kind: task.ActorKindHuman, Ref: operatorRef},
		StartedOrigin:     task.Origin{Kind: task.OriginKindWeb, Ref: originWebRef},
		DefinitionVersion: pinned.version, DefinitionDigest: pinned.digest,
		DefinitionSnapshot: pinned.raw,
		ActiveGateID:       looppkg.NodeID(story.ActiveGateID),
		IterationCap:       story.IterationCap, BudgetTokens: story.BudgetTokens,
		BudgetOnExceeded: dsl.BudgetExceededHalt, TokensUsed: story.TokensUsed,
		ParentLoopRunID:       looppkg.RunID(story.ParentRunID),
		GoalContextNudgeRatio: 0.8,
		Origin:                &looppkg.RunOrigin{Kind: looppkg.RunOriginCatalog},
		Inputs:                loopRunInputs(story),
	}
}

func runHistoryGenerations(
	story loopRunStory,
) []looppkg.RunHistoryGeneration {
	generations := make([]looppkg.RunHistoryGeneration, 0, len(story.Generations))
	for _, source := range story.Generations {
		outputs, blobs := generationOutputs(source)
		generation := looppkg.RunHistoryGeneration{
			Intent: looppkg.GenerationIntent{
				Generation:       int64(source.Number),
				ParentGeneration: int64(source.Parent),
				Origin:           looppkg.GenerationOrigin(source.Origin),
			},
			CreatedAt: source.CreatedAt,
			Outputs:   outputs, OutputBlobs: blobs,
			Attempts: generationAttempts(story, source),
			Verdicts: generationVerdicts(source),
		}
		if source.Quarantine != nil {
			generation.Controls = []looppkg.NodeControlMutation{{
				Kind:   looppkg.NodeControlMutationQuarantine,
				NodeID: looppkg.NodeID(source.Quarantine.NodeID),
				QuarantineEntry: json.RawMessage(fmt.Sprintf(
					`{"reason":%q,"attempts":3}`, source.Quarantine.Reason,
				)),
				At: source.Quarantine.At,
			}}
		}
		generations = append(generations, generation)
	}
	return generations
}

func generationOutputs(
	source loopGenerationStory,
) ([]looppkg.GenerationOutput, []looppkg.GenerationOutputBlob) {
	outputs := make([]looppkg.GenerationOutput, 0, len(source.Cells))
	blobs := make([]looppkg.GenerationOutputBlob, 0, len(source.Cells))
	for _, cell := range source.Cells {
		output := looppkg.GenerationOutput{
			Generation: source.Number, NodeID: cell.NodeID, ItemIndex: cell.ItemIndex,
			Status: cell.Status, Attempt: cell.Attempt,
		}
		output.OutputRef = cell.OutputRef
		output.ChildLoopRunID = cell.ChildRunID
		if cell.Payload != "" {
			payload := json.RawMessage(cell.Payload)
			ref := looppkg.OutputRefForPayload(payload)
			output.OutputRef = ref
			blobs = append(blobs, looppkg.GenerationOutputBlob{
				OutputRef: ref, Payload: payload, At: source.CreatedAt,
			})
		}
		outputs = append(outputs, output)
	}
	return outputs, blobs
}

func loopRunInputs(story loopRunStory) map[string]any {
	if story.Inputs != nil {
		return story.Inputs
	}
	switch story.LoopName {
	case loopLaunchReadiness:
		reviewer := agentProductLead
		if story.WorkspaceKey == workspaceKeyPlatform {
			reviewer = agentReleaseManager
		}
		return map[string]any{keyMarket: "BR", "reviewer": reviewer}
	case loopMarketRollout:
		return map[string]any{"markets": "BR,MX,CO,CL", "reviewer": agentProductLead}
	case loopSettlementAudit:
		return map[string]any{"partner": "mercadox", "auditor": agentPlatformEngineer}
	case loopReleaseTrain:
		return map[string]any{keyMarket: "BR", "partner": "mercadox"}
	case loopIncidentPostmort:
		return map[string]any{"incident_id": "2026-08-14-auth-dip", "author": agentProductLead}
	case loopDocsFreshness:
		return map[string]any{"area": "payments"}
	case loopDisputeSweep:
		return map[string]any{"pattern": "data/risk/disputes/*.json"}
	case loopChargebackTriage:
		return map[string]any{"threshold_per_1k": 0.6}
	default:
		return map[string]any{}
	}
}

func generationAttempts(story loopRunStory, source loopGenerationStory) []looppkg.NodeAttempt {
	attempts := make([]looppkg.NodeAttempt, 0, len(source.Attempts))
	for _, attempt := range source.Attempts {
		failureClass := looppkg.FailureClass(attempt.FailureClass)
		ended := attempt.EndedAt
		record := looppkg.NodeAttempt{
			LoopRunID: looppkg.RunID(story.ID), Generation: source.Number,
			NodeID: looppkg.NodeID(attempt.NodeID), ItemIndex: attempt.ItemIndex,
			Attempt: attempt.Attempt, FailureClass: &failureClass,
			FailureCode: attempt.FailureCode, Cause: attempt.Cause, Hint: attempt.Hint,
			Target:      attempt.Target,
			Disposition: looppkg.AttemptDisposition(attempt.Disposition),
			StartedAt:   attempt.StartedAt, EndedAt: &ended,
		}
		if !attempt.RetryAt.IsZero() {
			retryAt := attempt.RetryAt
			record.NextAttemptAt = &retryAt
		}
		attempts = append(attempts, record)
	}
	return attempts
}

func generationVerdicts(source loopGenerationStory) []looppkg.RunHistoryVerdict {
	verdicts := make([]looppkg.RunHistoryVerdict, 0, len(source.Verdicts))
	for _, verdict := range source.Verdicts {
		intent := gate.VerdictIntent{
			GateID: verdict.GateID, Outcome: gate.VerdictOutcome(verdict.Outcome),
			BlockingIssues: json.RawMessage(`[]`), Criteria: json.RawMessage(`[]`),
		}
		if verdict.Scored {
			score := verdict.Score
			intent.Score = &score
		}
		verdicts = append(verdicts, looppkg.RunHistoryVerdict{
			Generation: source.Number, Intent: intent, DecidedAt: verdict.DecidedAt,
		})
	}
	return verdicts
}

func runHistoryEvents(story loopRunStory) ([]looppkg.RunHistoryEvent, error) {
	events := make([]looppkg.RunHistoryEvent, 0, len(story.Events))
	for _, event := range story.Events {
		payload, err := json.Marshal(event.Payload)
		if err != nil {
			return nil, fmt.Errorf(
				"demo seed: encode Loop event %q for %q: %w",
				event.Kind,
				story.ID,
				err,
			)
		}
		events = append(events, looppkg.RunHistoryEvent{
			Kind: looppkg.RunEventKind(event.Kind), Payload: json.RawMessage(payload),
			At: normalizeEventTime(event.At),
		})
	}
	return events, nil
}

func normalizeEventTime(at time.Time) time.Time {
	return at.UTC()
}
