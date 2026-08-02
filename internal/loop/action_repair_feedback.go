package loop

import (
	"encoding/json"
	"fmt"
	"sort"
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
	sort.Slice(failures, func(i, j int) bool {
		if failures[i].NodeID == failures[j].NodeID {
			return failures[i].ItemIndex < failures[j].ItemIndex
		}
		return failures[i].NodeID < failures[j].NodeID
	})
	return failures
}

func runAgentPromptWithRepairFeedback(
	prompt string,
	generation int,
	failures []ActionRepairFailure,
) (string, error) {
	if generation <= 1 || len(failures) == 0 {
		return prompt, nil
	}
	payload, err := json.Marshal(actionRepairFeedback{Generation: generation, Failures: failures})
	if err != nil {
		return "", fmt.Errorf("loop: marshal action repair feedback: %w", err)
	}
	return strings.TrimSpace(prompt) + "\n\nAutomatic generation repair context:\n" + string(payload), nil
}
