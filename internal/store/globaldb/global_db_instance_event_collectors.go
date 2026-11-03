package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/heartbeat"
)

func (r *InstanceRepo) collectAutomationEventFacts(
	ctx context.Context,
	id string,
	facts *heartbeat.CollectorFacts,
) error {
	rows, err := r.queries.ListPendingAgentInstanceEvents(ctx, id)
	if err != nil {
		return fmt.Errorf("collect instance events: %w", err)
	}
	for _, row := range rows {
		facts.AutomationEvents = append(facts.AutomationEvents, heartbeat.AutomationEventFact{
			TriggerID: row.TriggerID, RunID: row.RunID,
			TriggerName: row.TriggerName, Event: row.Event,
			Instruction: row.Instruction,
			ID:          row.ID, InstanceID: row.InstanceID, Summary: row.Summary, ReceivedAt: row.ReceivedAt,
		})
	}
	wake, err := r.queries.GetInstanceWakeState(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("collect omitted instance events: %w", err)
	}
	facts.EventsOmitted = wake.EventsOmitted
	return nil
}
