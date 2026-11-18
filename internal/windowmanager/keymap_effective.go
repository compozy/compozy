package windowmanager

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
)

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
		if RetiredShortcut(commandID) {
			diagnostics = append(
				diagnostics,
				ShortcutDiagnostic{
					CommandID: commandID,
					Message:   "retired shortcut does nothing; remove it before v0.8.0",
				},
			)
			continue
		}
		resolved, _ := CommandAlias(commandID)
		if _, exists := bindableIDs[resolved]; !exists {
			diagnostics = append(diagnostics, ShortcutDiagnostic{
				CommandID: commandID,
				Message:   "stored shortcut references a command that is no longer registered",
			})
			continue
		}
		known[commandID] = binding
	}
	slices.SortFunc(diagnostics, func(a, b ShortcutDiagnostic) int {
		return cmp.Compare(a.CommandID, b.CommandID)
	})
	effective, err := effectiveKeymap(known, touchedFamilies, overridesDisplaceDefaults)
	return effective, canonical, diagnostics, err
}

// defaultDisplacement decides whether an override's chord may leave another
// action's default binding instead of colliding with it. Nil keeps every
// collision an error.
type defaultDisplacement func(action, chord string) bool

// overridesDisplaceDefaults lets every stored override win: a default that a
// release adds on a chord the operator already bound leaves its action, so an
// upgrade never turns a working keymap into an error.
func overridesDisplaceDefaults(string, string) bool { return true }

// CanonicalShortcutsV2Retaining validates a keymap mutation like
// CanonicalShortcutsV2, except that a chord the retained (stored) overrides
// already took from a daemon default may keep displacing it, whichever action
// claims it now, so moving such a chord between commands never revives the
// default. A chord newly recorded onto another action's default still conflicts.
func CanonicalShortcutsV2Retaining(
	overrides map[string]ShortcutBinding,
	retained map[string]ShortcutBinding,
	bindableIDs BindableIDs,
) (map[string]ShortcutBinding, error) {
	canonical, touchedFamilies, err := canonicalStoredShortcutOverrides(overrides)
	if err != nil {
		return nil, err
	}
	if err := validateBindableShortcutIDs(canonical, bindableIDs); err != nil {
		return nil, err
	}
	retainedCanonical, _, err := canonicalStoredShortcutOverrides(retained)
	if err != nil {
		return nil, err
	}
	held := make(map[string]struct{})
	for action, binding := range retainedCanonical {
		if RetiredShortcut(action) {
			continue
		}
		for _, chord := range binding {
			held[chord] = struct{}{}
		}
	}
	displace := func(_, chord string) bool {
		_, ok := held[chord]
		return ok
	}
	if _, err := effectiveKeymap(canonical, touchedFamilies, displace); err != nil {
		return nil, err
	}
	return canonical, nil
}

func effectiveKeymap(
	canonical map[string]ShortcutBinding,
	touchedFamilies map[string]struct{},
	displace defaultDisplacement,
) (map[string]ShortcutBinding, error) {
	effective := DefaultKeymap()
	for familyAction := range touchedFamilies {
		family, _ := shortcutFamily(familyAction)
		for index := 1; index <= family.max; index++ {
			delete(effective, family.action+"."+strconv.Itoa(index))
		}
	}
	overridden := make(map[string]struct{}, len(canonical))
	claims := make(map[string]string)
	for action, binding := range canonical {
		if RetiredShortcut(action) {
			continue
		}
		resolved, alias := CommandAlias(action)
		if alias {
			if _, exists := canonical[resolved]; exists {
				return nil, fmt.Errorf(
					"shortcut aliases %q and %q both configured: %w",
					action,
					resolved,
					ErrInvalidCommand,
				)
			}
		}
		effective[resolved] = append(ShortcutBinding(nil), binding...)
		overridden[resolved] = struct{}{}
		for _, chord := range binding {
			claims[chord] = resolved
		}
	}
	if displace != nil {
		displaceClaimedDefaults(effective, overridden, claims, displace)
	}
	if err := validateShortcutConflicts(effective); err != nil {
		return nil, err
	}
	return effective, nil
}

// displaceClaimedDefaults removes, from every action still on its default
// binding, the chords an override of another action claims and may displace.
func displaceClaimedDefaults(
	effective map[string]ShortcutBinding,
	overridden map[string]struct{},
	claims map[string]string,
	displace defaultDisplacement,
) {
	for action, binding := range effective {
		if _, isOverride := overridden[action]; isOverride {
			continue
		}
		kept := make(ShortcutBinding, 0, len(binding))
		for _, chord := range binding {
			claimer, claimed := claims[chord]
			if claimed && claimer != action && displace(claimer, chord) {
				continue
			}
			kept = append(kept, chord)
		}
		effective[action] = kept
	}
}
