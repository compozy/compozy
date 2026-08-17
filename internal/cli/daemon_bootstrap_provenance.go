package cli

import (
	"errors"
	"fmt"
	"os"

	compozyconfig "github.com/compozy/compozy/internal/config"
	compozyupdate "github.com/compozy/compozy/internal/update"
	"github.com/spf13/cobra"
)

type bootstrapProvisionRequest struct {
	resolution    bootstrapResolution
	homePaths     compozyconfig.HomePaths
	bundlePath    string
	installedPath string
	provenance    compozyupdate.DesktopProvenanceMetadata
}

func provisionBootstrapRuntime(cmd *cobra.Command, request bootstrapProvisionRequest) error {
	if request.resolution != bootstrapResolutionProvision {
		return nil
	}
	if err := writeBootstrapEvent(cmd, bootstrapEvent{
		Type: bootstrapEventType, Phase: bootstrapPhaseProvision, Status: bootstrapStatusStarted,
		Resolution: request.resolution, Message: "Provisioning the bundled CompozyOS runtime.",
	}); err != nil {
		return err
	}
	if err := provisionBundledRuntime(request.bundlePath, request.installedPath); err != nil {
		return writeBootstrapFailure(cmd, bootstrapPhaseProvision, bootstrapProbeUnavailable, err)
	}
	if err := compozyupdate.WriteDesktopProvenance(
		request.homePaths,
		request.installedPath,
		request.provenance,
	); err != nil {
		removeErr := os.Remove(request.installedPath)
		if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("cli: remove unowned provisioned runtime: %w", removeErr))
		}
		return writeBootstrapFailure(cmd, bootstrapPhaseProvision, bootstrapProbeUnavailable, err)
	}
	return writeBootstrapEvent(cmd, bootstrapEvent{
		Type: bootstrapEventType, Phase: bootstrapPhaseProvision, Status: bootstrapStatusCompleted,
		Resolution: request.resolution, Message: "Provisioned the bundled CompozyOS runtime.",
	})
}
