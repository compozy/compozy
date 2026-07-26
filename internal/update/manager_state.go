package update

import (
	"context"

	"errors"
	"fmt"

	"os"
	"os/exec"

	"strings"
	"time"
)

// Restore rolls back an applied binary swap using the preserved sibling backup.
func (m *Manager) Restore(applied AppliedBinary) error {
	if strings.TrimSpace(applied.BackupPath) == "" || strings.TrimSpace(applied.TargetPath) == "" {
		return errors.New("update: applied binary rollback paths are required")
	}
	backupInfo, err := os.Stat(applied.BackupPath)
	if err != nil {
		return fmt.Errorf("update: stat backup executable %q: %w", applied.BackupPath, err)
	}
	return m.binaryApplier.RestoreBinary(
		applied.BackupPath,
		applied.TargetPath,
		executableBinaryMode(backupInfo.Mode().Perm(), 0),
	)
}

// Finalize removes the preserved sibling backup after the full update flow succeeds.
func (m *Manager) Finalize(applied AppliedBinary) error {
	if strings.TrimSpace(applied.BackupPath) == "" {
		return nil
	}
	if err := os.Remove(applied.BackupPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("update: remove backup binary %q: %w", applied.BackupPath, err)
	}
	return nil
}

func (m *Manager) composeState(install installInfo, latest *Release, checkedAt *time.Time) State {
	state := State{
		Managed:        install.Managed,
		InstallMethod:  strings.TrimSpace(install.Method),
		CurrentVersion: strings.TrimSpace(m.currentVersion),
		CheckedAt:      checkedAt,
	}
	if latest != nil {
		state.LatestVersion = strings.TrimSpace(latest.Version)
		state.ReleaseURL = strings.TrimSpace(latest.ReleaseURL)
	}
	if state.InstallMethod == "" {
		state.InstallMethod = string(InstallMethodUnknown)
	}

	switch {
	case isDevVersion(state.CurrentVersion):
		state.Status = StatusUnsupported
		state.Message = "AGH self-update is unavailable for dev builds."
		state.Recommendation = "Install a tagged AGH release binary or rebuild from source."
		return state
	case latest == nil || strings.TrimSpace(latest.Version) == "":
		state.Status = StatusFailed
		state.Message = "The latest stable AGH release metadata is unavailable."
		return state
	}

	comparison, err := compareVersions(state.CurrentVersion, latest.Version)
	if err != nil {
		state.Status = StatusUnsupported
		state.Message = "The running AGH version cannot be compared against published releases."
		state.LastError = err.Error()
		return state
	}

	state.Available = comparison < 0
	supportedPlatform := supportsDirectBinarySelfUpdate(m.runtimeOS, m.runtimeArch)
	state.Supported = !state.Managed &&
		state.InstallMethod == string(InstallMethodDirectBinary) &&
		supportedPlatform

	switch {
	case state.Managed && state.Available:
		state.Status = StatusDeferred
		state.Message = "AGH is managed by an external package manager; no local update was performed."
		state.Recommendation = updateRecommendation(state.InstallMethod, state.ReleaseURL)
	case state.Managed:
		state.Status = StatusCurrent
		state.Message = "AGH is already on the latest stable release. Managed installs stay on the package manager path."
		state.Recommendation = updateRecommendation(state.InstallMethod, state.ReleaseURL)
	case !state.Supported && state.Available:
		state.Status = StatusUnsupported
		state.Message = "A newer stable AGH release is available, but this install method does not support in-place updates."
		state.Recommendation = updateRecommendation(state.InstallMethod, state.ReleaseURL)
	case !state.Supported:
		state.Status = StatusUnsupported
		state.Message = "This AGH install method does not support in-place self-update."
		state.Recommendation = updateRecommendation(state.InstallMethod, state.ReleaseURL)
	case state.Available:
		state.Status = StatusAvailable
		state.Message = "A newer stable AGH release is available."
		state.Recommendation = "Run `agh update`."
	default:
		state.Status = StatusCurrent
		state.Message = "AGH is already on the latest stable release."
	}

	if state.InstallMethod == string(InstallMethodDirectBinary) && !supportedPlatform {
		state.Recommendation = manualDirectBinaryRecommendation(state.ReleaseURL, m.runtimeOS)
	}
	return state
}

func (m *Manager) archiveBinaryName() string {
	if m.runtimeOS == runtimeOSWindows {
		return aghWindowsBinaryName
	}
	return aghBinaryName
}

func supportsDirectBinarySelfUpdate(runtimeOS string, runtimeArch string) bool {
	if runtimeOS != runtimeOSDarwin && runtimeOS != runtimeOSLinux {
		return false
	}
	return runtimeArch == runtimeArchAMD64 || runtimeArch == runtimeArchARM64
}

func updateRecommendation(installMethod string, releaseURL string) string {
	switch installMethod {
	case string(InstallMethodHomebrew):
		return "Use `brew upgrade compozy/compozy/agh`."
	case string(InstallMethodNPM):
		return "Use `npm update -g @compozy/agh`."
	case string(InstallMethodAPT):
		return "Use `sudo apt update && sudo apt upgrade agh`."
	case string(InstallMethodDNF):
		return "Use `sudo dnf upgrade agh`."
	case string(InstallMethodRPM):
		return "Upgrade the installed RPM package through your system package tooling."
	case string(InstallMethodScoop):
		return "Use `scoop update agh`."
	case string(InstallMethodGoInstall):
		return "Use `go install " + goInstallModulePath + "@latest`."
	case string(InstallMethodDirectBinary):
		return manualDirectBinaryRecommendation(releaseURL, "")
	default:
		if strings.TrimSpace(installMethod) == "" {
			return ""
		}
		return "Use the package manager or installer that manages this AGH binary instead of mutating it in place."
	}
}

func manualDirectBinaryRecommendation(releaseURL string, runtimeOS string) string {
	if runtimeOS == runtimeOSWindows {
		if strings.TrimSpace(releaseURL) == "" {
			return "Download the latest AGH Windows release archive and replace `agh.exe` manually."
		}
		return "Download the latest AGH Windows release archive from " + releaseURL + " and replace `agh.exe` manually."
	}
	if strings.TrimSpace(releaseURL) == "" {
		return "Download the latest AGH release archive and replace the binary manually."
	}
	return "Download the latest AGH release archive from " + releaseURL + " and replace the binary manually."
}

func defaultRunCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
