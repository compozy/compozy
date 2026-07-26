package daemon

import (
	"encoding/json"
	"errors"

	"strings"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/network"

	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := strings.TrimSpace(*value)
	return &cloned
}

func cloneTrimmedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			cloned = append(cloned, trimmed)
		}
	}
	return cloned
}

func cloneIntPtr(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneBoolPtr(value *bool) *bool {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneRawMessagePtr(value *json.RawMessage) *json.RawMessage {
	if value == nil {
		return nil
	}
	cloned := cloneJSON(*value)
	return &cloned
}

func taskPriorityPtr(value *string) *taskpkg.Priority {
	if value == nil {
		return nil
	}
	priority := taskpkg.Priority(strings.TrimSpace(*value))
	return &priority
}

func taskApprovalPolicyPtr(value *string) *taskpkg.ApprovalPolicy {
	if value == nil {
		return nil
	}
	policy := taskpkg.ApprovalPolicy(strings.TrimSpace(*value))
	return &policy
}

func cloneTaskOwner(owner *taskpkg.Ownership) *taskpkg.Ownership {
	if owner == nil {
		return nil
	}
	cloned := *owner
	cloned.Kind = taskpkg.OwnerKind(strings.TrimSpace(string(cloned.Kind)))
	cloned.Ref = strings.TrimSpace(cloned.Ref)
	return &cloned
}

func cloneJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func cloneExtensionMap(src network.ExtensionMap) network.ExtensionMap {
	if len(src) == 0 {
		return nil
	}
	dst := make(network.ExtensionMap, len(src))
	for key, value := range src {
		dst[key] = cloneJSON(value)
	}
	return dst
}

func nativeToolPolicyInputs(cfg *aghconfig.Config) (toolspkg.PolicyInputs, error) {
	if cfg == nil {
		return toolspkg.PolicyInputs{}, errors.New("daemon: native tool config is required")
	}
	trustedSources := make([]toolspkg.SourceGrant, 0, len(cfg.Tools.Policy.TrustedSources))
	for _, raw := range cfg.Tools.Policy.TrustedSources {
		grant, err := toolspkg.ParseSourceGrant(raw)
		if err != nil {
			return toolspkg.PolicyInputs{}, err
		}
		trustedSources = append(trustedSources, grant)
	}
	return toolspkg.PolicyInputs{
		ToolsDisabled:        !cfg.Tools.Enabled,
		SystemPermissionMode: nativeToolPermissionMode(cfg.Permissions.Mode),
		ExternalDefault:      nativeToolExternalDefault(cfg.Tools.Policy.ExternalDefault),
		TrustedSources:       trustedSources,
	}, nil
}

func nativeToolPermissionMode(mode aghconfig.PermissionMode) toolspkg.PermissionMode {
	switch mode {
	case aghconfig.PermissionModeDenyAll:
		return toolspkg.PermissionModeDenyAll
	case aghconfig.PermissionModeApproveReads:
		return toolspkg.PermissionModeApproveReads
	case aghconfig.PermissionModeApproveAll:
		return toolspkg.PermissionModeApproveAll
	default:
		return ""
	}
}

func nativeToolExternalDefault(value aghconfig.ToolsExternalDefault) toolspkg.ExternalDefault {
	switch value {
	case aghconfig.ToolsExternalDefaultAsk:
		return toolspkg.ExternalDefaultAsk
	case aghconfig.ToolsExternalDefaultEnabled:
		return toolspkg.ExternalDefaultEnabled
	default:
		return toolspkg.ExternalDefaultDisabled
	}
}
