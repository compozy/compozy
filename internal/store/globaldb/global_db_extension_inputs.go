package globaldb

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/extensioninput"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ extensioninput.Store = (*ExtensionInputRepo)(nil)

// ExtensionInputStore exposes the input repository at the daemon composition boundary.
func (g *GlobalDB) ExtensionInputStore() extensioninput.Store { return g.ExtensionInputs }

// List reads all active and inactive inputs from one exact instance.
func (r *ExtensionInputRepo) List(
	ctx context.Context, instance extensioninput.Instance,
) (map[string]extensioninput.Record, error) {
	if err := r.checkReady(ctx, "list extension inputs"); err != nil {
		return nil, err
	}
	instance, err := normalizeExtensionInputInstance(instance)
	if err != nil {
		return nil, err
	}
	return readExtensionInputs(ctx, r.queries, instance)
}

// Apply verifies before-images and commits the complete batch in one transaction.
func (r *ExtensionInputRepo) Apply(
	ctx context.Context, instance extensioninput.Instance, mutations []extensioninput.Mutation,
) (err error) {
	if err := r.checkReady(ctx, "apply extension inputs"); err != nil {
		return err
	}
	instance, err = normalizeExtensionInputInstance(instance)
	if err != nil {
		return err
	}
	if err := validateExtensionInputMutations(mutations); err != nil {
		return err
	}
	if len(mutations) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin extension input batch: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("store: rollback extension input batch: %w", rollbackErr))
		}
	}()
	queries := sqlcgen.New(tx)
	current, err := readExtensionInputs(ctx, queries, instance)
	if err != nil {
		return err
	}
	for _, mutation := range mutations {
		before, found := current[mutation.InputID]
		if (mutation.Before == nil && found) || (mutation.Before != nil &&
			(!found || !equalExtensionInputRecord(before, *mutation.Before))) {
			return fmt.Errorf("store: input %q: %w", mutation.InputID, extensioninput.ErrConflict)
		}
		if err := writeExtensionInput(ctx, queries, instance, mutation); err != nil {
			return fmt.Errorf("store: write extension input %q: %w", mutation.InputID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit extension input batch: %w", err)
	}
	return nil
}

func normalizeExtensionInputInstance(instance extensioninput.Instance) (extensioninput.Instance, error) {
	instance.Extension = strings.TrimSpace(instance.Extension)
	instance.ProfileID = strings.TrimSpace(instance.ProfileID)
	instance.WorkspaceID = strings.TrimSpace(instance.WorkspaceID)
	if instance.Extension == "" {
		return instance, errors.New("store: extension input instance name is required")
	}
	return instance, nil
}

func validateExtensionInputMutations(mutations []extensioninput.Mutation) error {
	seen := make(map[string]struct{}, len(mutations))
	for _, mutation := range mutations {
		if mutation.InputID == "" || strings.TrimSpace(mutation.InputID) != mutation.InputID {
			return errors.New("store: extension input id is required and must be normalized")
		}
		if _, exists := seen[mutation.InputID]; exists {
			return fmt.Errorf("store: duplicate extension input %q", mutation.InputID)
		}
		seen[mutation.InputID] = struct{}{}
		if mutation.After == nil {
			if mutation.Before == nil {
				return fmt.Errorf("store: extension input %q mutation has no state", mutation.InputID)
			}
			continue
		}
		if err := validateStoredExtensionInput(*mutation.After); err != nil {
			return fmt.Errorf("store: extension input %q: %w", mutation.InputID, err)
		}
	}
	return nil
}

func validateStoredExtensionInput(record extensioninput.Record) error {
	if record.UpdatedAt.IsZero() {
		return errors.New("updated_at is required")
	}
	var value any
	if err := json.Unmarshal(record.Value, &value); err != nil {
		return errors.New("value must be valid JSON")
	}
	switch record.Type {
	case "string", "identifier":
		text, ok := value.(string)
		if !ok || len(text) > 8192 || strings.ContainsRune(text, '\x00') {
			return errors.New("value must be a string of at most 8 KiB without NUL")
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return errors.New("value must be a boolean")
		}
	default:
		return errors.New("only non-secret input types can be persisted")
	}
	return nil
}

func equalExtensionInputRecord(left, right extensioninput.Record) bool {
	return left.Type == right.Type && bytes.Equal(left.Value, right.Value) && left.Active == right.Active &&
		left.UpdatedAt.Equal(right.UpdatedAt)
}

func readExtensionInputs(
	ctx context.Context, queries *sqlcgen.Queries, instance extensioninput.Instance,
) (map[string]extensioninput.Record, error) {
	rows, err := queries.ListExtensionInputs(ctx, sqlcgen.ListExtensionInputsParams{
		Extension: instance.Extension, Profile: instance.ProfileID, WorkspaceID: instance.WorkspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("store: list extension inputs: %w", err)
	}
	result := make(map[string]extensioninput.Record, len(rows))
	for _, row := range rows {
		updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("store: read extension input %q timestamp: %w", row.InputID, err)
		}
		result[row.InputID] = extensioninput.Record{
			Type: row.Type, Value: json.RawMessage(row.ValueJson), Active: row.Active == 1, UpdatedAt: updatedAt,
		}
	}
	return result, nil
}

func writeExtensionInput(
	ctx context.Context, queries *sqlcgen.Queries, instance extensioninput.Instance, mutation extensioninput.Mutation,
) error {
	if mutation.After == nil {
		return queries.DeleteExtensionInput(ctx, sqlcgen.DeleteExtensionInputParams{
			Extension: instance.Extension, Profile: instance.ProfileID, WorkspaceID: instance.WorkspaceID,
			InputID: mutation.InputID,
		})
	}
	row := mutation.After
	var active int64
	if row.Active {
		active = 1
	}
	return queries.UpsertExtensionInput(ctx, sqlcgen.UpsertExtensionInputParams{
		Extension: instance.Extension, Profile: instance.ProfileID, WorkspaceID: instance.WorkspaceID,
		InputID: mutation.InputID, Type: row.Type, ValueJson: string(row.Value), Active: active,
		UpdatedAt: store.FormatTimestamp(row.UpdatedAt),
	})
}
