package resources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

func normalizeActor(actor MutationActor) (MutationActor, error) {
	normalized := actor
	normalized.Kind = actor.Kind.Normalize()
	normalized.ID = strings.TrimSpace(actor.ID)
	normalized.SessionNonce = strings.TrimSpace(actor.SessionNonce)
	normalized.Owner = actor.Owner.Normalize()
	normalized.Source = actor.Source.Normalize()
	normalized.MaxScope = actor.MaxScope.Normalize()
	normalized.GrantedKinds = normalizeKinds(actor.GrantedKinds)
	normalized.GrantedScopes = normalizeScopeKinds(actor.GrantedScopes)

	if err := normalized.Kind.Validate("actor.kind"); err != nil {
		return MutationActor{}, err
	}
	if normalized.ID == "" {
		return MutationActor{}, fmt.Errorf("%w: actor.id is required", ErrValidation)
	}
	if err := normalized.MaxScope.Validate("actor.max_scope"); err != nil {
		return MutationActor{}, err
	}
	if normalized.Kind == MutationActorKindExtension {
		if err := normalized.Source.Validate("actor.source"); err != nil {
			return MutationActor{}, err
		}
	}
	if actorOwnerProvided(normalized.Owner) {
		if normalized.Kind == MutationActorKindExtension {
			return MutationActor{}, fmt.Errorf(
				"%w: extension actors cannot override resource owner",
				ErrPermissionDenied,
			)
		}
		if err := normalized.Owner.Validate("actor.owner"); err != nil {
			return MutationActor{}, err
		}
	}
	return normalized, nil
}

func normalizeDraft(draft RawDraft, maxSpecBytes int) (RawDraft, error) {
	normalized := draft
	normalized.Kind = draft.Kind.Normalize()
	normalized.ID = strings.TrimSpace(draft.ID)
	normalized.Scope = draft.Scope.Normalize()

	if err := normalized.Kind.Validate("draft.kind"); err != nil {
		return RawDraft{}, err
	}
	if normalized.ID == "" {
		return RawDraft{}, fmt.Errorf("%w: draft.id is required", ErrValidation)
	}
	if err := normalized.Scope.Validate("draft.scope"); err != nil {
		return RawDraft{}, err
	}
	specJSON, err := normalizeJSON(draft.SpecJSON, maxSpecBytes, "draft.spec_json")
	if err != nil {
		return RawDraft{}, err
	}
	normalized.SpecJSON = specJSON
	if normalized.ExpectedVersion < 0 {
		return RawDraft{}, fmt.Errorf(
			"%w: draft.expected_version cannot be negative: %d",
			ErrValidation,
			normalized.ExpectedVersion,
		)
	}
	return normalized, nil
}

func normalizeSnapshot(snapshot SourceSnapshot, maxRecords int) (SourceSnapshot, error) {
	normalized := snapshot
	if normalized.SourceVersion <= 0 {
		return SourceSnapshot{}, fmt.Errorf(
			"%w: snapshot.source_version must be positive: %d",
			ErrValidation,
			normalized.SourceVersion,
		)
	}
	if err := validateBoundedCount(len(snapshot.Records), maxRecords, "snapshot.records"); err != nil {
		return SourceSnapshot{}, err
	}
	return normalized, nil
}

func normalizeJSON(payload []byte, maxBytes int, path string) ([]byte, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("%w: %s is required", ErrValidation, path)
	}
	if !json.Valid(trimmed) {
		return nil, fmt.Errorf("%w: %s must contain valid JSON", ErrValidation, path)
	}
	if len(trimmed) > maxBytes {
		return nil, fmt.Errorf("%w: %s exceeds %d bytes", ErrPayloadTooLarge, path, maxBytes)
	}
	return append([]byte(nil), trimmed...), nil
}

func validateBoundedCount(count int, maxCount int, path string) error {
	if count < 0 {
		return fmt.Errorf("%w: %s cannot be negative: %d", ErrValidation, path, count)
	}
	if count > maxCount {
		return fmt.Errorf("%w: %s exceeds %d: %d", ErrPayloadTooLarge, path, maxCount, count)
	}
	return nil
}

