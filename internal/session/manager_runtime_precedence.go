package session

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

// applyResolvedRuntimeDefaults materializes the effective session runtime in
// provider/project -> agent -> session order. Prompt selections are rebuilt as
// session specs and therefore form the final overlay through the same path.
func (s *sessionStartSpec) applyResolvedRuntimeDefaults(resolved compozyconfig.ResolvedAgent) error {
	if s == nil {
		return nil
	}
	if s.speed == "" {
		s.speed = resolved.SpeedValue()
	}
	normalizedSpeed, err := normalizeRequestedSpeed(s.speed)
	if err != nil {
		return fmt.Errorf("session: resolve runtime speed: %w", err)
	}
	s.speed = normalizedSpeed

	agentOptions := ACPOptionSelectionsFromConfig(resolved.ACPOptionsValue())
	merged, err := MergeRuntimeACPOptions(agentOptions, s.acpOptions)
	if err != nil {
		return fmt.Errorf("session: resolve runtime ACP options: %w", err)
	}
	s.acpOptions = merged
	return nil
}

// MergeRuntimeACPOptions merges base ACP options with explicit runtime overrides in option ID order.
func MergeRuntimeACPOptions(
	base []acp.SessionConfigOptionSelection,
	overrides []acp.SessionConfigOptionSelection,
) ([]acp.SessionConfigOptionSelection, error) {
	if len(base) == 0 && len(overrides) == 0 {
		return nil, nil
	}
	merged := make(map[string]acp.SessionConfigOptionSelection, len(base)+len(overrides))
	for _, layer := range [][]acp.SessionConfigOptionSelection{base, overrides} {
		normalized, err := acp.NormalizeSessionConfigOptionSelections(layer)
		if err != nil {
			return nil, err
		}
		for _, selection := range normalized {
			merged[selection.ID] = selection
		}
	}
	result := make([]acp.SessionConfigOptionSelection, 0, len(merged))
	for _, selection := range merged {
		result = append(result, selection)
	}
	slices.SortFunc(result, func(left acp.SessionConfigOptionSelection, right acp.SessionConfigOptionSelection) int {
		return cmp.Compare(left.ID, right.ID)
	})
	return result, nil
}
