package task

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/reasoning"
	speedpkg "github.com/compozy/compozy/internal/speed"
)

// ACPOptionSelection identifies one provider-advertised ACP option value.
// Exactly one of ValueID or BoolValue must be set.
type ACPOptionSelection struct {
	ID        string `json:"id"`
	ValueID   string `json:"value_id,omitempty"`
	BoolValue *bool  `json:"bool_value,omitempty"`
}

func normalizeProfileRuntime(
	reasoningEffort string,
	speed speedpkg.Speed,
	selections []ACPOptionSelection,
	path string,
) (string, speedpkg.Speed, []ACPOptionSelection, error) {
	reasoningEffort = strings.TrimSpace(reasoningEffort)
	speed = speedpkg.Speed(strings.TrimSpace(string(speed)))
	normalizedSelections, err := normalizeProfileACPOptionSelections(selections, path+".acp_options")
	if err != nil {
		return "", "", nil, err
	}
	return reasoningEffort, speed, normalizedSelections, nil
}

func validateProfileRuntime(
	reasoningEffort string,
	speed speedpkg.Speed,
	selections []ACPOptionSelection,
	path string,
) error {
	if effort := strings.TrimSpace(reasoningEffort); effort != "" && !reasoning.IsValid(effort) {
		return fmt.Errorf(
			"%w: %s.reasoning_effort %q is invalid; expected %s",
			ErrValidation,
			path,
			effort,
			strings.Join(reasoning.Values(), ", "),
		)
	}
	if value := strings.TrimSpace(string(speed)); value != "" {
		if _, err := speedpkg.Parse(value); err != nil {
			return fmt.Errorf("%w: %s.speed: %w", ErrValidation, path, err)
		}
	}
	_, err := normalizeProfileACPOptionSelections(selections, path+".acp_options")
	return err
}

func normalizeProfileACPOptionSelections(
	selections []ACPOptionSelection,
	path string,
) ([]ACPOptionSelection, error) {
	if len(selections) == 0 {
		return nil, nil
	}
	normalized := make([]ACPOptionSelection, 0, len(selections))
	seen := make(map[string]struct{}, len(selections))
	for index, selection := range selections {
		candidate := ACPOptionSelection{
			ID:      strings.TrimSpace(selection.ID),
			ValueID: strings.TrimSpace(selection.ValueID),
		}
		if candidate.ID == "" {
			return nil, fmt.Errorf("%w: %s[%d].id is required", ErrValidation, path, index)
		}
		if (candidate.ValueID == "") == (selection.BoolValue == nil) {
			return nil, fmt.Errorf(
				"%w: %s[%d] requires exactly one value_id or bool_value",
				ErrValidation,
				path,
				index,
			)
		}
		if _, exists := seen[candidate.ID]; exists {
			return nil, fmt.Errorf(
				"%w: %s[%d] selects option %q more than once",
				ErrValidation,
				path,
				index,
				candidate.ID,
			)
		}
		seen[candidate.ID] = struct{}{}
		if selection.BoolValue != nil {
			candidate.BoolValue = new(*selection.BoolValue)
		}
		normalized = append(normalized, candidate)
	}
	slices.SortFunc(normalized, func(left ACPOptionSelection, right ACPOptionSelection) int {
		return cmp.Compare(left.ID, right.ID)
	})
	return normalized, nil
}
