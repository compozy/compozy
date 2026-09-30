package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Masterminds/semver/v3"
	compozyconfig "github.com/compozy/compozy/internal/config"
	shellquote "github.com/kballard/go-shellquote"
)

const appBundleIdentifier = "com.compozy.os"

const platformWindows = "windows"

type appRegistrationCommand func(context.Context, string, ...string) (string, error)

func resolvePlatformAppInstallation(
	ctx context.Context,
	_ compozyconfig.HomePaths,
) (appInstallation, error) {
	switch runtime.GOOS {
	case "darwin":
		return resolveDarwinAppInstallation(ctx, runAppRegistrationCommand)
	case platformWindows:
		return resolveWindowsAppInstallation(ctx, runAppRegistrationCommand)
	default:
		return resolveLinuxAppInstallation()
	}
}

func resolveDarwinAppInstallation(
	ctx context.Context,
	run appRegistrationCommand,
) (appInstallation, error) {
	query := "kMDItemCFBundleIdentifier == '" + appBundleIdentifier + "'"
	output, err := run(ctx, "mdfind", query)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return appInstallation{}, nil
		}
		return appInstallation{}, fmt.Errorf("query macOS app registration: %w", err)
	}
	bundlePath := firstNonEmptyLine(output)
	if bundlePath == "" {
		return appInstallation{}, nil
	}
	versionOutput, err := run(
		ctx,
		"defaults",
		"read",
		filepath.Join(bundlePath, "Contents", "Info"),
		"CFBundleShortVersionString",
	)
	if err != nil {
		return appInstallation{Installed: true}, nil
	}
	return appInstallation{Installed: true, Version: strings.TrimSpace(versionOutput)}, nil
}

func resolveWindowsAppInstallation(
	ctx context.Context,
	run appRegistrationCommand,
) (appInstallation, error) {
	script := "$paths=@('HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\*'," +
		"'HKLM:\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\*');" +
		"$app=Get-ItemProperty $paths -ErrorAction SilentlyContinue | " +
		"Where-Object {$_.DisplayName -eq 'CompozyOS'} | Select-Object -First 1;" +
		"if($app){Write-Output 'installed'; Write-Output $app.DisplayVersion}"
	output, err := run(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return appInstallation{}, nil
		}
		return appInstallation{}, fmt.Errorf("query Windows app registration: %w", err)
	}
	lines := nonEmptyLines(output)
	if len(lines) == 0 || lines[0] != marketplaceInstalledKey {
		return appInstallation{}, nil
	}
	version := ""
	if len(lines) > 1 {
		version = lines[1]
	}
	return appInstallation{Installed: true, Version: version}, nil
}

