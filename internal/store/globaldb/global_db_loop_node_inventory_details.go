package globaldb

import (
	"context"
	"fmt"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

type nodeInventoryIdentity struct {
	runID      looppkg.RunID
	generation int
	nodeID     looppkg.NodeID
	itemIndex  int
}

func hydrateNodeInventoryItems(
	ctx context.Context,
	queries *sqlcgen.Queries,
	workspaceID looppkg.WorkspaceID,
	items []looppkg.NodeInventoryItem,
) ([]looppkg.NodeInventoryItem, error) {
	indexesByRun := make(map[looppkg.RunID][]int)
	for index := range items {
		indexesByRun[items[index].LoopRunID] = append(indexesByRun[items[index].LoopRunID], index)
	}
	for runID, indexes := range indexesByRun {
		if err := hydrateNodeInventoryRun(ctx, queries, workspaceID, runID, indexes, items); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func hydrateNodeInventoryRun(
	ctx context.Context,
	queries *sqlcgen.Queries,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
	indexes []int,
	items []looppkg.NodeInventoryItem,
) error {
	needsControls, needsWaits, retryGenerations := nodeInventoryDetailNeeds(indexes, items)
	if needsControls {
		if err := hydrateNodeInventoryControls(ctx, queries, workspaceID, runID, indexes, items); err != nil {
			return err
		}
	}
	if needsWaits {
		if err := hydrateNodeInventoryWaits(ctx, queries, workspaceID, runID, indexes, items); err != nil {
			return err
		}
	}
	if len(retryGenerations) > 0 {
		if err := hydrateNodeInventoryRetries(
			ctx, queries, workspaceID, runID, retryGenerations, indexes, items,
		); err != nil {
			return err
		}
	}
	return nil
}

func nodeInventoryDetailNeeds(
	indexes []int,
	items []looppkg.NodeInventoryItem,
) (bool, bool, map[int]struct{}) {
	needsControls := false
	needsWaits := false
	retryGenerations := make(map[int]struct{})
	for _, index := range indexes {
		switch items[index].State {
		case looppkg.NodeInventoryWaiting:
			needsWaits = true
		case looppkg.NodeInventoryQuarantined, looppkg.NodeInventoryAttention:
			needsControls = true
		case looppkg.NodeInventoryRetrying:
			retryGenerations[items[index].Generation] = struct{}{}
		}
	}
	return needsControls, needsWaits, retryGenerations
}

func hydrateNodeInventoryControls(
	ctx context.Context,
	queries *sqlcgen.Queries,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
	indexes []int,
	items []looppkg.NodeInventoryItem,
) error {
	rows, err := queries.ListLoopNodeControls(ctx, sqlcgen.ListLoopNodeControlsParams{
		WorkspaceID: string(workspaceID),
		LoopRunID:   string(runID),
	})
	if err != nil {
		return fmt.Errorf("store: hydrate loop node inventory controls: %w", err)
	}
	controls := make(map[looppkg.NodeID]looppkg.NodeControl, len(rows))
	for _, row := range rows {
		control, mapErr := loopNodeControlFromGenerated(row)
		if mapErr != nil {
			return mapErr
		}
		controls[control.NodeID] = control
	}
	for _, index := range indexes {
		if items[index].State != looppkg.NodeInventoryQuarantined &&
			items[index].State != looppkg.NodeInventoryAttention {
			continue
		}
		if control, ok := controls[items[index].NodeID]; ok {
			items[index].Control = &control
		}
	}
	return nil
}

func hydrateNodeInventoryWaits(
	ctx context.Context,
	queries *sqlcgen.Queries,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
	indexes []int,
	items []looppkg.NodeInventoryItem,
) error {
	rows, err := queries.ListLoopNodeWaits(ctx, sqlcgen.ListLoopNodeWaitsParams{
		WorkspaceID: string(workspaceID),
		LoopRunID:   string(runID),
	})
	if err != nil {
		return fmt.Errorf("store: hydrate loop node inventory waits: %w", err)
	}
	waits := make(map[nodeInventoryIdentity]looppkg.NodeWait, len(rows))
	for _, row := range rows {
		wait, mapErr := loopNodeWaitFromGenerated(row)
		if mapErr != nil {
			return mapErr
		}
		waits[nodeInventoryIdentity{
			runID: wait.LoopRunID, generation: wait.Generation,
			nodeID: wait.NodeID, itemIndex: wait.ItemIndex,
		}] = wait
	}
	for _, index := range indexes {
		if items[index].State != looppkg.NodeInventoryWaiting {
			continue
		}
		identity := nodeInventoryItemIdentity(items[index])
		if wait, ok := waits[identity]; ok {
			items[index].Wait = &wait
		}
	}
	return nil
}

func hydrateNodeInventoryRetries(
	ctx context.Context,
	queries *sqlcgen.Queries,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
	generations map[int]struct{},
	indexes []int,
	items []looppkg.NodeInventoryItem,
) error {
	attemptRows, err := queries.ListLoopNodeAttempts(ctx, sqlcgen.ListLoopNodeAttemptsParams{
		WorkspaceID: string(workspaceID),
		LoopRunID:   string(runID),
	})
	if err != nil {
		return fmt.Errorf("store: hydrate loop node inventory attempts: %w", err)
	}
	attempts := make(map[nodeInventoryAttemptIdentity]looppkg.NodeAttempt, len(attemptRows))
	for _, row := range attemptRows {
		attempt, mapErr := loopNodeAttemptFromGenerated(row)
		if mapErr != nil {
			return mapErr
		}
		attempts[nodeAttemptIdentity(attempt)] = attempt
	}
	outputs := make(map[nodeInventoryIdentity]looppkg.GenerationOutput)
	for generation := range generations {
		rows, queryErr := queries.ListLoopGenerationOutputs(ctx, sqlcgen.ListLoopGenerationOutputsParams{
			LoopRunID: string(runID), Generation: int64(generation), WorkspaceID: string(workspaceID),
		})
		if queryErr != nil {
			return fmt.Errorf("store: hydrate loop node inventory outputs: %w", queryErr)
		}
		for _, row := range rows {
			output, mapErr := generationOutputFromGenerated(row)
			if mapErr != nil {
				return mapErr
			}
			outputs[nodeOutputIdentity(runID, output)] = output
		}
	}
	for _, index := range indexes {
		if items[index].State != looppkg.NodeInventoryRetrying {
			continue
		}
		identity := nodeInventoryItemIdentity(items[index])
		output, ok := outputs[identity]
		if !ok {
			continue
		}
		items[index].Output = &output
		if attempt, found := attempts[nodeInventoryAttemptIdentity{
			nodeInventoryIdentity: identity,
			attempt:               output.Attempt,
		}]; found {
			items[index].Attempt = &attempt
		}
	}
	return nil
}

type nodeInventoryAttemptIdentity struct {
	nodeInventoryIdentity
	attempt int
}

func nodeInventoryItemIdentity(item looppkg.NodeInventoryItem) nodeInventoryIdentity {
	return nodeInventoryIdentity{
		runID: item.LoopRunID, generation: item.Generation,
		nodeID: item.NodeID, itemIndex: item.ItemIndex,
	}
}

func nodeOutputIdentity(runID looppkg.RunID, output looppkg.GenerationOutput) nodeInventoryIdentity {
	return nodeInventoryIdentity{
		runID: runID, generation: output.Generation,
		nodeID: looppkg.NodeID(output.NodeID), itemIndex: output.ItemIndex,
	}
}

func nodeAttemptIdentity(attempt looppkg.NodeAttempt) nodeInventoryAttemptIdentity {
	return nodeInventoryAttemptIdentity{
		nodeInventoryIdentity: nodeInventoryIdentity{
			runID: attempt.LoopRunID, generation: attempt.Generation,
			nodeID: attempt.NodeID, itemIndex: attempt.ItemIndex,
		},
		attempt: attempt.Attempt,
	}
}
