package goal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop"
)

func authoritativeGoalStructuredResult(
	result loop.ActionPromptResult,
	status string,
) (json.RawMessage, error) {
	object := make(map[string]json.RawMessage)
	candidate, candidateErr := loop.ActionStructuredCandidate(result)
	if candidateErr == nil {
		if err := json.Unmarshal(candidate, &object); err != nil {
			return nil, fmt.Errorf("goal: decode structured action result: %w", err)
		}
	} else if len(bytes.TrimSpace(result.Structured)) > 0 {
		return nil, fmt.Errorf("goal: load structured action result: %w", candidateErr)
	}

	statusValue, err := json.Marshal(strings.TrimSpace(status))
	if err != nil {
		return nil, fmt.Errorf("goal: encode authoritative status: %w", err)
	}
	object["status"] = statusValue
	encoded, err := json.Marshal(object)
	if err != nil {
		return nil, fmt.Errorf("goal: encode structured action result: %w", err)
	}
	return encoded, nil
}

// A completion or blocked claim retains normal judge handling even with live waits.
func hasTerminalClaim(result loop.ActionPromptResult) bool {
	candidate, err := loop.ActionStructuredCandidate(result)
	if err != nil {
		return false
	}
	var claim struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(candidate, &claim); err != nil {
		return false
	}
	return claim.Status == goalStatusComplete || claim.Status == goalStatusBlocked
}
