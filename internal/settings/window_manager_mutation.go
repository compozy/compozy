package settings

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/cmdpalette"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/windowmanager"
)

func (s *service) windowManagerBindableIDs(
	ctx context.Context,
	workspaceID string,
) (windowmanager.BindableIDs, error) {
	if s.cmdPalette == nil || workspaceID == "" {
		return windowmanager.DefaultBindableIDs(), nil
	}
	catalog, err := s.cmdPalette.Catalog(ctx, cmdpalette.CatalogRequest{
		ProfileLens: cmdpalette.ScopedProfileLens(cmdpalette.DefaultProfileLensID, "default"),
		WorkspaceID: cmdpalette.WorkspaceID(workspaceID),
	})
	if err != nil {
		return nil, fmt.Errorf("settings: load command catalog for shortcut validation: %w", err)
	}
	ids := make([]string, 0, len(catalog.Commands))
	for _, command := range catalog.Commands {
		ids = append(ids, string(command.ID))
	}
	return catalogBindableIDs(ids), nil
}

func catalogBindableIDs(ids []string) windowmanager.BindableIDs {
	result := windowmanager.NewBindableIDs(ids)
	result[windowmanager.DefaultGlobalSummonCommandID] = struct{}{}
	return result
}

func normalizeShortcutMutation(
	current map[string]windowmanager.ShortcutBinding,
	desired map[string]windowmanager.ShortcutBinding,
	bindableIDs windowmanager.BindableIDs,
	overwrite bool,
) (map[string]windowmanager.ShortcutBinding, error) {
	resolved := windowmanager.CloneShortcutMap(desired)
	currentEffective, _, err := windowmanager.TolerantEffectiveKeymap(current, bindableIDs)
	if err != nil {
		return nil, unprocessableError(err)
	}
	for attempts := 0; attempts <= len(bindableIDs); attempts++ {
		_, canonicalErr := windowmanager.CanonicalShortcutsV2(resolved, bindableIDs)
		if canonicalErr == nil {
			return resolved, nil
		}
		var conflict *windowmanager.ShortcutConflictError
		if !errors.As(canonicalErr, &conflict) {
			return nil, unprocessableError(canonicalErr)
		}
		loser, hasExistingOwner := shortcutOverwriteLoser(currentEffective, conflict)
		if hasExistingOwner {
			conflict = semanticShortcutConflict(conflict, loser)
		}
		if !overwrite {
			return nil, conflictError(conflict)
		}
		if !hasExistingOwner {
			return nil, conflictError(conflict)
		}
		removeShortcutChord(resolved, loser, conflict.Chord)
	}
	return nil, errors.New("settings: shortcut overwrite did not converge")
}

func normalizeGlobalShortcutMutation(
	current map[string]string,
	desired map[string]string,
	bindableIDs windowmanager.BindableIDs,
	overwrite bool,
) (map[string]string, error) {
	resolved := windowmanager.CloneGlobalShortcutMap(desired)
	currentCanonical, err := windowmanager.CanonicalGlobalShortcuts(current, bindableIDs)
	if err != nil {
		return nil, unprocessableError(err)
	}
	for attempts := 0; attempts <= len(bindableIDs); attempts++ {
		canonical, canonicalErr := windowmanager.CanonicalGlobalShortcuts(resolved, bindableIDs)
		if canonicalErr == nil {
			return canonical, nil
		}
		var conflict *windowmanager.ShortcutConflictError
		if !errors.As(canonicalErr, &conflict) {
			return nil, unprocessableError(canonicalErr)
		}
		loser, exists := globalShortcutOverwriteLoser(currentCanonical, conflict)
		if exists {
			conflict = semanticShortcutConflict(conflict, loser)
		}
		if !overwrite || !exists {
			return nil, conflictError(conflict)
		}
		delete(resolved, loser)
	}
	return nil, errors.New("settings: global shortcut overwrite did not converge")
}

func globalShortcutOverwriteLoser(
	current map[string]string,
	conflict *windowmanager.ShortcutConflictError,
) (string, bool) {
	ownerHadChord := current[conflict.Owner] == conflict.Chord
	commandHadChord := current[conflict.Command] == conflict.Chord
	switch {
	case ownerHadChord && !commandHadChord:
		return conflict.Owner, true
	case commandHadChord && !ownerHadChord:
		return conflict.Command, true
	default:
		return "", false
	}
}

