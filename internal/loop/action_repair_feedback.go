package loop

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// ActionRepairFailure is one classified failure supplied to a repair generation.
type ActionRepairFailure struct {
	NodeID      string             `json:"node_id"`
	ItemIndex   int                `json:"item_index"`
	Disposition AttemptDisposition `json:"disposition"`
	Failure     ClassifiedFailure  `json:"failure"`
}

type actionRepairFeedback struct {
	Generation int                   `json:"generation"`
	Failures   []ActionRepairFailure `json:"failures"`
}

func actionRepairFailures(history GenerationHistory) []ActionRepairFailure {
	if history.Previous == nil {
		return nil
	}
	failures := make([]ActionRepairFailure, 0)
	for nodeID, items := range history.Previous.Nodes {
		for itemIndex, projection := range items {
			if projection.Failure == nil || projection.Disposition != AttemptEscalated {
				continue
			}
			failures = append(failures, ActionRepairFailure{
				NodeID: nodeID, ItemIndex: itemIndex,
				Disposition: projection.Disposition, Failure: *projection.Failure,
			})
		}
	}
	slices.SortFunc(failures, func(a, b ActionRepairFailure) int {
		return cmp.Or(cmp.Compare(a.NodeID, b.NodeID), cmp.Compare(a.ItemIndex, b.ItemIndex))
	})
	return failures
}

func runAgentPromptWithRepairFeedback(
	prompt string,
	generation int,
	failures []ActionRepairFailure,
) (string, error) {
	if generation <= 1 {
		return prompt, nil
	}
	escalated := make([]ActionRepairFailure, 0, len(failures))
	for _, failure := range failures {
		if failure.Disposition == AttemptEscalated {
			escalated = append(escalated, failure)
		}
	}
	if len(escalated) == 0 {
		return prompt, nil
	}
	payload, err := json.Marshal(actionRepairFeedback{Generation: generation, Failures: escalated})
	if err != nil {
		return "", fmt.Errorf("loop: marshal action repair feedback: %w", err)
	}
	return strings.TrimSpace(prompt) + "\n\nAutomatic generation repair context:\n" + string(payload), nil
}
