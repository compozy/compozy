package windowmanager

// ExtensionDefaultShortcut is one ordered bind-if-free extension claim.
type ExtensionDefaultShortcut struct {
	CommandID string
	Chord     string
	Source    string
	Active    bool
}

// ExtensionDefaultStatus explains whether one extension claim is effective.
type ExtensionDefaultStatus struct {
	CommandID    string
	Binding      ShortcutBinding
	Source       string
	Dormant      bool
	ConflictWith string
}

// TolerantEffectiveKeymapWithExtensionDefaults applies the extension tier after
// core defaults and user overrides, preserving every dormant claim for settings.
func TolerantEffectiveKeymapWithExtensionDefaults(
	overrides map[string]ShortcutBinding,
	bindableIDs BindableIDs,
	defaults []ExtensionDefaultShortcut,
) (map[string]ShortcutBinding, []ExtensionDefaultStatus, []ShortcutDiagnostic, error) {
	effective, canonicalOverrides, diagnostics, err := tolerantEffectiveKeymap(overrides, bindableIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	owners := shortcutOwners(effective)
	statuses := make([]ExtensionDefaultStatus, 0, len(defaults))
	for _, claim := range defaults {
		canonicalChord, chordErr := canonicalShortcutChord(claim.Chord)
		if chordErr != nil {
			return nil, nil, nil, chordErr
		}
		status := ExtensionDefaultStatus{
			CommandID: claim.CommandID, Binding: ShortcutBinding{canonicalChord}, Source: claim.Source,
		}
		if !claim.Active {
			status.Dormant = true
			statuses = append(statuses, status)
			continue
		}
		if _, known := bindableIDs[claim.CommandID]; !known {
			status.Dormant = true
			statuses = append(statuses, status)
			continue
		}
		if _, overridden := canonicalOverrides[claim.CommandID]; overridden {
			status.Dormant = true
			status.ConflictWith = claim.CommandID
			statuses = append(statuses, status)
			continue
		}
		if owner, occupied := owners[canonicalChord]; occupied {
			status.Dormant = true
			status.ConflictWith = owner
			statuses = append(statuses, status)
			continue
		}
		effective[claim.CommandID] = append(effective[claim.CommandID], canonicalChord)
		owners[canonicalChord] = claim.CommandID
		statuses = append(statuses, status)
	}
	return effective, statuses, diagnostics, nil
}

func shortcutOwners(shortcuts map[string]ShortcutBinding) map[string]string {
	owners := make(map[string]string)
	for commandID, binding := range shortcuts {
		for _, chord := range binding {
			owners[chord] = commandID
		}
	}
	return owners
}
