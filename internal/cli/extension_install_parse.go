package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	registrygit "github.com/compozy/compozy/internal/registry/gitsrc"
)

var windowsAbsoluteInstallPath = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

type extensionInstallPlan struct {
	Attempts []contract.InstallExtensionRequest
}

func parseExtensionInstallPlan(
	input string,
	version string,
	asset string,
	allowUnverified bool,
) (extensionInstallPlan, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return extensionInstallPlan{}, errors.New("cli: extension install source is required")
	}

	request := contract.InstallExtensionRequest{
		Version:         strings.TrimSpace(version),
		Asset:           strings.TrimSpace(asset),
		AllowUnverified: allowUnverified,
	}
	switch {
	case isPathLikeExtensionInstallRef(trimmed):
		path, err := resolveExtensionInstallPath(trimmed)
		if err != nil {
			return extensionInstallPlan{}, err
		}
		request.Source = contract.InstallExtensionSourceLocalPath
		request.Ref = path
		return extensionInstallPlan{Attempts: []contract.InstallExtensionRequest{request}}, nil
	case strings.HasPrefix(trimmed, "github:"):
		request.Source = contract.InstallExtensionSourceGitHub
		request.Ref = strings.TrimSpace(strings.TrimPrefix(trimmed, "github:"))
		if err := validateGitHubExtensionRef(request.Ref); err != nil {
			return extensionInstallPlan{}, err
		}
		return extensionInstallPlan{Attempts: []contract.InstallExtensionRequest{request}}, nil
	case strings.HasPrefix(trimmed, "git:"):
		request.Source = contract.InstallExtensionSourceGit
		request.Ref = strings.TrimSpace(strings.TrimPrefix(trimmed, "git:"))
		if err := validateGitExtensionRef(request.Ref); err != nil {
			return extensionInstallPlan{}, err
		}
		return extensionInstallPlan{Attempts: []contract.InstallExtensionRequest{request}}, nil
	default:
		if err := validateGitHubExtensionRef(trimmed); err != nil {
			path, pathErr := resolveExtensionInstallPath(trimmed)
			if pathErr != nil {
				return extensionInstallPlan{}, pathErr
			}
			request.Source = contract.InstallExtensionSourceLocalPath
			request.Ref = path
			return extensionInstallPlan{Attempts: []contract.InstallExtensionRequest{request}}, nil
		}
		request.Ref = trimmed
		request.Source = contract.InstallExtensionSourceCurated
		githubRequest := request
		githubRequest.Source = contract.InstallExtensionSourceGitHub
		return extensionInstallPlan{Attempts: []contract.InstallExtensionRequest{request, githubRequest}}, nil
	}
}

func isPathLikeExtensionInstallRef(value string) bool {
	return filepath.IsAbs(value) ||
		strings.HasPrefix(value, "./") ||
		strings.HasPrefix(value, "../") ||
		strings.HasPrefix(value, `.\`) ||
		strings.HasPrefix(value, `..\`) ||
		windowsAbsoluteInstallPath.MatchString(value)
}

func resolveExtensionInstallPath(value string) (string, error) {
	absPath, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("cli: resolve extension install path %q: %w", value, err)
	}
	info, err := os.Stat(absPath)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("cli: extension install path %q does not exist", value)
	}
	if err != nil {
		return "", fmt.Errorf("cli: inspect extension install path %q: %w", value, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("cli: extension install path %q must be a directory", value)
	}
	return filepath.Clean(absPath), nil
}

func validateGitHubExtensionRef(value string) error {
	ref := strings.TrimSpace(value)
	if ref == "" {
		return errors.New("GitHub repository is required")
	}
	repository := ref
	if at := strings.LastIndex(repository, "@"); at > 0 {
		repository = repository[:at]
	}
	parts := strings.Split(repository, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return fmt.Errorf("GitHub repository %q must use owner/repo format", ref)
	}
	return nil
}

func validateGitExtensionRef(value string) error {
	ref := strings.TrimSpace(value)
	if ref == "" {
		return errors.New("cli: git repository URL is required")
	}
	repository, _ := splitExtensionDistributionRef(ref)
	if err := registrygit.ValidateRepositoryRef(repository); err != nil {
		return fmt.Errorf("cli: invalid git repository URL: %w", err)
	}
	return nil
}

func splitExtensionDistributionRef(value string) (string, string) {
	trimmed := strings.TrimSpace(value)
	index := strings.LastIndex(trimmed, "@")
	if index <= 0 || index == len(trimmed)-1 {
		return trimmed, ""
	}
	if scheme := strings.Index(trimmed, "://"); scheme >= 0 {
		hostEnd := strings.Index(trimmed[scheme+3:], "/")
		if hostEnd < 0 || index < scheme+3+hostEnd {
			return trimmed, ""
		}
	}
	return strings.TrimSpace(trimmed[:index]), strings.TrimSpace(trimmed[index+1:])
}
