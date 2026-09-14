package extensionpkg

import (
	"cmp"
	"slices"

	"github.com/compozy/compozy/internal/api/contract"
)

// DescribeInputState exposes configuration presence and activity without values or secret references.
func DescribeInputState(
	manifest *Manifest,
	state InputState,
	getenv func(string) string,
) []contract.ExtensionInputStatePayload {
	result := make([]contract.ExtensionInputStatePayload, 0, len(state.Values))
	declared := make(map[string]struct{})
	if manifest != nil {
		for _, input := range manifest.Inputs {
			declared[input.ID] = struct{}{}
			_, _, present, err := effectiveInputValue(input, state, getenv)
			active := true
			if record, found := state.Values[input.ID]; found {
				active = record.Active
			}
			result = append(result, contract.ExtensionInputStatePayload{
				ID: input.ID, Type: input.Type, Set: present && err == nil, Active: active,
			})
		}
	}
	for id, record := range state.Values {
		if _, found := declared[id]; found {
			continue
		}
		result = append(result, contract.ExtensionInputStatePayload{
			ID: id, Type: record.Type, Set: len(record.Value) > 0 || record.SecretRef != "", Active: false,
		})
	}
	slices.SortFunc(
		result,
		func(left, right contract.ExtensionInputStatePayload) int { return cmp.Compare(left.ID, right.ID) },
	)
	return result
}
