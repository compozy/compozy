package windowmanager

import "sort"

// TolerantEffectiveKeymap drops dead stored ids while preserving live overrides.
func TolerantEffectiveKeymap(
	overrides map[string]ShortcutBinding,
	bindableIDs BindableIDs,
) (map[string]ShortcutBinding, []ShortcutDiagnostic, error) {
	effective, _, diagnostics, err := tolerantEffectiveKeymap(overrides, bindableIDs)
	return effective, diagnostics, err
}

func tolerantEffectiveKeymap(
	overrides map[string]ShortcutBinding,
	bindableIDs BindableIDs,
) (map[string]ShortcutBinding, map[string]ShortcutBinding, []ShortcutDiagnostic, error) {
	canonical, touchedFamilies, err := canonicalStoredShortcutOverrides(overrides)
	if err != nil {
		return nil, nil, nil, err
	}
	known := make(map[string]ShortcutBinding, len(canonical))
	diagnostics := make([]ShortcutDiagnostic, 0)
	for commandID, binding := range canonical {
		if _, exists := bindableIDs[commandID]; !exists {
			diagnostics = append(diagnostics, ShortcutDiagnostic{
				CommandID: commandID,
				Message:   "stored shortcut references a command that is no longer registered",
			})
			continue
		}
		known[commandID] = binding
	}
	sort.Slice(diagnostics, func(left, right int) bool {
		return diagnostics[left].CommandID < diagnostics[right].CommandID
	})
	effective, err := effectiveKeymap(known, touchedFamilies)
	return effective, canonical, diagnostics, err
}
