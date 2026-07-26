package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/task"
)

const circuitBreakerReasonCode = "circuit_breaker"

func (r *CoordinatorRunner) terminalForFailedGeneration(
	ctx context.Context,
	run Run,
	generation int,
	noProgressWindow int,
	graph dsl.Graph,
	outputs []GenerationOutput,
	failed GenerationOutput,
) (*task.CoordinatorTerminal, error) {
	stalled, err := r.stalledBlockingIssueTerminal(ctx, run.ID, generation, noProgressWindow, outputs)
	if err != nil {
		return nil, err
	}
	if stalled != nil {
		return stalled, nil
	}
	terminal := failedOutputTerminal(failed)
	if terminal.Status != string(StatusFailed) {
		return &terminal, nil
	}
	history, err := r.generationFailureHistory(
		ctx,
		run.ID,
		generation,
		outputs,
		LoopFailureBreakerLimit,
	)
	if err != nil {
		return nil, err
	}
	if perNodeFailureLimitReached(history) ||
		(run.IterationCap <= 0 && graphHasWatchSource(graph) && failedGenerationLimitReached(history)) {
		breaker := circuitBreakerTerminal()
		return &breaker, nil
	}
	return &terminal, nil
}

func failedOutputTerminal(output GenerationOutput) task.CoordinatorTerminal {
	status := StatusFailed
	cause := TransitionCauseContract
	reasonCode := "node_failed"
	if explicitDependencyBlocker(output.OutputRef) {
		status = StatusBlocked
		cause = TransitionCauseContract
		reasonCode = output.OutputRef
	}
	return task.CoordinatorTerminal{
		Status:     string(status),
		Cause:      string(cause),
		ReasonCode: reasonCode,
	}
}

func (r *CoordinatorRunner) generationFailureHistory(
	ctx context.Context,
	runID RunID,
	generation int,
	current []GenerationOutput,
	limit int,
) ([][]GenerationOutput, error) {
	if generation <= 0 || limit <= 0 {
		return nil, nil
	}
	history := make([][]GenerationOutput, 0, limit)
	history = append(history, current)
	for offset := 1; offset < limit; offset++ {
		previousGeneration := generation - offset
		if previousGeneration <= 0 {
			break
		}
		previous, err := r.outputs.ListGenerationOutputs(ctx, runID, previousGeneration)
		if err != nil {
			return nil, fmt.Errorf(
				"loop: list generation %d outputs for failure breaker: %w",
				previousGeneration,
				err,
			)
		}
		history = append(history, previous)
	}
	return history, nil
}

func perNodeFailureLimitReached(history [][]GenerationOutput) bool {
	if len(history) < LoopFailureBreakerLimit {
		return false
	}
	failing := failedNodeIDs(history[0])
	for _, outputs := range history[1:LoopFailureBreakerLimit] {
		previous := failedNodeIDs(outputs)
		for nodeID := range failing {
			if _, ok := previous[nodeID]; !ok {
				delete(failing, nodeID)
			}
		}
		if len(failing) == 0 {
			return false
		}
	}
	return len(failing) > 0
}

func failedGenerationLimitReached(history [][]GenerationOutput) bool {
	if len(history) < LoopFailureBreakerLimit {
		return false
	}
	for _, outputs := range history[:LoopFailureBreakerLimit] {
		if len(failedNodeIDs(outputs)) == 0 {
			return false
		}
	}
	return true
}

func failedNodeIDs(outputs []GenerationOutput) map[string]struct{} {
	failed := make(map[string]struct{})
	for _, output := range outputs {
		if output.Status != generationOutputFailed {
			continue
		}
		nodeID := strings.TrimSpace(output.NodeID)
		if nodeID != "" {
			failed[nodeID] = struct{}{}
		}
	}
	return failed
}

func graphHasWatchSource(graph dsl.Graph) bool {
	for _, node := range graph.Nodes {
		if node.Class != dsl.NodeClassSource {
			continue
		}
		switch dsl.SourceKind(node.Kind) {
		case dsl.SourceWatchSource, dsl.SourceWatchEvents:
			return true
		}
	}
	return false
}

func circuitBreakerTerminal() task.CoordinatorTerminal {
	return task.CoordinatorTerminal{
		Status:     string(StatusStalled),
		Cause:      string(TransitionCauseNoProgress),
		ReasonCode: circuitBreakerReasonCode,
	}
}

