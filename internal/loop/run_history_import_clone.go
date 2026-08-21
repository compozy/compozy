package loop

import (
	"encoding/json"
	"maps"
	"slices"
	"time"
)

func cloneRunHistorySnapshot(snapshot *RunHistorySnapshot) RunHistorySnapshot {
	cloned := *snapshot
	cloned.Run = cloneRunHistoryRun(snapshot.Run)
	cloned.Generations = cloneRunHistoryGenerations(snapshot.Generations)
	cloned.Decisions = slices.Clone(snapshot.Decisions)
	cloned.Events = cloneRunHistoryEvents(snapshot.Events)
	cloned.GoalTurns = cloneRunHistoryGoalTurns(snapshot.GoalTurns)
	if snapshot.Best != nil {
		best := *snapshot.Best
		cloned.Best = &best
	}
	return cloned
}

func cloneRunHistoryGoalTurns(turns []RunHistoryGoalTurn) []RunHistoryGoalTurn {
	cloned := make([]RunHistoryGoalTurn, len(turns))
	for index, turn := range turns {
		cloned[index] = turn
		cloned[index].BlockingIssues = cloneRawMessage(turn.BlockingIssues)
		cloned[index].Criteria = cloneRawMessage(turn.Criteria)
		cloned[index].Warnings = cloneRawMessage(turn.Warnings)
	}
	return cloned
}

func cloneRunHistoryRun(run Run) Run {
	cloned := run
	cloned.DefinitionSnapshot = cloneRawMessage(run.DefinitionSnapshot)
	if run.ActiveHumanCriteria != nil {
		activeHumanCriteria := run.ActiveHumanCriteriaValue()
		cloned.ActiveHumanCriteria = &activeHumanCriteria
	}
	cloned.Inputs = cloneRunHistoryValues(run.Inputs)
	cloned.StartMetadata = cloneRunHistoryValues(run.StartMetadata)
	if run.Origin != nil {
		origin := *run.Origin
		cloned.Origin = &origin
	}
	return cloned
}

func cloneRunHistoryValues(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = cloneRunHistoryValue(value)
	}
	return cloned
}

func cloneRunHistoryValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneRunHistoryValues(typed)
	case []any:
		cloned := make([]any, len(typed))
		for index := range typed {
			cloned[index] = cloneRunHistoryValue(typed[index])
		}
		return cloned
	case []string:
		return slices.Clone(typed)
	case []int:
		return slices.Clone(typed)
	case []int64:
		return slices.Clone(typed)
	case map[string]string:
		return maps.Clone(typed)
	case json.RawMessage:
		return cloneRawMessage(typed)
	default:
		return typed
	}
}

func cloneRunHistoryGenerations(generations []RunHistoryGeneration) []RunHistoryGeneration {
	cloned := make([]RunHistoryGeneration, 0, len(generations))
	for _, generation := range generations {
		cloned = append(cloned, cloneRunHistoryGeneration(generation))
	}
	return cloned
}

func cloneRunHistoryGeneration(generation RunHistoryGeneration) RunHistoryGeneration {
	cloned := generation
	cloned.Outputs = cloneRunHistoryOutputs(generation.Outputs)
	cloned.OutputBlobs = cloneRunHistoryBlobs(generation.OutputBlobs)
	cloned.Attempts = cloneRunHistoryAttempts(generation.Attempts)
	cloned.Controls = cloneRunHistoryControls(generation.Controls)
	cloned.Waits = cloneRunHistoryWaits(generation.Waits)
	cloned.Requests = cloneRunHistoryRequests(generation.Requests)
	cloned.Verdicts = cloneRunHistoryVerdicts(generation.Verdicts)
	return cloned
}

func cloneRunHistoryOutputs(outputs []GenerationOutput) []GenerationOutput {
	cloned := make([]GenerationOutput, 0, len(outputs))
	for _, output := range outputs {
		copied := output
		if output.ResolvedRuntime != nil {
			runtime := *output.ResolvedRuntime
			copied.ResolvedRuntime = &runtime
		}
		copied.NextAttemptAt = cloneRunHistoryTime(output.NextAttemptAt)
		copied.FirstScheduledAt = cloneRunHistoryTime(output.FirstScheduledAt)
		copied.ExpectedEpoch = cloneRunHistoryEpoch(output.ExpectedEpoch)
		cloned = append(cloned, copied)
	}
	return cloned
}

