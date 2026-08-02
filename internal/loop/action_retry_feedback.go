package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type actionRetryFeedback struct {
	Attempt     int               `json:"attempt"`
	LastFailure ClassifiedFailure `json:"last_failure"`
}

func (r *CoordinatorRunner) actionRetryFailure(
	ctx context.Context,
	run Run,
	meta coordinatorActionRunMetadata,
) (*ClassifiedFailure, error) {
	if meta.Attempt <= 1 || r.attempts == nil {
		return nil, nil
	}
	attempts, err := r.listNodeAttempts(ctx, run)
	if err != nil {
		return nil, err
	}
	var prior *NodeAttempt
	for index := range attempts {
		attempt := &attempts[index]
		if attempt.Generation != meta.Generation || string(attempt.NodeID) != meta.NodeID ||
			attempt.ItemIndex != meta.ItemIndex || attempt.Attempt >= meta.Attempt ||
			attempt.FailureClass == nil {
			continue
		}
		if prior == nil || attempt.Attempt > prior.Attempt {
			prior = attempt
		}
	}
	if prior == nil {
		return nil, nil
	}
	failure := classifiedFailureFromAttempt(*prior)
	return &failure, nil
}

func runAgentPromptWithRetryFeedback(
	prompt string,
	attempt int,
	failure *ClassifiedFailure,
) (string, error) {
	if attempt <= 1 || failure == nil {
		return prompt, nil
	}
	payload, err := json.Marshal(actionRetryFeedback{Attempt: attempt, LastFailure: *failure})
	if err != nil {
		return "", fmt.Errorf("loop: marshal action retry feedback: %w", err)
	}
	return strings.TrimSpace(prompt) + "\n\nAutomatic retry context:\n" + string(payload), nil
}
