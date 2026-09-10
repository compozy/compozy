package globaldb

import (
	"context"
	"fmt"

	automation "github.com/compozy/compozy/internal/automation/model"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (g *AutomationRepo) prepareAutomationRunInsert(
	ctx context.Context,
	run automation.Run,
) (automation.Run, sqlcgen.InsertAutomationRunParams, error) {
	normalized, err := g.normalizeRunForCreate(run)
	if err != nil {
		return automation.Run{}, sqlcgen.InsertAutomationRunParams{}, err
	}
	// Capture the parent's owner while it exists; run history outlives catalog entries.
	err = g.db.QueryRowContext(ctx, `SELECT COALESCE(`+
		automationOwnerProfileJobResourceSQL+`?),`+
		automationOwnerProfileJobTableSQL+`?),`+
		automationOwnerProfileTriggerResourceSQL+`?),`+
		automationOwnerProfileTriggerTableSQL+`?), ?)`,
		normalized.JobID, normalized.JobID, normalized.TriggerID, normalized.TriggerID,
		normalized.ProfileID,
	).Scan(&normalized.ProfileID)
	if err != nil {
		return automation.Run{}, sqlcgen.InsertAutomationRunParams{}, fmt.Errorf(
			"store: resolve automation run owner: %w",
			err,
		)
	}
	metadataJSON, err := encodeAutomationRunMetadata(normalized.Metadata)
	if err != nil {
		return automation.Run{}, sqlcgen.InsertAutomationRunParams{}, err
	}
	networkParticipation, err := encodeOptionalAutomationParticipation(
		normalized.NetworkParticipation,
		normalized.NetworkParticipation == nil,
		"run.network_participation",
	)
	if err != nil {
		return automation.Run{}, sqlcgen.InsertAutomationRunParams{}, err
	}

	return normalized, automationRunParams(normalized, networkParticipation, metadataJSON), nil
}