func cloneRunHistoryBlobs(blobs []GenerationOutputBlob) []GenerationOutputBlob {
	cloned := make([]GenerationOutputBlob, 0, len(blobs))
	for _, blob := range blobs {
		copied := blob
		copied.Payload = cloneRawMessage(blob.Payload)
		cloned = append(cloned, copied)
	}
	return cloned
}

func cloneRunHistoryAttempts(attempts []NodeAttempt) []NodeAttempt {
	cloned := make([]NodeAttempt, 0, len(attempts))
	for _, attempt := range attempts {
		copied := attempt
		if attempt.FailureClass != nil {
			failureClass := *attempt.FailureClass
			copied.FailureClass = &failureClass
		}
		copied.EndedAt = cloneRunHistoryTime(attempt.EndedAt)
		copied.NextAttemptAt = cloneRunHistoryTime(attempt.NextAttemptAt)
		cloned = append(cloned, copied)
	}
	return cloned
}

func cloneRunHistoryControls(controls []NodeControlMutation) []NodeControlMutation {
	cloned := make([]NodeControlMutation, 0, len(controls))
	for _, control := range controls {
		copied := control
		copied.QuarantineEntry = cloneRawMessage(control.QuarantineEntry)
		if control.GateRevisions != nil {
			revisions := make(map[int]int, len(control.GateRevisions))
			maps.Copy(revisions, control.GateRevisions)
			copied.GateRevisions = revisions
		}
		cloned = append(cloned, copied)
	}
	return cloned
}

func cloneRunHistoryWaits(waits []NodeWaitIntent) []NodeWaitIntent {
	cloned := make([]NodeWaitIntent, 0, len(waits))
	for _, wait := range waits {
		copied := wait
		copied.ResumeAt = cloneRunHistoryTime(wait.ResumeAt)
		copied.NextEscalationAt = cloneRunHistoryTime(wait.NextEscalationAt)
		copied.ClaimedAt = cloneRunHistoryTime(wait.ClaimedAt)
		copied.Expect = cloneRawMessage(wait.Expect)
		copied.AheadPayload = cloneRawMessage(wait.AheadPayload)
		cloned = append(cloned, copied)
	}
	return cloned
}

func cloneRunHistoryRequests(requests []RequestIntent) []RequestIntent {
	cloned := make([]RequestIntent, 0, len(requests))
	for _, request := range requests {
		copied := request
		copied.Context = cloneRawMessage(request.Context)
		copied.ContextPreview = cloneRawMessage(request.ContextPreview)
		copied.AnswerSchema = cloneRawMessage(request.AnswerSchema)
		copied.EditSchema = cloneRawMessage(request.EditSchema)
		copied.RespondSchema = cloneRawMessage(request.RespondSchema)
		copied.Decisions = cloneRawMessage(request.Decisions)
		copied.Proposed = cloneRawMessage(request.Proposed)
		copied.ProposedPreview = cloneRawMessage(request.ProposedPreview)
		copied.ExpiresAt = cloneRunHistoryTime(request.ExpiresAt)
		cloned = append(cloned, copied)
	}
	return cloned
}

func cloneRunHistoryVerdicts(verdicts []RunHistoryVerdict) []RunHistoryVerdict {
	cloned := make([]RunHistoryVerdict, 0, len(verdicts))
	for _, verdict := range verdicts {
		copied := verdict
		copied.Intent.BlockingIssues = cloneRawMessage(verdict.Intent.BlockingIssues)
		copied.Intent.Criteria = cloneRawMessage(verdict.Intent.Criteria)
		if verdict.Intent.Score != nil {
			score := *verdict.Intent.Score
			copied.Intent.Score = &score
		}
		if verdict.Intent.RouteCauseRank != nil {
			rank := *verdict.Intent.RouteCauseRank
			copied.Intent.RouteCauseRank = &rank
		}
		cloned = append(cloned, copied)
	}
	return cloned
}

func cloneRunHistoryEvents(events []RunHistoryEvent) []RunHistoryEvent {
	cloned := make([]RunHistoryEvent, 0, len(events))
	for _, event := range events {
		copied := event
		if raw, ok := event.Payload.(json.RawMessage); ok {
			copied.Payload = cloneRawMessage(raw)
		}
		cloned = append(cloned, copied)
	}
	return cloned
}

func cloneRunHistoryTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func cloneRunHistoryEpoch(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
