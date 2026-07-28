package acpmock

import (
	"fmt"
	"math"
	"strings"
)

// Validate ensures the turn fixture is usable.
func (t TurnFixture) Validate(path string) error {
	if err := t.Match.Validate(path + ".match"); err != nil {
		return err
	}
	if len(t.Steps) == 0 {
		return fmt.Errorf("acpmock: %s.steps must contain at least one step", path)
	}
	for idx, step := range t.Steps {
		if err := step.Validate(fmt.Sprintf("%s.steps[%d]", path, idx)); err != nil {
			return err
		}
	}
	if stopReason := strings.TrimSpace(t.StopReason); stopReason != "" {
		if err := validatePromptStopReason(path, stopReason); err != nil {
			return err
		}
	}
	if t.Usage != nil {
		if err := t.Usage.Validate(path + ".usage"); err != nil {
			return err
		}
	}
	return nil
}

// Validate rejects impossible aggregate token counts.
func (u TurnUsage) Validate(path string) error {
	if u.InputTokens < 0 {
		return fmt.Errorf("acpmock: %s.input_tokens must be >= 0", path)
	}
	if u.OutputTokens < 0 {
		return fmt.Errorf("acpmock: %s.output_tokens must be >= 0", path)
	}
	if u.TotalTokens != nil && *u.TotalTokens < 0 {
		return fmt.Errorf("acpmock: %s.total_tokens must be >= 0", path)
	}
	if u.InputTokens > math.MaxInt-u.OutputTokens {
		return fmt.Errorf("acpmock: %s total tokens exceed int capacity", path)
	}
	return nil
}
