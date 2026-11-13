package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/compozy/compozy/internal/api/contract"
)

func writeBootstrapEvent(cmd *cobra.Command, event bootstrapEvent) error {
	mode, err := resolveOutputFormat(cmd)
	if err != nil {
		return err
	}
	if mode == OutputJSON || mode == OutputJSONL {
		return writeJSONLineWithoutWorkspaceResolution(cmd, event)
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", event.Phase, event.Message)
	return err
}

// bootstrapStage refines a phase for the desktop progress label.
type bootstrapStage string

// bootstrapStageMigrate reports that a replacement runtime is upgrading persisted data before readiness.
const bootstrapStageMigrate bootstrapStage = "migrate"

// writeBootstrapMigrationProgress tells the desktop the replacement is upgrading data, not failing to start.
func writeBootstrapMigrationProgress(cmd *cobra.Command, progress contract.RestartProgress) error {
	percent := restartProgressPercent(progress)
	completed, total := max(progress.Completed, 0), max(progress.Total, 0)
	return writeBootstrapEvent(cmd, bootstrapEvent{
		Type: bootstrapEventType, Phase: bootstrapPhaseProvision, Status: bootstrapStatusStarted,
		Resolution: bootstrapResolutionProvision, Stage: bootstrapStageMigrate, Percent: &percent,
		Completed: &completed, Total: &total, Migration: progress.Stage,
		Message: fmt.Sprintf("Upgrading CompozyOS data (%d%%).", percent),
	})
}

func writeBootstrapFailure(
	cmd *cobra.Command,
	phase bootstrapPhase,
	classification bootstrapProbeClass,
	cause error,
) error {
	event := bootstrapEvent{
		Type: bootstrapEventType, Phase: phase, Status: bootstrapStatusFailed, Classification: classification,
		Message: cause.Error(),
	}
	if compatibilityFailure, ok := errors.AsType[*bootstrapCompatibilityFailure](cause); ok {
		event.Compatibility = new(compatibilityFailure.bootstrapCompatibility)
	}
	if configFailure, ok := errors.AsType[*bootstrapConfigFailure](cause); ok {
		event.Classification = bootstrapProbeConfigInvalid
		event.Config = configFailure.detail()
	}
	if err := writeBootstrapEvent(cmd, event); err != nil {
		return err
	}
	return cause
}