func semanticShortcutConflict(
	conflict *windowmanager.ShortcutConflictError,
	loser string,
) *windowmanager.ShortcutConflictError {
	claimant := conflict.Owner
	if loser == claimant {
		claimant = conflict.Command
	}
	return &windowmanager.ShortcutConflictError{
		Chord: conflict.Chord, Owner: loser, Command: claimant,
	}
}

func shortcutOverwriteLoser(
	current map[string]windowmanager.ShortcutBinding,
	conflict *windowmanager.ShortcutConflictError,
) (string, bool) {
	ownerHadChord := shortcutBindingContains(current[conflict.Owner], conflict.Chord)
	commandHadChord := shortcutBindingContains(current[conflict.Command], conflict.Chord)
	switch {
	case ownerHadChord && !commandHadChord:
		return conflict.Owner, true
	case commandHadChord && !ownerHadChord:
		return conflict.Command, true
	default:
		return "", false
	}
}

func removeShortcutChord(
	shortcuts map[string]windowmanager.ShortcutBinding,
	commandID string,
	chord string,
) {
	binding, exists := shortcuts[commandID]
	if !exists {
		binding = windowmanager.DefaultKeymap()[commandID]
	}
	filtered := make(windowmanager.ShortcutBinding, 0, len(binding))
	for _, candidate := range binding {
		if !strings.EqualFold(candidate, chord) {
			filtered = append(filtered, candidate)
		}
	}
	shortcuts[commandID] = filtered
}

func shortcutBindingContains(binding windowmanager.ShortcutBinding, chord string) bool {
	return slices.Contains(binding, chord)
}

func normalizeAliasMutation(
	current map[string]string,
	desired map[string]string,
	bindableIDs windowmanager.BindableIDs,
	overwrite bool,
) (map[string]string, error) {
	aliases := cloneAliases(desired)
	commandIDs := make([]string, 0, len(aliases))
	for commandID := range aliases {
		commandIDs = append(commandIDs, commandID)
	}
	sort.Strings(commandIDs)
	owners := make(map[string]string, len(aliases))
	for _, commandID := range commandIDs {
		alias := aliases[commandID]
		if _, exists := bindableIDs[commandID]; !exists {
			return nil, unprocessableError(&windowmanager.UnknownShortcutIDError{CommandID: commandID})
		}
		if err := compozyconfig.ValidateCmdPaletteAlias(alias); err != nil {
			return nil, unprocessableError(&InvalidAliasError{CommandID: commandID})
		}
		owner, conflict := owners[alias]
		if !conflict {
			owners[alias] = commandID
			continue
		}
		aliasConflict := &AliasConflictError{Alias: alias, Owner: owner, Command: commandID}
		loser, hasExistingOwner := aliasOverwriteLoser(current, aliasConflict)
		if hasExistingOwner {
			aliasConflict = semanticAliasConflict(aliasConflict, loser)
		}
		if !overwrite {
			return nil, conflictError(aliasConflict)
		}
		if !hasExistingOwner {
			return nil, conflictError(aliasConflict)
		}
		delete(aliases, loser)
		owners[alias] = aliasOverwriteWinner(aliasConflict, loser)
	}
	return aliases, nil
}

func semanticAliasConflict(conflict *AliasConflictError, loser string) *AliasConflictError {
	claimant := conflict.Owner
	if loser == claimant {
		claimant = conflict.Command
	}
	return &AliasConflictError{Alias: conflict.Alias, Owner: loser, Command: claimant}
}

func aliasOverwriteLoser(current map[string]string, conflict *AliasConflictError) (string, bool) {
	ownerHadAlias := current[conflict.Owner] == conflict.Alias
	commandHadAlias := current[conflict.Command] == conflict.Alias
	switch {
	case ownerHadAlias && !commandHadAlias:
		return conflict.Owner, true
	case commandHadAlias && !ownerHadAlias:
		return conflict.Command, true
	default:
		return "", false
	}
}

func aliasOverwriteWinner(conflict *AliasConflictError, loser string) string {
	if loser == conflict.Owner {
		return conflict.Command
	}
	return conflict.Owner
}
