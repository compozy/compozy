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
	if err := m.binaryApplier.RestoreBinary(
		applied.BackupPath,
		applied.TargetPath,
		executableBinaryMode(backupInfo.Mode().Perm(), 0),
	); err != nil {
		return err
	}
	if applied.InstallMethod == InstallMethodDesktopApp {
		return rewriteDesktopProvenance(m.homePaths, applied.TargetPath)
	}
	return nil
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
		state.Message = "CompozyOS self-update is unavailable for dev builds."
		state.Recommendation = "Install a tagged CompozyOS release binary or rebuild from source."
		return state
	case latest == nil || strings.TrimSpace(latest.Version) == "":
		state.Status = StatusFailed
		state.Message = "The latest CompozyOS release metadata for the active channel is unavailable."
		return state
	}

	comparison, err := compareVersions(state.CurrentVersion, latest.Version)
	if err != nil {
		state.Status = StatusUnsupported
		state.Message = "The running CompozyOS version cannot be compared against published releases."
		state.LastError = err.Error()
		return state
	}

	state.Available = comparison < 0
	supportedPlatform := supportsDirectBinarySelfUpdate(m.runtimeOS, m.runtimeArch)
	state.Supported = !state.Managed &&
		(state.InstallMethod == string(InstallMethodDirectBinary) ||
			state.InstallMethod == string(InstallMethodDesktopApp)) &&
		supportedPlatform

	switch {
	case state.Managed && state.Available:
		state.Status = StatusAvailable
		state.Message = "A newer CompozyOS release is available through its managing install surface; " +
			"no local update was performed."
		state.Recommendation = m.updateRecommendation(state.InstallMethod, latest)
	case state.Managed:
		state.Status = StatusUpToDate
		state.Message = "CompozyOS is already current on its release channel. " +
			"Managed installs stay on their owning update surface."
		state.Recommendation = m.updateRecommendation(state.InstallMethod, latest)
	case !state.Supported && state.Available:
		state.Status = StatusUnsupported
		state.Message = "A newer CompozyOS release is available on the active channel, " +
			"but this install method does not support in-place updates."
		state.Recommendation = m.updateRecommendation(state.InstallMethod, latest)
	case !state.Supported:
		state.Status = StatusUnsupported
		state.Message = "This CompozyOS install method does not support in-place self-update."
		state.Recommendation = m.updateRecommendation(state.InstallMethod, latest)
	case state.Available:
		state.Status = StatusAvailable
		state.Message = "A newer CompozyOS release is available on the active channel."
		state.Recommendation = "Run `compozy update`."
	default:
		state.Status = StatusUpToDate
		state.Message = "CompozyOS is already current on its release channel."
	}

	if state.InstallMethod == string(InstallMethodDirectBinary) && !supportedPlatform {
		state.Recommendation = ManualDirectBinaryRecommendation(state.ReleaseURL, m.runtimeOS)
	}
	return state
}

func (m *Manager) archiveBinaryName() string {
	if m.runtimeOS == runtimeOSWindows {
		return compozyWindowsBinaryName
	}
	return compozyBinaryName
}

func supportsDirectBinarySelfUpdate(runtimeOS string, runtimeArch string) bool {
	if runtimeOS != runtimeOSDarwin && runtimeOS != runtimeOSLinux {
		return false
	}
	return runtimeArch == runtimeArchAMD64 || runtimeArch == runtimeArchARM64
}

func (m *Manager) updateRecommendation(installMethod string, release *Release) string {
	releaseURL := ""
	releaseVersion := ""
	if release != nil {
		releaseURL = strings.TrimSpace(release.ReleaseURL)
		releaseVersion = strings.TrimSpace(release.Version)
	}
	if m.releaseTrack == releaseTrackBeta {
		switch installMethod {
		case string(InstallMethodNPM):
			return "Use `npm install -g @compozy/cli@beta`."
		case string(InstallMethodGoInstall):
			return "Use `go install " + goInstallModulePath + "@" + releaseVersion + "`."
		case string(InstallMethodDirectBinary):
			return ManualDirectBinaryRecommendation(releaseURL, m.runtimeOS)
		default:
			return "Install the v0.3 beta with `curl -fsSL https://compozy.com/install.sh | sh`."
		}
	}

	switch installMethod {
	case string(InstallMethodHomebrew):
		return "Use `brew upgrade compozy/compozy/compozy`."
	case string(InstallMethodNPM):
		return "Use `npm update -g @compozy/cli`."
	case string(InstallMethodAPT):
		return "Use `sudo apt update && sudo apt upgrade compozy`."
	case string(InstallMethodDNF):
		return "Use `sudo dnf upgrade compozy`."
	case string(InstallMethodRPM):
		return "Upgrade the installed RPM package through your system package tooling."
	case string(InstallMethodScoop):
		return "Use `scoop update compozy`."
	case string(InstallMethodGoInstall):
		return "Use `go install " + goInstallModulePath + "@latest`."
	case string(InstallMethodDirectBinary):
		return ManualDirectBinaryRecommendation(releaseURL, m.runtimeOS)
	default:
		if strings.TrimSpace(installMethod) == "" {
			return ""
		}
		return "Use the package manager or installer that manages this CompozyOS binary instead of mutating it in place."
	}
}

// ManualDirectBinaryRecommendation returns the recovery path when in-place update is unavailable.
func ManualDirectBinaryRecommendation(releaseURL string, runtimeOS string) string {
	if runtimeOS == runtimeOSWindows {
		if strings.TrimSpace(releaseURL) == "" {
			return "Download the latest CompozyOS Windows release archive and replace `compozy.exe` manually."
		}
		return "Download the latest CompozyOS Windows release archive from " + releaseURL + " and replace `compozy.exe` manually."
	}
	if strings.TrimSpace(releaseURL) == "" {
		return "Install the verified CompozyOS binary with `curl -fsSL https://compozy.com/install.sh | sh`."
	}
	return "Download the latest CompozyOS release archive from " + releaseURL + " and replace the binary manually."
}

func defaultRunCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
