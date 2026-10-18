package daemon

import (
	"fmt"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type SessionRuntimeMode string

const (
	SessionRuntimeModeEvidence SessionRuntimeMode = "evidence"
)

type SessionPolicy struct {
	Runtime SessionRuntimePolicy
}

type SessionRuntimePolicy struct {
	Mode        SessionRuntimeMode
	Permissions compozyconfig.PermissionMode
}

func sessionPolicyFromTaskExecutionProfile(profile *taskpkg.ExecutionProfile) SessionPolicy {
	if profile == nil {
		return SessionPolicy{}
	}
	return SessionPolicy{

		Runtime: SessionRuntimePolicy{
			Mode: SessionRuntimeMode(profile.Runtime.Mode.Normalize()),
		},
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

func normalizeSessionRuntimeMode(mode SessionRuntimeMode) SessionRuntimeMode {
	return SessionRuntimeMode(strings.ToLower(strings.TrimSpace(string(mode))))
}

func normalizeSessionPermissionMode(mode compozyconfig.PermissionMode) compozyconfig.PermissionMode {
	return compozyconfig.PermissionMode(strings.ToLower(strings.TrimSpace(string(mode))))
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
