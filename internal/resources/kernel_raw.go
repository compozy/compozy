package resources

import (
	"context"

	"fmt"
	"strings"
)

func normalizeFilter(filter ResourceFilter) (ResourceFilter, error) {
	normalized := filter
	normalized.Kind = filter.Kind.Normalize()
	if normalized.Kind != "" {
		if err := normalized.Kind.Validate("filter.kind"); err != nil {
			return ResourceFilter{}, err
		}
	}
	if filter.Scope != nil {
		scope := filter.Scope.Normalize()
		if err := scope.Validate("filter.scope"); err != nil {
			return ResourceFilter{}, err
		}
		normalized.Scope = &scope
	}
	if filter.Owner != nil {
		owner := filter.Owner.Normalize()
		if err := owner.Validate("filter.owner"); err != nil {
			return ResourceFilter{}, err
		}
		normalized.Owner = &owner
	}
	if filter.Source != nil {
		source := filter.Source.Normalize()
		if err := source.Validate("filter.source"); err != nil {
			return ResourceFilter{}, err
		}
		normalized.Source = &source
	}
	if normalized.Limit < 0 {
		return ResourceFilter{}, fmt.Errorf("%w: filter.limit cannot be negative: %d", ErrValidation, normalized.Limit)
	}
	return normalized, nil
}

func (k *Kernel) preparePutRaw(actor MutationActor, draft RawDraft) (MutationActor, RawDraft, error) {
	normalizedActor, err := normalizeActor(actor)
	if err != nil {
		return MutationActor{}, RawDraft{}, err
	}
	if normalizedActor.Kind == MutationActorKindExtension {
		return MutationActor{}, RawDraft{}, fmt.Errorf(
			"%w: extension actors cannot use direct raw mutations",
			ErrDirectMutationNotAllowed,
		)
	}
	if err := normalizedActor.Source.Validate("actor.source"); err != nil {
		return MutationActor{}, RawDraft{}, err
	}

	normalizedDraft, err := normalizeDraft(draft, k.maxSpecBytes)
	if err != nil {
		return MutationActor{}, RawDraft{}, err
	}
	if err := validateActorWriteAccess(normalizedActor, normalizedDraft.Kind, normalizedDraft.Scope); err != nil {
		return MutationActor{}, RawDraft{}, err
	}
	return normalizedActor, normalizedDraft, nil
}

func (k *Kernel) putRawWithExecutor(
	ctx context.Context,
	exec sqlExecutor,
	actor MutationActor,
	draft RawDraft,
) (RawRecord, error) {
	existing, found, err := lookupRecordWithExecutor(ctx, exec, draft.Kind, draft.ID)
	if err != nil {
		return RawRecord{}, err
	}
	if !found {
		if draft.ExpectedVersion != 0 {
			return RawRecord{}, ErrNotFound
		}
		return k.insertRawRecord(ctx, exec, actor, draft)
	}

	if err := validateActorWriteAccess(actor, existing.Kind, existing.Scope); err != nil {
		return RawRecord{}, err
	}
	if existing.Source != actor.Source {
		return RawRecord{}, fmt.Errorf(
			"%w: actor cannot mutate source %q/%q",
			ErrPermissionDenied,
			existing.Source.Kind,
			existing.Source.ID,
		)
	}
	if draft.ExpectedVersion == 0 || existing.Version != draft.ExpectedVersion {
		return RawRecord{}, fmt.Errorf("%w: expected version %d", ErrConflict, draft.ExpectedVersion)
	}

	now := k.now()
	record := RawRecord{
		Kind:      draft.Kind,
		ID:        draft.ID,
		Version:   existing.Version + 1,
		Scope:     draft.Scope,
		Owner:     ownerFromActor(actor),
		Source:    actor.Source,
		SpecJSON:  append([]byte(nil), draft.SpecJSON...),
		CreatedAt: existing.CreatedAt,
		UpdatedAt: now,
	}
	if err := updateRecord(ctx, exec, record, draft.ExpectedVersion); err != nil {
		return RawRecord{}, err
	}
	return record, nil
}

func (k *Kernel) insertRawRecord(
	ctx context.Context,
	exec sqlExecutor,
	actor MutationActor,
	draft RawDraft,
) (RawRecord, error) {
	now := k.now()
	record := RawRecord{
		Kind:      draft.Kind,
		ID:        draft.ID,
		Version:   1,
		Scope:     draft.Scope,
		Owner:     ownerFromActor(actor),
		Source:    actor.Source,
		SpecJSON:  append([]byte(nil), draft.SpecJSON...),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := insertRecord(ctx, exec, record); err != nil {
		return RawRecord{}, err
	}
	return record, nil
}

func (k *Kernel) prepareDeleteRaw(
	actor MutationActor,
	kind ResourceKind,
	id string,
) (MutationActor, ResourceKind, string, error) {
	normalizedActor, err := normalizeActor(actor)
	if err != nil {
		return MutationActor{}, "", "", err
	}
	if normalizedActor.Kind == MutationActorKindExtension {
		return MutationActor{}, "", "", fmt.Errorf(
			"%w: extension actors cannot use direct raw mutations",
			ErrDirectMutationNotAllowed,
		)
	}
	if err := normalizedActor.Source.Validate("actor.source"); err != nil {
		return MutationActor{}, "", "", err
	}

	normalizedKind := kind.Normalize()
	if err := normalizedKind.Validate("kind"); err != nil {
		return MutationActor{}, "", "", err
	}
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return MutationActor{}, "", "", fmt.Errorf("%w: id is required", ErrValidation)
	}
	return normalizedActor, normalizedKind, trimmedID, nil
}

