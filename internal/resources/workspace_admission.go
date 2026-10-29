package resources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// checkAutomationWorkspaceAdmission shares the resource write transaction with archive admission.
func checkAutomationWorkspaceAdmission(ctx context.Context, tx sqlExecutor, record RawRecord) error {
	if record.Kind != "automation.job" && record.Kind != "automation.trigger" {
		return nil
	}
	var spec struct {
		WorkspaceID string `json:"workspace_id"`
		Enabled     bool   `json:"enabled"`
		Source      string `json:"source"`
		LoopTarget  *struct {
			WorkspaceID string `json:"workspace_id"`
		} `json:"loop_target"`
	}
	if err := json.Unmarshal(record.SpecJSON, &spec); err != nil {
		return fmt.Errorf("resources: decode automation admission: %w", err)
	}
	target := ""
	if spec.LoopTarget != nil {
		target = spec.LoopTarget.WorkspaceID
	}
	return store.RequireAutomationWorkspaceActive(ctx, tx, string(record.Kind), record.ID,
		spec.WorkspaceID, target, spec.Enabled, spec.Source == "config" || spec.Source == "package")
}
