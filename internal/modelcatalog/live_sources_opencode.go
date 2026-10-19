package modelcatalog

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providerexec"
)

// isAutomaticOpenCodeDiscovery reports whether the built-in OpenCode discovery
// path owns this source. Explicit discovery commands bypass version detection;
// explicit endpoints and disabled discovery never reach the command branch.
func isAutomaticOpenCodeDiscovery(providerID string, provider compozyconfig.ProviderConfig) bool {
	if compozyconfig.CanonicalProviderName(providerID) != liveSourcesOpencodeKey {
		return false
	}
	return strings.TrimSpace(provider.Models.Discovery.Command) == ""
}

// parseOpenCodeMajorVersion detects one complete semantic version from
// `--version` output. It never scans diagnostic text for a version number.
func parseOpenCodeMajorVersion(output string) (uint64, error) {
	trimmed := strings.TrimSpace(output)
	if rest, ok := strings.CutPrefix(trimmed, "opencode "); ok {
		trimmed = strings.TrimSpace(rest)
	}
	version, err := semver.StrictNewVersion(strings.TrimPrefix(strings.TrimSpace(trimmed), "v"))
	if err != nil {
		return 0, fmt.Errorf("model catalog: could not determine OpenCode version")
	}
	major := version.Major()
	if major != 1 && major != 2 {
		return 0, fmt.Errorf(
			"model catalog: unsupported OpenCode major version %d; supported versions are 1 and 2",
			major,
		)
	}
	return major, nil
}

func (s *LiveProviderSource) listOpenCode(
	ctx context.Context,
	provider compozyconfig.ProviderConfig,
	env []string,
	expectedFingerprint string,
	timeout time.Duration,
	now time.Time,
) ([]ModelRow, error) {
	if ctx == nil {
		return nil, fmt.Errorf("model catalog: live provider context is required")
	}
	resolved, err := providerexec.ResolveOpenCodeCommand(provider, env, s.workingDir)
	if err != nil {
		return nil, err
	}
	base, err := providerexec.DeriveOpenCodeDiscoveryBase(resolved)
	if err != nil {
		return nil, err
	}
	// Discovery must run under the same private assignments the session launch applies.
	runEnv := providerexec.ApplyOpenCodeAssignments(env, base.Environment)
	if strings.TrimSpace(expectedFingerprint) != "" {
		current, fingerprintErr := s.CatalogExecutionFingerprint()
		if fingerprintErr != nil {
			return nil, fingerprintErr
		}
		if current != strings.TrimSpace(expectedFingerprint) {
			return nil, fmt.Errorf("model catalog: OpenCode discovery invocation changed during refresh")
		}
	}
	versionOutput, err := s.runOpenCodeVersion(ctx, base.Executable, base.Args, runEnv, timeout)
	if err != nil {
		return nil, err
	}
	major, err := parseOpenCodeMajorVersion(versionOutput)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	listArgs := append(append([]string(nil), base.Args...), "models", "--verbose")
	decode := parseOpenCodeV1ModelRows
	stdoutDetail := true
	if major == 2 {
		values := url.Values{}
		values.Set("location[directory]", s.workingDir)
		listArgs = append(
			append([]string(nil), base.Args...),
			"api", "--standalone", "GET", "/api/model?"+values.Encode(),
		)
		decode = parseOpenCodeV2ModelRows
		// The v2 envelope may carry provider-specific settings; its raw
		// output never becomes a diagnostic, even redacted.
		stdoutDetail = false
	}
	listOutput, err := s.runOpenCodeList(ctx, base.Executable, listArgs, runEnv, timeout, stdoutDetail)
	if err != nil {
		return nil, err
	}
	return decode(s.providerID, listOutput, now)
}

func (s *LiveProviderSource) openCodeDiscoveryRequest(
	command string,
	args []string,
	env []string,
	timeout time.Duration,
) DiscoveryCommandRequest {
	return DiscoveryCommandRequest{
		ProviderID: s.providerID,
		Command:    command,
		Args:       args,
		Dir:        s.workingDir,
		Env:        env,
		Timeout:    timeout,
	}
}

// runOpenCodeVersion runs `--version` without echoing its stdout on failure.
func (s *LiveProviderSource) runOpenCodeVersion(
	ctx context.Context,
	command string,
	prefix []string,
	env []string,
	timeout time.Duration,
) (string, error) {
	args := append(append([]string(nil), prefix...), "--version")
	result, err := s.commandExecutor.RunDiscoveryCommand(
		ctx,
		s.openCodeDiscoveryRequest(command, args, env, timeout),
	)
	if err != nil {
		if detail := RedactString(firstNonEmptyLine(result.Stderr)); detail != "" {
			return "", fmt.Errorf("model catalog: could not determine OpenCode version: %s: %w", detail, err)
		}
		return "", fmt.Errorf("model catalog: could not determine OpenCode version: %w", err)
	}
	if result.ExitCode != 0 {
		detail := RedactString(firstNonEmptyLine(result.Stderr))
		if detail == "" {
			detail = noDiscoveryDiagnostic
		}
		return "", fmt.Errorf("model catalog: could not determine OpenCode version: %s", detail)
	}
	return result.Stdout, nil
}

// runOpenCodeList runs one version-selected catalog command with the shared
// execution/redaction handling used by the generic command path.
func (s *LiveProviderSource) runOpenCodeList(
	ctx context.Context,
	command string,
	args []string,
	env []string,
	timeout time.Duration,
	stdoutDetail bool,
) (string, error) {
	result, err := s.commandExecutor.RunDiscoveryCommand(
		ctx,
		s.openCodeDiscoveryRequest(command, args, env, timeout),
	)
	if err != nil {
		detail := firstNonEmptyLine(result.Stderr)
		if detail == "" && stdoutDetail {
			detail = firstNonEmptyLine(result.Stdout)
		}
		if detail != "" {
			return "", fmt.Errorf("%w: %s", err, RedactString(detail))
		}
		return "", err
	}
	if result.ExitCode != 0 {
		detail := firstNonEmptyLine(result.Stderr)
		if detail == "" && stdoutDetail {
			detail = firstNonEmptyLine(result.Stdout)
		}
		if detail == "" {
			detail = noDiscoveryDiagnostic
		}
		return "", fmt.Errorf(
			"model catalog: discovery command for %q exited %d: %s",
			s.providerID,
			result.ExitCode,
			RedactString(detail),
		)
	}
	return result.Stdout, nil
}
