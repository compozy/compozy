package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
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
		compatibility := compatibilityFailure.bootstrapCompatibility
		event.Compatibility = &compatibility
	}
	if err := writeBootstrapEvent(cmd, event); err != nil {
		return err
	}
	return cause
}