func (k *Kernel) deleteRawWithExecutor(
	ctx context.Context,
	exec sqlExecutor,
	actor MutationActor,
	kind ResourceKind,
	id string,
	expectedVersion int64,
) error {
	if expectedVersion < 0 {
		return fmt.Errorf("%w: expected_version cannot be negative: %d", ErrValidation, expectedVersion)
	}

	existing, found, err := lookupRecordWithExecutor(ctx, exec, kind, id)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	if err := validateActorWriteAccess(actor, existing.Kind, existing.Scope); err != nil {
		return err
	}
	if existing.Source != actor.Source {
		return fmt.Errorf(
			"%w: actor cannot mutate source %q/%q",
			ErrPermissionDenied,
			existing.Source.Kind,
			existing.Source.ID,
		)
	}

	result, err := exec.ExecContext(ctx, deleteRecordByVersionQuery, kind, id, expectedVersion)
	if err != nil {
		return fmt.Errorf("resources: delete record %q/%q: %w", kind, id, err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("resources: rows affected for delete %q/%q: %w", kind, id, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("%w: expected version %d", ErrConflict, expectedVersion)
	}
	return nil
}

func (k *Kernel) prepareListRaw(
	actor MutationActor,
	filter ResourceFilter,
) (MutationActor, ResourceFilter, error) {
	normalizedActor, err := normalizeActor(actor)
	if err != nil {
		return MutationActor{}, ResourceFilter{}, err
	}
	normalizedFilter, err := normalizeFilter(filter)
	if err != nil {
		return MutationActor{}, ResourceFilter{}, err
	}

	if normalizedFilter.Kind != "" && !actorAllowsKind(normalizedActor, normalizedFilter.Kind) {
		return MutationActor{}, ResourceFilter{}, fmt.Errorf(
			"%w: actor cannot read resource kind %q",
			ErrPermissionDenied,
			normalizedFilter.Kind,
		)
	}
	if normalizedFilter.Scope != nil {
		if !actorAllowsScopeKind(normalizedActor, normalizedFilter.Scope.Kind) {
			return MutationActor{}, ResourceFilter{}, fmt.Errorf(
				"%w: actor cannot read scope kind %q",
				ErrPermissionDenied,
				normalizedFilter.Scope.Kind,
			)
		}
		if !actorAllowsScope(normalizedActor, *normalizedFilter.Scope) {
			return MutationActor{}, ResourceFilter{}, fmt.Errorf(
				"%w: actor max scope does not allow %q/%q",
				ErrPermissionDenied,
				normalizedFilter.Scope.Kind,
				normalizedFilter.Scope.ID,
			)
		}
	}
	if normalizedActor.Kind == MutationActorKindExtension &&
		normalizedFilter.Source != nil &&
		*normalizedFilter.Source != normalizedActor.Source {
		return MutationActor{}, ResourceFilter{}, fmt.Errorf(
			"%w: actor cannot read source %q/%q",
			ErrPermissionDenied,
			normalizedFilter.Source.Kind,
			normalizedFilter.Source.ID,
		)
	}
	return normalizedActor, normalizedFilter, nil
}

func (k *Kernel) extensionReadGrantsEmpty(actor MutationActor) bool {
	return actor.Kind == MutationActorKindExtension &&
		(len(actor.GrantedKinds) == 0 || len(actor.GrantedScopes) == 0)
}

func buildListRawQuery(actor MutationActor, filter ResourceFilter) (string, []any) {
	clauses := make([]string, 0, 8)
	args := make([]any, 0, 10)

	if actor.Kind == MutationActorKindExtension {
		clauses = append(clauses, "source_kind = ?", "source_id = ?")
		args = append(args, actor.Source.Kind, actor.Source.ID)
	}
	if filter.Kind != "" {
		clauses = append(clauses, "kind = ?")
		args = append(args, filter.Kind)
	}
	if filter.Scope != nil {
		clauses = append(clauses, "scope_kind = ?")
		args = append(args, filter.Scope.Kind)
		if filter.Scope.Kind == ResourceScopeKindGlobal {
			clauses = append(clauses, "scope_id IS NULL")
		} else {
			clauses = append(clauses, "scope_id = ?")
			args = append(args, filter.Scope.ID)
		}
	}
	if filter.Owner != nil {
		clauses = append(clauses, "owner_kind = ?", "owner_id = ?")
		args = append(args, filter.Owner.Kind, filter.Owner.ID)
	}
	if filter.Source != nil && actor.Kind != MutationActorKindExtension {
		clauses = append(clauses, "source_kind = ?", "source_id = ?")
		args = append(args, filter.Source.Kind, filter.Source.ID)
	}

	var builder strings.Builder
	builder.WriteString(rawRecordSelectQuery)
	if len(clauses) > 0 {
		builder.WriteString("\nWHERE ")
		builder.WriteString(strings.Join(clauses, "\n\tAND "))
	}
	builder.WriteString(resourceRecordOrderBy)
	return builder.String(), args
}