func (r *CoordinatorRunner) stalledBlockingIssueTerminal(
	ctx context.Context,
	runID RunID,
	generation int,
	window int,
	outputs []GenerationOutput,
) (*task.CoordinatorTerminal, error) {
	current := blockingIssueSignature(outputs)
	if len(current) == 0 {
		return nil, nil
	}
	if window <= 0 {
		return nil, nil
	}
	if window == 1 {
		return stalledBlockingIssuesTerminal(), nil
	}
	for offset := 1; offset < window; offset++ {
		previousGeneration := generation - offset
		if previousGeneration <= 0 {
			return nil, nil
		}
		previous, err := r.outputs.ListGenerationOutputs(ctx, runID, previousGeneration)
		if err != nil {
			return nil, err
		}
		if !sameStringSet(current, blockingIssueSignature(previous)) {
			return nil, nil
		}
	}
	return stalledBlockingIssuesTerminal(), nil
}

func stalledBlockingIssuesTerminal() *task.CoordinatorTerminal {
	return &task.CoordinatorTerminal{
		Status:     string(StatusStalled),
		Cause:      string(TransitionCauseNoProgress),
		ReasonCode: blockingIssuesRepeatedCode,
	}
}

func blockingIssueSignature(outputs []GenerationOutput) []string {
	seen := make(map[string]struct{})
	for _, output := range outputs {
		if output.Status != generationOutputFailed {
			continue
		}
		for _, id := range blockingIssueIDs(output.OutputRef) {
			seen[id] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func blockingIssueIDs(value string) []string {
	var payload struct {
		BlockingIssues []struct {
			ID string `json:"id"`
		} `json:"blocking_issues"`
	}
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return nil
	}
	ids := make([]string, 0, len(payload.BlockingIssues))
	seen := make(map[string]struct{}, len(payload.BlockingIssues))
	for _, issue := range payload.BlockingIssues {
		id := strings.TrimSpace(issue.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func sameStringSet(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if left[idx] != right[idx] {
			return false
		}
	}
	return true
}

func explicitDependencyBlocker(value string) bool {
	const (
		dependencyMissing   = "dependency_missing"
		credentialMissing   = "credential_missing" // #nosec G101 -- public reason code, not a credential.
		resourceUnreachable = "resource_unreachable"
	)

	switch strings.TrimSpace(value) {
	case dependencyMissing, credentialMissing, resourceUnreachable:
		return true
	default:
		return false
	}
}

func failureReasonCode(value string) string {
	var payload struct {
		ReasonCode string `json:"reason_code"`
		Code       string `json:"code"`
	}
	if err := json.Unmarshal([]byte(value), &payload); err == nil {
		if strings.TrimSpace(payload.ReasonCode) != "" {
			return strings.TrimSpace(payload.ReasonCode)
		}
		if strings.TrimSpace(payload.Code) != "" {
			return strings.TrimSpace(payload.Code)
		}
	}
	return ""
}

func graphDependencies(graph dsl.Graph) map[dsl.NodeID][]dsl.NodeID {
	dependencies := make(map[dsl.NodeID][]dsl.NodeID, len(graph.Nodes))
	for _, node := range graph.Nodes {
		dependencies[node.ID] = nil
	}
	for _, edge := range graph.Edges {
		dependencies[edge.To] = append(dependencies[edge.To], edge.From)
	}
	return dependencies
}

func graphDependents(graph dsl.Graph) map[dsl.NodeID][]dsl.NodeID {
	dependents := make(map[dsl.NodeID][]dsl.NodeID, len(graph.Nodes))
	for _, node := range graph.Nodes {
		dependents[node.ID] = nil
	}
	for _, edge := range graph.Edges {
		dependents[edge.From] = append(dependents[edge.From], edge.To)
	}
	return dependents
}

func coordinatorRunID(loopRunID RunID, generation int) string {
	return fmt.Sprintf("run.loop.%s.g%d.coordinator", loopRunID, generation)
}

func coordinatorIdempotencyKey(loopRunID RunID, generation int) string {
	return fmt.Sprintf("loop.coordinator.%s.%d", loopRunID, generation)
}

func coordinatorNodeTaskID(
	loopRunID RunID,
	generation int,
	nodeID dsl.NodeID,
	itemIndex int,
) string {
	return fmt.Sprintf("loop.%s.g%d.node.%s.%d", loopRunID, generation, nodeID, itemIndex)
}

func coordinatorNodeRunID(
	loopRunID RunID,
	generation int,
	nodeID dsl.NodeID,
	itemIndex int,
) string {
	return fmt.Sprintf("run.loop.%s.g%d.node.%s.%d", loopRunID, generation, nodeID, itemIndex)
}

func coordinatorNodeIdempotencyKey(
	loopRunID RunID,
	generation int,
	nodeID dsl.NodeID,
	itemIndex int,
) string {
	return fmt.Sprintf("loop.node.%s.%d.%s.%d", loopRunID, generation, nodeID, itemIndex)
}
