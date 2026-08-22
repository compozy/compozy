package globaldb

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	"github.com/compozy/compozy/internal/worktree"
)

func worktreeFromSQLC(row sqlcgen.Worktree) (*worktree.Worktree, error) {
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return nil, err
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &worktree.Worktree{
		ID: row.ID, ProfileID: row.ProfileID, WorkspaceID: row.WorkspaceID, Name: row.Name, Branch: row.Branch,
		Path: row.Path, GitDir: row.GitDir, State: worktree.State(row.State),
		PendingPhase: worktree.PendingPhase(row.PendingPhase), Origin: worktree.Origin(row.Origin),
		SetupState: worktree.SetupState(row.SetupState), SetupError: row.SetupError,
		BaseRef: row.BaseRef, CreatedBranch: row.CreatedBranch != 0,
		RunNamespace: row.RunNamespace, CreatedHead: row.CreatedHead, RunID: row.RunID,
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}

func worktreesFromSQLC(rows []sqlcgen.Worktree) ([]worktree.Worktree, error) {
	items := make([]worktree.Worktree, 0, len(rows))
	for _, row := range rows {
		item, err := worktreeFromSQLC(row)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, nil
}

func nullableString(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

func nullableInt(value *int) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}

func nullableBool(value *bool) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: boolToInt64(*value), Valid: true}
}

func nullableTime(value *time.Time) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: store.FormatTimestamp(*value), Valid: true}
}

func pointerString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	cloned := value.String
	return &cloned
}

func pointerInt(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	cloned := int(value.Int64)
	return &cloned
}

func pointerBool(value sql.NullInt64) *bool {
	if !value.Valid {
		return nil
	}
	cloned := value.Int64 != 0
	return &cloned
}

func pointerTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	parsed, err := store.ParseTimestamp(value.String)
	if err != nil {
		return nil, fmt.Errorf("store: parse worktree timestamp: %w", err)
	}
	return &parsed, nil
}

func boolToInt64(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
