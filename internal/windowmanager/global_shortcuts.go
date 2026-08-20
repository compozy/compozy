package windowmanager

import (
	"sort"
	"strings"
)

const DefaultGlobalSummonCommandID = "palette.summon.global"

const DefaultGlobalSummonChord = "meta+shift+Space"

// GlobalShortcutStatus reports the desktop shell's registration result for one intended chord.
type GlobalShortcutStatus string

const (
	GlobalShortcutRegistered       GlobalShortcutStatus = "registered"
	GlobalShortcutFailedInUse      GlobalShortcutStatus = "failed_in_use"
	GlobalShortcutFailedPermission GlobalShortcutStatus = "failed_permission"
	GlobalShortcutUnsupported      GlobalShortcutStatus = "unsupported"
)

// GlobalShortcutRegistration is ephemeral shell-owned state for one command.
type GlobalShortcutRegistration struct {
	CommandID     string               `json:"command_id"`
	IntendedChord string               `json:"intended_chord"`
	ActiveChord   string               `json:"active_chord,omitempty"`
	Status        GlobalShortcutStatus `json:"status"`
	Reason        string               `json:"reason,omitempty"`
	SettingsURL   string               `json:"settings_url,omitempty"`
}

// CloneGlobalShortcutMap returns an ownership-safe copy of intended global bindings.
func CloneGlobalShortcutMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	cloned := make(map[string]string, len(source))
	for commandID, chord := range source {
		cloned[commandID] = chord
	}
	return cloned
}

// CloneGlobalShortcutRegistrations returns an ownership-safe status slice.
func CloneGlobalShortcutRegistrations(
	source []GlobalShortcutRegistration,
) []GlobalShortcutRegistration {
	if source == nil {
		return nil
	}
	return append([]GlobalShortcutRegistration(nil), source...)
}

// CanonicalStoredGlobalShortcuts validates syntax and collisions without a live catalog.
func CanonicalStoredGlobalShortcuts(source map[string]string) (map[string]string, error) {
	return canonicalGlobalShortcuts(source, nil)
}

// CanonicalGlobalShortcuts validates intended bindings against one catalog snapshot.
func CanonicalGlobalShortcuts(
	source map[string]string,
	bindableIDs BindableIDs,
) (map[string]string, error) {
	return canonicalGlobalShortcuts(source, bindableIDs)
}

func canonicalGlobalShortcuts(
	source map[string]string,
	bindableIDs BindableIDs,
) (map[string]string, error) {
	commandIDs := make([]string, 0, len(source))
	for commandID := range source {
		commandIDs = append(commandIDs, commandID)
	}
	sort.Strings(commandIDs)
	canonical := make(map[string]string, len(source))
	owners := make(map[string]string, len(source))
	for _, rawID := range commandIDs {
		commandID := strings.TrimSpace(rawID)
		if commandID == "" || commandID != rawID {
			return nil, &UnknownShortcutIDError{CommandID: rawID}
		}
		if bindableIDs != nil {
			if _, exists := bindableIDs[commandID]; !exists {
				return nil, &UnknownShortcutIDError{CommandID: commandID}
			}
		}
		chord, err := CanonicalShortcutChord(source[rawID])
		if err != nil {
			return nil, err
		}
		if owner, exists := owners[chord]; exists {
			return nil, &ShortcutConflictError{Chord: chord, Owner: owner, Command: commandID}
		}
		owners[chord] = commandID
		canonical[commandID] = chord
	}
	return canonical, nil
}

func validGlobalShortcutStatus(status GlobalShortcutStatus) bool {
	switch status {
	case GlobalShortcutRegistered,
		GlobalShortcutFailedInUse,
		GlobalShortcutFailedPermission,
		GlobalShortcutUnsupported:
		return true
	default:
		return false
	}
}

func normalizeGlobalShortcutRegistrations(
	registrations []GlobalShortcutRegistration,
) ([]GlobalShortcutRegistration, error) {
	result := make([]GlobalShortcutRegistration, len(registrations))
	seen := make(map[string]struct{}, len(registrations))
	for index, registration := range registrations {
		registration.CommandID = strings.TrimSpace(registration.CommandID)
		registration.IntendedChord = strings.TrimSpace(registration.IntendedChord)
		registration.ActiveChord = strings.TrimSpace(registration.ActiveChord)
		registration.Reason = strings.TrimSpace(registration.Reason)
		registration.SettingsURL = strings.TrimSpace(registration.SettingsURL)
		if registration.CommandID == "" || registration.IntendedChord == "" ||
			!validGlobalShortcutStatus(registration.Status) {
			return nil, ErrInvalidCommand
		}
		intendedChord, err := CanonicalShortcutChord(registration.IntendedChord)
		if err != nil {
			return nil, ErrInvalidCommand
		}
		registration.IntendedChord = intendedChord
		if registration.ActiveChord != "" {
			activeChord, activeErr := CanonicalShortcutChord(registration.ActiveChord)
			if activeErr != nil {
				return nil, ErrInvalidCommand
			}
			registration.ActiveChord = activeChord
		}
		if registration.Status == GlobalShortcutRegistered &&
			registration.ActiveChord != registration.IntendedChord {
			return nil, ErrInvalidCommand
		}
		if registration.Status != GlobalShortcutRegistered && registration.Reason == "" {
			return nil, ErrInvalidCommand
		}
		if _, exists := seen[registration.CommandID]; exists {
			return nil, ErrInvalidCommand
		}
		seen[registration.CommandID] = struct{}{}
		result[index] = registration
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].CommandID < result[right].CommandID
	})
	return result, nil
}
