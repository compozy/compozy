package daemon

import (
	"fmt"
	"slices"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type SessionSandboxMode string

const (
	SessionSandboxModeNone SessionSandboxMode = "none"
	SessionSandboxModeRef  SessionSandboxMode = "ref"
)

type SessionRuntimeMode string

const (
	SessionRuntimeModeEvidence SessionRuntimeMode = "evidence"
)

type SessionPolicy struct {
	Sandbox SessionSandboxPolicy
	Runtime SessionRuntimePolicy
}

type SessionSandboxPolicy struct {
	Mode       SessionSandboxMode
	SandboxRef string
}

type SessionRuntimePolicy struct {
	Mode        SessionRuntimeMode
	Permissions aghconfig.PermissionMode
}

func sessionPolicyFromTaskExecutionProfile(profile *taskpkg.ExecutionProfile) SessionPolicy {
	if profile == nil {
		return SessionPolicy{}
	}
	return SessionPolicy{
		Sandbox: SessionSandboxPolicy{
			Mode:       SessionSandboxMode(profile.Sandbox.Mode.Normalize()),
			SandboxRef: strings.TrimSpace(profile.Sandbox.SandboxRef),
		},
		Runtime: SessionRuntimePolicy{
			Mode: SessionRuntimeMode(profile.Runtime.Mode.Normalize()),
		},
	}
}

func applySessionSandboxPolicy(opts *session.CreateOpts, p SessionPolicy) {
	if opts == nil {
		return
	}
	switch normalizeSessionSandboxMode(p.Sandbox.Mode) {
	case SessionSandboxModeNone:
		opts.DisableSandbox = true
		opts.SandboxRef = ""
	case SessionSandboxModeRef:
		opts.DisableSandbox = false
		opts.SandboxRef = strings.TrimSpace(p.Sandbox.SandboxRef)
	default:
		return
	}
}

func applySessionPermissionPolicy(opts *session.CreateOpts, p SessionPolicy) {
	if opts == nil {
		return
	}
	if permissions := normalizeSessionPermissionMode(p.Runtime.Permissions); permissions != "" {
		opts.Permissions = permissions
	}
	if normalizeSessionRuntimeMode(p.Runtime.Mode) != SessionRuntimeModeEvidence {
		return
	}
	guidance := "Runtime evidence mode is enabled for this task. You may boot local app runtimes, " +
		"run browser or simulator validation, and capture runtime evidence artifacts required by the task."
	if sessionRuntimeEvidenceCanAutoApprove(p) {
		opts.Permissions = aghconfig.PermissionModeApproveAll
	} else {
		guidance += " AGH keeps the configured permission mode because the task profile did not select a sandbox."
	}
	opts.PromptOverlay = joinPromptOverlays(opts.PromptOverlay, guidance)
}

func applyAllowedToolsNarrowing(opts *session.CreateOpts, requested []string) error {
	if opts == nil {
		return nil
	}
	normalized, err := normalizeAllowedToolsForSessionGate(requested)
	if err != nil {
		return err
	}
	opts.AllowedToolsOverride = normalized
	return nil
}

func sessionRuntimeEvidenceCanAutoApprove(p SessionPolicy) bool {
	return normalizeSessionSandboxMode(p.Sandbox.Mode) == SessionSandboxModeRef &&
		strings.TrimSpace(p.Sandbox.SandboxRef) != ""
}

func normalizeSessionSandboxMode(mode SessionSandboxMode) SessionSandboxMode {
	return SessionSandboxMode(strings.ToLower(strings.TrimSpace(string(mode))))
}

func normalizeSessionRuntimeMode(mode SessionRuntimeMode) SessionRuntimeMode {
	return SessionRuntimeMode(strings.ToLower(strings.TrimSpace(string(mode))))
}

func normalizeSessionPermissionMode(mode aghconfig.PermissionMode) aghconfig.PermissionMode {
	return aghconfig.PermissionMode(strings.ToLower(strings.TrimSpace(string(mode))))
}

func normalizeAllowedToolsForSessionGate(requested []string) ([]string, error) {
	if len(requested) == 0 {
		return nil, nil
	}
	normalized := make([]string, 0, len(requested))
	seen := make(map[string]struct{}, len(requested))
	for idx, raw := range requested {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return nil, fmt.Errorf("%w: allowed_tools[%d] is required", session.ErrValidation, idx)
		}
		id := toolspkg.ToolID(trimmed)
		if err := id.Validate(); err != nil {
			return nil, fmt.Errorf(
				"%w: allowed_tools[%d] %q must be a canonical ToolID: %w",
				session.ErrValidation,
				idx,
				trimmed,
				err,
			)
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	slices.Sort(normalized)
	return normalized, nil
}