func runAppRegistrationCommand(ctx context.Context, name string, args ...string) (string, error) {
	output, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func resolveLinuxAppInstallation() (appInstallation, error) {
	return resolveLinuxAppInstallationAt(strings.TrimSpace(os.Getenv("HOME")))
}

func resolveLinuxAppInstallationAt(homeDir string) (appInstallation, error) {
	if homeDir != "" {
		matches, err := filepath.Glob(filepath.Join(homeDir, "Applications", "CompozyOS-*-linux-*.AppImage"))
		if err != nil {
			return appInstallation{}, err
		}
		var installed appInstallation
		for _, path := range matches {
			executable, executableFound, err := inspectLinuxAppExecutable(path)
			if err != nil {
				return appInstallation{}, err
			}
			if !executableFound {
				continue
			}
			version, _, found := strings.Cut(strings.TrimPrefix(filepath.Base(path), "CompozyOS-"), "-linux-")
			if found {
				candidate, err := semver.NewVersion(version)
				if err != nil {
					continue
				}
				if installed.Installed {
					previous, err := semver.NewVersion(installed.Version)
					if err == nil && !candidate.GreaterThan(previous) {
						continue
					}
				}
				installed = appInstallation{Installed: true, Version: version, Executable: executable}
			}
		}
		if installed.Installed {
			return installed, nil
		}
	}
	candidates := make([]string, 0, 4)
	if homeDir != "" {
		candidates = append(
			candidates,
			filepath.Join(homeDir, ".local", "share", "applications", appBundleIdentifier+".desktop"),
			filepath.Join(homeDir, ".local", "share", "applications", "compozyos.desktop"),
		)
	}
	candidates = append(candidates,
		filepath.Join(string(filepath.Separator), "usr", "share", "applications", appBundleIdentifier+".desktop"),
		filepath.Join(string(filepath.Separator), "usr", "share", "applications", "compozyos.desktop"),
	)
	for _, path := range candidates {
		if strings.TrimSpace(path) == "" {
			continue
		}
		installation, found, err := parseDesktopRegistration(path)
		if err != nil {
			return appInstallation{}, err
		}
		if found {
			return installation, nil
		}
	}
	if executable, err := exec.LookPath("compozyos"); err == nil {
		return appInstallation{Installed: true, Executable: executable}, nil
	}
	return appInstallation{}, nil
}

func parseDesktopRegistration(path string) (appInstallation, bool, error) {
	// #nosec G703 -- path is selected from fixed platform registration candidates or a test fixture.
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return appInstallation{}, false, nil
	}
	if err != nil {
		return appInstallation{}, false, fmt.Errorf("open Linux desktop registration %q: %w", path, err)
	}
	name := ""
	version := ""
	executable := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, found := strings.Cut(scanner.Text(), "=")
		if !found {
			continue
		}
		switch strings.TrimSpace(key) {
		case "Exec":
			argv, err := shellquote.Split(strings.ReplaceAll(strings.TrimSpace(value), `\\`, `\`))
			if err == nil && len(argv) > 0 {
				executable = strings.ReplaceAll(argv[0], "%%", "%")
			}
		case cliNameValue:
			name = strings.TrimSpace(value)
		case "X-Compozy-Version", "X-AppImage-Version":
			version = strings.TrimSpace(value)
		}
	}
	if err := scanner.Err(); err != nil {
		closeErr := file.Close()
		if closeErr != nil {
			return appInstallation{}, false, errors.Join(err, closeErr)
		}
		return appInstallation{}, false, fmt.Errorf("read Linux desktop registration %q: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return appInstallation{}, false, fmt.Errorf("close Linux desktop registration %q: %w", path, err)
	}
	if name != "CompozyOS" {
		return appInstallation{}, false, nil
	}
	executable, found, err := inspectLinuxAppExecutable(executable)
	if err != nil || !found {
		return appInstallation{}, false, err
	}
	return appInstallation{Installed: true, Version: version, Executable: executable}, true, nil
}

func inspectLinuxAppExecutable(executable string) (string, bool, error) {
	if !filepath.IsAbs(executable) {
		resolved, err := exec.LookPath(executable)
		if err != nil {
			return "", false, nil
		}
		executable = resolved
	}
	if !strings.EqualFold(filepath.Base(executable), "compozyos") && !strings.HasSuffix(executable, ".AppImage") {
		return "", false, nil
	}
	canonical, err := filepath.EvalSymlinks(executable)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("resolve Linux app executable: %w", err)
	}
	directory, err := os.OpenRoot(filepath.Dir(canonical))
	if err != nil {
		return "", false, fmt.Errorf("open Linux app executable directory: %w", err)
	}
	info, statErr := directory.Stat(filepath.Base(canonical))
	closeErr := directory.Close()
	if errors.Is(statErr, os.ErrNotExist) && closeErr == nil {
		return "", false, nil
	}
	if err := errors.Join(statErr, closeErr); err != nil {
		return "", false, fmt.Errorf("inspect Linux app executable: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", false, nil
	}
	return canonical, true, nil
}

func firstNonEmptyLine(value string) string {
	lines := nonEmptyLines(value)
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}

func nonEmptyLines(value string) []string {
	lines := make([]string, 0, 2)
	for line := range strings.SplitSeq(value, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}
