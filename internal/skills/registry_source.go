package skills

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/resources"
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

// SkillPrecedenceRank returns an explicit low-to-high resolver rank.
func SkillPrecedenceRank(source SkillSource) int {
	switch source {
	case SourceBundled:
		return 0
	case SourceMarketplace:
		return 10
	case SourceUser:
		return 20
	case SourceProfile:
		return 30
	case SourceAdditional:
		return 40
	case SourceWorkspace:
		return 50
	case SourceWorkspaceProfile:
		return 60
	case SourceAgentLocal:
		return 70
	default:
		return -1
	}
}

func sourceTierFor(spec compozyconfig.SkillRootSpec) SkillSource {
	if spec.Kind == compozyconfig.RootKindCustom {
		return SourceAdditional
	}
	switch spec.ResourceScope.Kind.Normalize() {
	case resources.ResourceScopeKindUser:
		return SourceUser
	case resources.ResourceScopeKindProfile:
		return SourceProfile
	case resources.ResourceScopeKindWorkspace:
		return SourceWorkspace
	case resources.ResourceScopeKindWorkspaceProfile:
		return SourceWorkspaceProfile
	default:
		return SourceAdditional
	}
}

func assignSkillRoot(skill *Skill, spec compozyconfig.SkillRootSpec) {
	if skill == nil {
		return
	}
	skill.Source = sourceTierFor(spec)
	skill.RootID = spec.RootID()
	skill.RootDir = strings.TrimSpace(spec.Dir)
	skill.ResourceScope = spec.ResourceScope.Normalize()
	if strings.TrimSpace(spec.SourceSlug) != compozyconfig.SkillSourceCompozy {
		skill.Origin = strings.TrimSpace(spec.SourceSlug)
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
	case SourceProfile:
		return skillSourceProfileName
	case SourceWorkspaceProfile:
		return skillSourceWorkspaceProfileName
	default:
		return skillSourceUnknownName
	}
}

func skillSourceFromWorkspacePath(source string) (SkillSource, bool, error) {
	switch strings.TrimSpace(source) {
	case "", skillSourceWorkspaceName:
		return SourceWorkspace, true, nil
	case skillSourceWorkspaceProfileName:
		return SourceWorkspaceProfile, true, nil
	case skillSourceProfileName:
		return SourceProfile, true, nil
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
