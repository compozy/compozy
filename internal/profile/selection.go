package profile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store/globaldb"
)

type selectionStore struct{ manager *Manager }

var _ SelectionStore = (*selectionStore)(nil)

func (m *Manager) ListSelections(ctx context.Context) ([]Selection, error) {
	return m.selections.List(ctx)
}

func (m *Manager) PutSelection(ctx context.Context, selection Selection) error {
	return m.selections.Put(ctx, selection)
}

func (s *selectionStore) Get(
	ctx context.Context,
	lens SelectionLens,
	workspaceID string,
) (Selection, bool, error) {
	validated, err := normalizeSelectionLens(lens, workspaceID)
	if err != nil {
		return Selection{}, false, err
	}
	var selection Selection
	err = s.manager.store.DB().QueryRowContext(
		ctx,
		`SELECT lens, workspace_id, profile_id FROM profile_selections WHERE lens = ? AND workspace_id = ?`,
		validated.Lens,
		validated.WorkspaceID,
	).Scan(&selection.Lens, &selection.WorkspaceID, &selection.ProfileID)
	if errors.Is(err, sql.ErrNoRows) {
		return Selection{}, false, nil
	}
	if err != nil {
		return Selection{}, false, fmt.Errorf("profile: get selection: %w", err)
	}
	return selection, true, nil
}

func (s *selectionStore) List(ctx context.Context) (selections []Selection, err error) {
	if ctx == nil {
		return nil, errors.New("profile: list selections context is required")
	}
	rows, err := s.manager.store.DB().QueryContext(
		ctx,
		`SELECT lens, workspace_id, profile_id FROM profile_selections ORDER BY lens, workspace_id`,
	)
	if err != nil {
		return nil, fmt.Errorf("profile: list selections: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("profile: close selection rows: %w", closeErr))
		}
	}()
	selections = make([]Selection, 0)
	for rows.Next() {
		var selection Selection
		if err := rows.Scan(&selection.Lens, &selection.WorkspaceID, &selection.ProfileID); err != nil {
			return nil, fmt.Errorf("profile: scan selection: %w", err)
		}
		selections = append(selections, selection)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("profile: iterate selections: %w", err)
	}
	return selections, nil
}

func (s *selectionStore) Put(ctx context.Context, selection Selection) error {
	validated, err := normalizeSelectionLens(selection.Lens, selection.WorkspaceID)
	if err != nil {
		return err
	}
	profileID := strings.TrimSpace(selection.ProfileID)
	if profileID == "" || profileID == "@all" {
		return fmt.Errorf("%w: selection requires a real profile id", ErrInvalidInput)
	}
	return s.manager.write(ctx, "put profile selection", func(exec globaldb.ProfileWriteExecutor) error {
		profile, err := getProfileByID(ctx, exec, profileID)
		if err != nil {
			return err
		}
		if err := ensureAvailable(ctx, exec, profile, true); err != nil {
			return err
		}
		_, err = exec.ExecContext(
			ctx,
			`INSERT INTO profile_selections (lens, workspace_id, profile_id, updated_at)
			 VALUES (?, ?, ?, ?)
			 ON CONFLICT(lens, workspace_id) DO UPDATE SET profile_id = excluded.profile_id, updated_at = excluded.updated_at`,
			validated.Lens,
			validated.WorkspaceID,
			profile.ID,
			formatTimestamp(s.manager.now()),
		)
		if err != nil {
			return fmt.Errorf("profile: put selection: %w", err)
		}
		return nil
	})
}

func (s *selectionStore) SweepProfile(ctx context.Context, profileID string) error {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return fmt.Errorf("%w: profile id is required", ErrInvalidInput)
	}
	_, err := s.manager.store.DB().ExecContext(ctx, `DELETE FROM profile_selections WHERE profile_id = ?`, profileID)
	if err != nil {
		return fmt.Errorf("profile: sweep selections: %w", err)
	}
	return nil
}

func normalizeSelectionLens(lens SelectionLens, workspaceID string) (Selection, error) {
	l := Lens{Kind: lens, WorkspaceID: strings.TrimSpace(workspaceID)}
	if err := l.Validate(); err != nil {
		return Selection{}, err
	}
	return Selection{Lens: l.Kind, WorkspaceID: l.WorkspaceID}, nil
}
