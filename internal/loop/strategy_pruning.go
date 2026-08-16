package loop

import (
	"slices"
	"strconv"

	"github.com/compozy/compozy/internal/loop/dsl"
)

const (
	branchPrunedIntentPayloadLimit = 12 * 1024
	branchPrunedIntentOverhead     = 256
)

func branchPrunedEventIntents(
	fanOutID dsl.NodeID,
	itemIndexes []int,
	reason string,
) []GenerationLifecycleEventIntent {
	indexes := append([]int(nil), itemIndexes...)
	slices.Sort(indexes)
	indexes = slices.Compact(indexes)
	intents := make([]GenerationLifecycleEventIntent, 0, 1)
	current := make([]int, 0, len(indexes))
	encodedSize := branchPrunedIntentOverhead
	for _, itemIndex := range indexes {
		itemSize := len(strconv.Itoa(itemIndex))
		if len(current) > 0 {
			itemSize++
		}
		if len(current) > 0 && encodedSize+itemSize+2 > branchPrunedIntentPayloadLimit {
			intents = append(intents, GenerationLifecycleEventIntent{
				Kind: GenerationLifecycleEventBranchPruned, NodeID: string(fanOutID),
				ItemIndexes: current, Reason: reason,
			})
			current = make([]int, 0, len(indexes)-len(current))
			encodedSize = branchPrunedIntentOverhead
			itemSize = len(strconv.Itoa(itemIndex))
		}
		current = append(current, itemIndex)
		encodedSize += itemSize
	}
	if len(current) > 0 {
		intents = append(intents, GenerationLifecycleEventIntent{
			Kind: GenerationLifecycleEventBranchPruned, NodeID: string(fanOutID),
			ItemIndexes: current, Reason: reason,
		})
	}
	return intents
}
