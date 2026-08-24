package globaldb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	presetspkg "github.com/compozy/compozy/internal/notifications/presets"
)

func builtInPresetDefaultsCurrent(
	ctx context.Context,
	db *sql.DB,
	defaults []presetspkg.Preset,
) (bool, error) {
	if len(defaults) == 0 {
		return true, nil
	}
	clauses := make([]string, 0, len(defaults))
	args := make([]any, 0, len(defaults)*2)
	for _, preset := range defaults {
		normalized := preset.Normalize()
		clauses = append(clauses, "(name = ? AND default_version = ?)")
		args = append(args, normalized.Name, normalized.DefaultVersion)
	}
	query := `SELECT COUNT(*) FROM notification_presets
		WHERE built_in = 1 AND (` + strings.Join(clauses, " OR ") + `)`
	var count int
	if err := db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return false, fmt.Errorf("store: inspect built-in notification preset defaults: %w", err)
	}
	return count == len(defaults), nil
}
