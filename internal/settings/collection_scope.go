package settings

import (
	"errors"
	"fmt"
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
	available, err := availableCollectionScopes(collection)
	if err != nil {
		return nil, err
	}
	for _, candidate := range available {
		if candidate == scope {
			return available, nil
		}
	}
	return nil, conflictError(errors.New(
		"settings: collection " + string(collection) + " does not support " + string(scope) + " scope",
	))
}
