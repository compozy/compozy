package globaldb

import (
	automation "github.com/compozy/compozy/internal/automation/model"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (g *AutomationRepo) prepareAutomationRunInsert(
	run automation.Run,
) (automation.Run, sqlcgen.InsertAutomationRunParams, error) {
	normalized, err := g.normalizeRunForCreate(run)
	if err != nil {
		return automation.Run{}, sqlcgen.InsertAutomationRunParams{}, err
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
