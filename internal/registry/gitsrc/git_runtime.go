package gitsrc

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

var minimumSecureGitVersion = gitVersion{major: 2, minor: 37}

type gitVersion struct {
	major int
	minor int
	patch int
}

func (c *Client) requireSupportedGit(ctx context.Context, executable string) error {
	output, err := c.version(ctx, executable)
	if err != nil {
		return fmt.Errorf("gitsrc: inspect Git version: %w", err)
	}
	version, err := parseGitVersion(output)
	if err != nil {
		return err
	}
	if version.less(minimumSecureGitVersion) {
		return fmt.Errorf("%w: found %d.%d.%d", ErrGitVersionUnsupported, version.major, version.minor, version.patch)
	}
	return nil
}

func probeGitVersion(ctx context.Context, executable string) (string, error) {
	command := exec.CommandContext(ctx, executable, "--version")
	command.Env = isolatedGitEnvironment()
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("run git --version: %w", err)
	}
	return string(output), nil
}

func parseGitVersion(output string) (gitVersion, error) {
	value := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(output), "git version "))
	if value == "" {
		return gitVersion{}, fmt.Errorf(
			"%w: unrecognized version %q",
			ErrGitVersionUnsupported,
			strings.TrimSpace(output),
		)
	}
	core := strings.Fields(value)[0]
	parts := strings.Split(core, ".")
	if len(parts) < 2 {
		return gitVersion{}, fmt.Errorf("%w: unrecognized version %q", ErrGitVersionUnsupported, core)
	}
	numbers := [3]int{}
	for index := range min(len(parts), len(numbers)) {
		digits := leadingDigits(parts[index])
		if digits == "" {
			return gitVersion{}, fmt.Errorf("%w: unrecognized version %q", ErrGitVersionUnsupported, core)
		}
		parsed, err := strconv.Atoi(digits)
		if err != nil {
			return gitVersion{}, fmt.Errorf("%w: parse version %q: %v", ErrGitVersionUnsupported, core, err)
		}
		numbers[index] = parsed
	}
	return gitVersion{major: numbers[0], minor: numbers[1], patch: numbers[2]}, nil
}

func leadingDigits(value string) string {
	end := 0
	for end < len(value) && value[end] >= '0' && value[end] <= '9' {
		end++
	}
	return value[:end]
}

func (v gitVersion) less(other gitVersion) bool {
	if v.major != other.major {
		return v.major < other.major
	}
	if v.minor != other.minor {
		return v.minor < other.minor
	}
	return v.patch < other.patch
}

func isolatedGitEnvironment() []string {
	environment := os.Environ()
	filtered := make([]string, 0, len(environment)+4)
	for _, entry := range environment {
		name, _, found := strings.Cut(entry, "=")
		if !found || blockedGitEnvironmentName(name) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return append(
		filtered,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_TERMINAL_PROMPT=0",
		"GCM_INTERACTIVE=never",
	)
}

func blockedGitEnvironmentName(name string) bool {
	upper := strings.ToUpper(strings.TrimSpace(name))
	if strings.HasPrefix(upper, "GIT_") || upper == "GCM_INTERACTIVE" {
		return true
	}
	switch upper {
	case "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY":
		return true
	default:
		return false
	}
}
