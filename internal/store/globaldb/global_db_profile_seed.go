package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

const defaultProfileName = "default"

// VerifyDefaultProfile checks permanent identity without resetting editable presentation.
func (g *GlobalDB) VerifyDefaultProfile(ctx context.Context) error {
	if g == nil || g.db == nil {
		return errors.New("store: global database is required")
	}
	if g.closed.Load() != 0 {
		return store.ErrClosed
	}
	if ctx == nil {
		return errors.New("store: verify default profile context is required")
	}
	var name string
	var state string
	if err := g.db.QueryRowContext(
		ctx,
		`SELECT name, state FROM profiles WHERE id = ?`,
		store.DefaultProfileID,
	).Scan(&name, &state); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("store: default profile seed is missing")
		}
		return fmt.Errorf("store: read default profile seed: %w", err)
	}
	if strings.TrimSpace(name) != defaultProfileName || strings.TrimSpace(state) != globalDBSessionStateActive {
		return fmt.Errorf(
			"store: default profile seed does not match the permanent identity",
		)
	}
	return nil
}
