package settings

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func availableCollectionScopes(collection CollectionName) ([]ScopeKind, error) {
	switch collection {
	case CollectionProviders, CollectionSandboxes:
		return []ScopeKind{ScopeUser}, nil
	case CollectionMCPServers, CollectionHooks:
		return []ScopeKind{ScopeUser, ScopeProfile, ScopeWorkspace}, nil
	default:
		return nil, notFoundError(fmt.Errorf("settings: unknown collection %q", collection))
	}
}

func validateCollectionScope(collection CollectionName, scope ScopeKind) ([]ScopeKind, error) {
	if scope == ScopeAgent {
		return nil, conflictError(fmt.Errorf(
			"settings: collection %q does not support agent scope",
			collection,
		))
	}
	available, err := availableCollectionScopes(collection)
	if err != nil {
		return nil, err
	}
	if slices.Contains(available, scope) {
		return available, nil
	}
	return nil, conflictError(fmt.Errorf(
		"settings: collection %q does not support %s scope",
		collection,
		scope,
	))
}

// normalizeSettingsProfileName enforces the scope/profile relationship at the
// settings boundary. Callers use the returned value for both loading and
// writing so whitespace cannot select one profile and report another.
func normalizeSettingsProfileName(scope ScopeKind, raw string) (string, error) {
	profileName := strings.TrimSpace(raw)
	if scope == ScopeProfile {
		if profileName == "" || profileName == "default" {
			return "", validationError(errors.New(
				"settings: profile scope requires a non-default profile",
			))
		}
		if err := compozyconfig.ValidateResourceProfileName(profileName); err != nil {
			return "", validationError(err)
		}
		return profileName, nil
	}
	if profileName != "" {
		return "", conflictError(errors.New("settings: profile requires profile scope"))
	}
	return "", nil
}
