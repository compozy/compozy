package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

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

type bootstrapRuntimeInstall struct {
	installed  bool
	mode       os.FileMode
	backupPath string
}

func provisionBootstrapRuntime(cmd *cobra.Command, request *bootstrapProvisionRequest) error {
	if request.resolution != bootstrapResolutionProvision {
		return nil
	}
	if err := writeBootstrapEvent(cmd, bootstrapEvent{
		Type: bootstrapEventType, Phase: bootstrapPhaseProvision, Status: bootstrapStatusStarted,
		Resolution: request.resolution, Message: "Provisioning the bundled CompozyOS runtime.",
	}); err != nil {
		return err
	}
	install, err := installBootstrapRuntime(request)
	if err != nil {
		return writeBootstrapFailure(cmd, bootstrapPhaseProvision, bootstrapProbeUnavailable, err)
	}
	if err = compozyupdate.WriteDesktopProvenance(
		request.homePaths,
		request.installedPath,
		request.provenance,
	); err != nil {
		err = errors.Join(err, rollbackBootstrapRuntime(request, install))
		return writeBootstrapFailure(cmd, bootstrapPhaseProvision, bootstrapProbeUnavailable, err)
	}
	if err := removeBootstrapBackup(install.backupPath); err != nil {
		return writeBootstrapFailure(cmd, bootstrapPhaseProvision, bootstrapProbeUnavailable, err)
	}
	return writeBootstrapEvent(cmd, bootstrapEvent{
		Type: bootstrapEventType, Phase: bootstrapPhaseProvision, Status: bootstrapStatusCompleted,
		Resolution: request.resolution, Message: "Provisioned the bundled CompozyOS runtime.",
	})
}

func installBootstrapRuntime(request *bootstrapProvisionRequest) (bootstrapRuntimeInstall, error) {
	installedInfo, err := os.Stat(request.installedPath)
	if errors.Is(err, os.ErrNotExist) {
		return bootstrapRuntimeInstall{}, provisionBundledRuntime(request.bundlePath, request.installedPath)
	}
	if err != nil {
		return bootstrapRuntimeInstall{}, err
	}
	if !compozyupdate.RuntimeOwnedByDesktopApp(request.homePaths, request.installedPath) {
		return bootstrapRuntimeInstall{}, errors.New(
			"cli: refusing to replace a runtime not owned by the desktop app",
		)
	}
	install := bootstrapRuntimeInstall{installed: true, mode: installedInfo.Mode().Perm()}
	install.backupPath, err = bootstrapReplacementBackupPath(request.installedPath)
	if err != nil {
		return bootstrapRuntimeInstall{}, err
	}
	if err := compozyupdate.ApplyBinaryReplacement(
		request.bundlePath,
		request.installedPath,
		install.backupPath,
		install.mode,
	); err != nil {
		return bootstrapRuntimeInstall{}, cleanupBootstrapBackupAfterApplyFailure(
			err,
			request.installedPath,
			install.backupPath,
		)
	}
	return install, nil
}

func rollbackBootstrapRuntime(
	request *bootstrapProvisionRequest,
	install bootstrapRuntimeInstall,
) error {
	if !install.installed {
		if err := os.Remove(request.installedPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("cli: remove unowned provisioned runtime: %w", err)
		}
		return nil
	}
	restoreErr := compozyupdate.RestoreBinaryReplacement(
		install.backupPath,
		request.installedPath,
		install.mode,
	)
	if restoreErr != nil {
		return errors.Join(
			restoreErr,
			fmt.Errorf("cli: desktop runtime recovery copy retained at %q", install.backupPath),
		)
	}
	return removeBootstrapBackup(install.backupPath)
}

func bootstrapReplacementBackupPath(targetPath string) (string, error) {
	file, err := os.CreateTemp(filepath.Dir(targetPath), ".compozy-bootstrap-backup-*")
	if err != nil {
		return "", fmt.Errorf("cli: reserve desktop runtime backup path: %w", err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		removeErr := os.Remove(path)
		if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("cli: remove desktop runtime backup placeholder: %w", removeErr))
		}
		return "", fmt.Errorf("cli: close desktop runtime backup placeholder: %w", err)
	}
	if err := os.Remove(path); err != nil {
		return "", fmt.Errorf("cli: release desktop runtime backup placeholder: %w", err)
	}
	return path, nil
}

func cleanupBootstrapBackupAfterApplyFailure(applyErr error, targetPath string, backupPath string) error {
	_, statErr := os.Stat(targetPath)
	switch {
	case statErr == nil:
		return errors.Join(applyErr, removeBootstrapBackup(backupPath))
	case errors.Is(statErr, os.ErrNotExist):
		return errors.Join(
			applyErr,
			fmt.Errorf("cli: desktop runtime recovery copy retained at %q", backupPath),
		)
	default:
		return errors.Join(applyErr, fmt.Errorf("cli: inspect runtime after replacement failure: %w", statErr))
	}
}

func removeBootstrapBackup(path string) error {
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cli: remove desktop runtime bootstrap backup: %w", err)
	}
	return nil
}
