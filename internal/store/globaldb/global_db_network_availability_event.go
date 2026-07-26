package globaldb

import (
	"context"
	"encoding/json"
	"fmt"

	eventspkg "github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

type networkAvailabilityEventContent struct {
	PreviousEnabled bool   `json:"previous_enabled"`
	Enabled         bool   `json:"enabled"`
	Epoch           int64  `json:"epoch"`
	UpdatedBy       string `json:"updated_by"`
	UpdatedAt       string `json:"updated_at"`
}

func appendNetworkAvailabilityEventSummary(
	ctx context.Context,
	exec networkSQLExecutor,
	previous store.NetworkAvailability,
	current store.NetworkAvailability,
) error {
	content, err := json.Marshal(networkAvailabilityEventContent{
		PreviousEnabled: previous.Enabled,
		Enabled:         current.Enabled,
		Epoch:           current.Epoch,
		UpdatedBy:       current.UpdatedBy,
		UpdatedAt:       store.FormatTimestamp(current.UpdatedAt),
	})
	if err != nil {
		return fmt.Errorf("store: marshal network availability event: %w", err)
	}
	_, err = exec.ExecContext(
		ctx,
		`INSERT INTO event_summaries
        (id, type, content_json, actor_kind, actor_id, summary, timestamp)
        VALUES (?, ?, ?, ?, ?, ?, ?)`,
		store.NewID("sum"),
		eventspkg.NetworkAvailabilityChanged,
		string(content),
		string(taskpkg.ActorKindDaemon),
		current.UpdatedBy,
		"Network availability changed",
		store.FormatTimestamp(current.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("store: append network availability event: %w", err)
	}
	return nil
}
