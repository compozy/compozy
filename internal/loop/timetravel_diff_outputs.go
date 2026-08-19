package loop

import (
	"context"
	"sort"
)

func diffGenerationOutputs(
	ctx context.Context,
	reader GenerationOutputReader,
	workspaceID WorkspaceID,
	baseRunID RunID,
	base []GenerationOutput,
	againstRunID RunID,
	against []GenerationOutput,
	sharedOnly bool,
) ([]DiffNodeRow, error) {
	baseByKey := indexDiffOutputs(base)
	againstByKey := indexDiffOutputs(against)
	keys := diffOutputKeys(baseByKey, againstByKey, sharedOnly)
	rows := make([]DiffNodeRow, 0, len(keys))
	for _, key := range keys {
		row, include, err := diffGenerationOutputRow(
			ctx, reader, workspaceID, baseRunID, againstRunID, key, baseByKey, againstByKey,
		)
		if err != nil {
			return nil, err
		}
		if include {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func diffOutputKeys(
	baseByKey map[diffOutputKey]GenerationOutput,
	againstByKey map[diffOutputKey]GenerationOutput,
	sharedOnly bool,
) []diffOutputKey {
	keys := make([]diffOutputKey, 0, len(baseByKey)+len(againstByKey))
	seen := make(map[diffOutputKey]struct{}, len(baseByKey)+len(againstByKey))
	for key := range baseByKey {
		if sharedOnly {
			if _, exists := againstByKey[key]; !exists {
				continue
			}
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	for key := range againstByKey {
		if sharedOnly {
			continue
		}
		if _, exists := seen[key]; !exists {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].node == keys[j].node {
			return keys[i].item < keys[j].item
		}
		return keys[i].node < keys[j].node
	})
	return keys
}

func diffGenerationOutputRow(
	ctx context.Context,
	reader GenerationOutputReader,
	workspaceID WorkspaceID,
	baseRunID RunID,
	againstRunID RunID,
	key diffOutputKey,
	baseByKey map[diffOutputKey]GenerationOutput,
	againstByKey map[diffOutputKey]GenerationOutput,
) (DiffNodeRow, bool, error) {
	left, leftOK := baseByKey[key]
	right, rightOK := againstByKey[key]
	if leftOK && rightOK && left.Status == right.Status && left.OutputRef == right.OutputRef {
		carried := left.Generation != right.Generation || baseRunID != againstRunID
		return DiffNodeRow{NodeID: key.node, ItemIndex: key.item, Change: "carried"}, carried, nil
	}
	row := DiffNodeRow{NodeID: key.node, ItemIndex: key.item, Change: diffOutputChange(left, leftOK, right, rightOK)}
	if leftOK {
		value, err := diffOutputValue(ctx, reader, workspaceID, baseRunID, left)
		if err != nil {
			return DiffNodeRow{}, false, err
		}
		row.Base = value
	}
	if rightOK {
		value, err := diffOutputValue(ctx, reader, workspaceID, againstRunID, right)
		if err != nil {
			return DiffNodeRow{}, false, err
		}
		row.Against = value
	}
	return row, true, nil
}

func diffOutputChange(left GenerationOutput, leftOK bool, right GenerationOutput, rightOK bool) string {
	switch {
	case !leftOK:
		return timeTravelKindRerun
	case !rightOK:
		return "skipped"
	case generationOutputSettled(left.Status) && !generationOutputSettled(right.Status):
		return timeTravelKindRerun
	case !generationOutputSettled(left.Status) && generationOutputSettled(right.Status):
		return "skipped"
	default:
		return "changed"
	}
}
