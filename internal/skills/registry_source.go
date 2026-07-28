package skills

import (
	"context"
	"errors"
	"fmt"

	"strings"
)

func checkRegistryContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("skills: context is required")
	}
	return ctx.Err()
}

// SkillSourceName returns the canonical string label for a skill source.
func SkillSourceName(source SkillSource) string {
	return skillSourceName(source)
}

// SkillPrecedenceTierName returns the canonical public resolver tier label.
func SkillPrecedenceTierName(source SkillSource) string {
	switch source {
	case SourceAgentLocal:
		return "agent_local"
	default:
		return skillSourceName(source)
	}
}

func skillSourceName(source SkillSource) string {
	switch source {
	case SourceBundled:
		return registryBundledKey
	case SourceMarketplace:
		return skillSourceMarketplaceName
	case SourceUser:
		return registryUserKey
	case SourceAdditional:
		return registryAdditionalKey
	case SourceWorkspace:
		return skillSourceWorkspaceName
	case SourceAgentLocal:
		return registryAgentLocalValue
	default:
		return "unknown"
	}
}

func skillSourceFromWorkspacePath(source string) (SkillSource, bool, error) {
	switch strings.TrimSpace(source) {
	case "", skillSourceWorkspaceName:
		return SourceWorkspace, true, nil
	case registryAdditionalKey:
		return SourceAdditional, true, nil
	case skillSourceMarketplaceName:
		return SourceMarketplace, false, nil
	case registryGlobalKey:
		return SourceUser, false, nil
	default:
		return 0, false, fmt.Errorf("skills: unsupported workspace skill source %q", source)
	}
}

func warningSeverityName(severity WarningSeverity) string {
	switch severity {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}