func validateActorWriteAccess(actor MutationActor, kind ResourceKind, scope ResourceScope) error {
	if !actorAllowsKind(actor, kind) {
		return fmt.Errorf("%w: actor cannot access resource kind %q", ErrPermissionDenied, kind)
	}
	if !actorAllowsScopeKind(actor, scope.Kind) {
		return fmt.Errorf("%w: actor cannot access scope kind %q", ErrPermissionDenied, scope.Kind)
	}
	if !actorAllowsScope(actor, scope) {
		return fmt.Errorf("%w: actor max scope does not allow %q/%q", ErrPermissionDenied, scope.Kind, scope.ID)
	}
	return nil
}

func validateActorReadAccess(actor MutationActor, record RawRecord) error {
	if !actorAllowsKind(actor, record.Kind) {
		return fmt.Errorf("%w: actor cannot read resource kind %q", ErrPermissionDenied, record.Kind)
	}
	if !actorAllowsScopeKind(actor, record.Scope.Kind) {
		return fmt.Errorf("%w: actor cannot read scope kind %q", ErrPermissionDenied, record.Scope.Kind)
	}
	if !actorAllowsScope(actor, record.Scope) {
		return fmt.Errorf(
			"%w: actor max scope does not allow %q/%q",
			ErrPermissionDenied,
			record.Scope.Kind,
			record.Scope.ID,
		)
	}
	if actor.Kind == MutationActorKindExtension && record.Source != actor.Source {
		return fmt.Errorf(
			"%w: actor cannot read source %q/%q",
			ErrPermissionDenied,
			record.Source.Kind,
			record.Source.ID,
		)
	}
	return nil
}

func actorAllowsKind(actor MutationActor, kind ResourceKind) bool {
	if len(actor.GrantedKinds) == 0 {
		return actor.Kind != MutationActorKindExtension
	}
	return slices.Contains(actor.GrantedKinds, kind)
}

func actorAllowsScopeKind(actor MutationActor, scopeKind ResourceScopeKind) bool {
	if len(actor.GrantedScopes) == 0 {
		return actor.Kind != MutationActorKindExtension
	}
	return slices.Contains(actor.GrantedScopes, scopeKind)
}

func actorAllowsScope(actor MutationActor, target ResourceScope) bool {
	maxScope := actor.MaxScope.Normalize()
	target = target.Normalize()
	switch maxScope.Kind {
	case ResourceScopeKindGlobal:
		return target.Kind == ResourceScopeKindGlobal || target.Kind == ResourceScopeKindWorkspace
	case ResourceScopeKindWorkspace:
		return target.Kind == ResourceScopeKindWorkspace && target.ID == maxScope.ID
	default:
		return false
	}
}

func ownerFromActor(actor MutationActor) ResourceOwner {
	if actorOwnerProvided(actor.Owner) {
		return actor.Owner.Normalize()
	}
	if actor.Kind == MutationActorKindExtension {
		return ResourceOwner{
			Kind: ResourceOwnerKind(actor.Source.Kind),
			ID:   actor.Source.ID,
		}
	}
	return ResourceOwner{
		Kind: ResourceOwnerKind(actor.Kind),
		ID:   actor.ID,
	}
}

func actorOwnerProvided(owner ResourceOwner) bool {
	return strings.TrimSpace(string(owner.Kind)) != "" || strings.TrimSpace(owner.ID) != ""
}

func normalizeKinds(values []ResourceKind) []ResourceKind {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]ResourceKind, 0, len(values))
	seen := make(map[ResourceKind]struct{}, len(values))
	for _, value := range values {
		next := value.Normalize()
		if next == "" {
			continue
		}
		if _, ok := seen[next]; ok {
			continue
		}
		seen[next] = struct{}{}
		normalized = append(normalized, next)
	}
	return normalized
}

func normalizeScopeKinds(values []ResourceScopeKind) []ResourceScopeKind {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]ResourceScopeKind, 0, len(values))
	seen := make(map[ResourceScopeKind]struct{}, len(values))
	for _, value := range values {
		next := value.Normalize()
		if next == "" {
			continue
		}
		if _, ok := seen[next]; ok {
			continue
		}
		seen[next] = struct{}{}
		normalized = append(normalized, next)
	}
	return normalized
}

func nestedPath(path string, field string) string {
	trimmedPath := strings.TrimSpace(path)
	trimmedField := strings.TrimSpace(field)
	if trimmedPath == "" {
		return trimmedField
	}
	if trimmedField == "" {
		return trimmedPath
	}
	return trimmedPath + "." + trimmedField
}
