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

	compozyconfig "github.com/compozy/compozy/internal/config"
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
		return appInstallation{}, fmt.Errorf("read macOS app version: %w", err)
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
	candidates := make([]string, 0, 4)
	if homeDir := strings.TrimSpace(os.Getenv("HOME")); homeDir != "" {
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
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, found := strings.Cut(scanner.Text(), "=")
		if !found {
			continue
		}
		switch strings.TrimSpace(key) {
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
	return appInstallation{Installed: true, Version: version}, true, nil
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
