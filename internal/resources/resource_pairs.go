package resources

import "strings"

// ParseResourceScopePair builds an optional normalized scope from paired input fields.
func ParseResourceScopePair(rawKind string, rawID string, path string) (ResourceScope, bool, error) {
	if strings.TrimSpace(rawKind) == "" && strings.TrimSpace(rawID) == "" {
		return ResourceScope{}, false, nil
	}
	scope := ResourceScope{Kind: ResourceScopeKind(rawKind), ID: rawID}.Normalize()
	if err := scope.Validate(path); err != nil {
		return ResourceScope{}, false, err
	}
	return scope, true, nil
}

// ParseResourceOwnerPair builds an optional normalized owner from paired input fields.
func ParseResourceOwnerPair(rawKind string, rawID string, path string) (ResourceOwner, bool, error) {
	if strings.TrimSpace(rawKind) == "" && strings.TrimSpace(rawID) == "" {
		return ResourceOwner{}, false, nil
	}
	owner := ResourceOwner{Kind: ResourceOwnerKind(rawKind), ID: rawID}.Normalize()
	if err := owner.Validate(path); err != nil {
		return ResourceOwner{}, false, err
	}
	return owner, true, nil
}

// ParseResourceSourcePair builds an optional normalized source from paired input fields.
func ParseResourceSourcePair(rawKind string, rawID string, path string) (ResourceSource, bool, error) {
	if strings.TrimSpace(rawKind) == "" && strings.TrimSpace(rawID) == "" {
		return ResourceSource{}, false, nil
	}
	source := ResourceSource{Kind: ResourceSourceKind(rawKind), ID: rawID}.Normalize()
	if err := source.Validate(path); err != nil {
		return ResourceSource{}, false, err
	}
	return source, true, nil
}
