package settings

import (
	"context"
	"errors"
	"fmt"

	aghconfig "github.com/compozy/agh/internal/config"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

const (
	sectionsTimeoutKey = "timeout"
)

const (
	sectionsConsolidateKey            = "consolidate"
	sectionsControllerKey             = "controller"
	sectionsDailyKey                  = "daily"
	sectionsDecisionsKey              = "decisions"
	sectionsDefaultsKey               = "defaults"
	sectionsDreamKey                  = "dream"
	sectionsEnabledKey                = "enabled"
	sectionsExtensionsKey             = "extensions"
	sectionsExtractorKey              = "extractor"
	sectionsGatesKey                  = "gates"
	sectionsHTTPKey                   = "http"
	sectionsMarketplaceKey            = "marketplace"
	sectionsMaxKey                    = "max"
	sectionsModelKey                  = "model"
	sectionsModeKey                   = "mode"
	sectionsNoChangesValue            = "no changes"
	sectionsOperatorWriteRateLimitKey = "operator_write_rate_limit"
	sectionsPolicyKey                 = "policy"
	sectionsProviderKey               = "provider"
	sectionsReasoningEffortKey        = "reasoning_effort"
	sectionsFallbackChainKey          = "fallback_chain"
	sectionsQueueKey                  = "queue"
	sectionsRecallKey                 = "recall"
	sectionsResourcesKey              = "resources"
	sectionsRestartKey                = "restart"
	sectionsScoringKey                = "scoring"
	sectionsSessionKey                = "session"
	sectionsSignalsKey                = "signals"
	sectionsSnapshotRateLimitKey      = "snapshot_rate_limit"
	sectionsTranscriptsKey            = "transcripts"
	sectionsWeightsKey                = "weights"
	sectionsWindowKey                 = "window"
)

func (s *service) GetSection(ctx context.Context, req SectionRequest) (SectionEnvelope, error) {
	scope, workspaceID, agentName, err := s.resolveSectionScope(req.Section, req.Scope, req.WorkspaceID, req.AgentName)
	if err != nil {
		return SectionEnvelope{}, fmt.Errorf("settings: get section %q: %w", req.Section, err)
	}

	cfg, resolved, err := s.loadConfig(ctx, scope, workspaceID)
	if err != nil {
		return SectionEnvelope{}, fmt.Errorf("settings: load section %q config: %w", req.Section, err)
	}

	envelope := newSectionEnvelope(req.Section, scope, workspaceID, agentName)
	if err := s.populateSectionEnvelope(ctx, &envelope, &cfg, resolved); err != nil {
		return SectionEnvelope{}, err
	}

	return envelope, nil
}

func (s *service) UpdateSection(ctx context.Context, req SectionUpdateRequest) (MutationResult, error) {
	if req.Section == SectionSkills {
		if req.Skills == nil {
			return MutationResult{}, validationError(errors.New("settings: skills section payload is required"))
		}
		result, err := s.updateSkillsSection(ctx, req.SectionRequest, *req.Skills)
		return s.finalizeSectionUpdate(ctx, result, err)
	}

	result, err := s.updateConfigBackedSection(ctx, req)
	return s.finalizeSectionUpdate(ctx, result, err)
}

func (s *service) resolveSectionScope(
	section SectionName,
	scope ScopeKind,
	workspaceID string,
	agentName string,
) (ScopeKind, string, string, error) {
	normalizedScope, normalizedWorkspaceID, err := s.normalizeReadScope(scope, workspaceID)
	if err != nil {
		return "", "", "", err
	}
	normalizedAgentName, err := normalizeAgentName(agentName)
	if err != nil {
		return "", "", "", err
	}
	if err := validateSectionScope(section, normalizedScope, normalizedAgentName); err != nil {
		return "", "", "", err
	}
	return normalizedScope, normalizedWorkspaceID, normalizedAgentName, nil
}

func validateSectionScope(section SectionName, scope ScopeKind, agentName string) error {
	if section == SectionRoles {
		if scope == ScopeAgent {
			return conflictError(
				fmt.Errorf("settings: section %q does not support %s scope", section, scope),
			)
		}
		return nil
	}
	if section != SectionSkills && scope != ScopeGlobal {
		return conflictError(
			fmt.Errorf("settings: section %q does not support %s scope", section, scope),
		)
	}
	if section != SectionSkills {
		return nil
	}
	if scope == ScopeWorkspace {
		return conflictError(
			fmt.Errorf("settings: section %q does not support %s scope", section, scope),
		)
	}
	if scope == ScopeAgent && agentName == "" {
		return validationError(errors.New("settings: agent scope requires agent_name"))
	}
	return nil
}

func newSectionEnvelope(
	section SectionName,
	scope ScopeKind,
	workspaceID string,
	agentName string,
) SectionEnvelope {
	return SectionEnvelope{
		Section:         section,
		Scope:           scope,
		WorkspaceID:     workspaceID,
		AgentName:       agentName,
		AvailableScopes: []ScopeKind{ScopeGlobal},
	}
}

func (s *service) populateSectionEnvelope(
	ctx context.Context,
	envelope *SectionEnvelope,
	cfg *aghconfig.Config,
	resolved *workspacepkg.ResolvedWorkspace,
) error {
	switch envelope.Section {
	case SectionGeneral:
		envelope.Scope = ScopeGlobal
		section, err := s.buildGeneralSection(ctx, cfg)
		if err != nil {
			return err
		}
		envelope.General = &section
	case SectionMemory:
		envelope.Scope = ScopeGlobal
		section, err := s.buildMemorySection(ctx, cfg)
		if err != nil {
			return err
		}
		envelope.Memory = &section
	case SectionRoles:
		envelope.AvailableScopes = []ScopeKind{ScopeGlobal, ScopeWorkspace}
		section := RolesSection{Config: aghconfig.CloneRolesConfig(&cfg.Roles)}
		envelope.Roles = &section
	case SectionSkills:
		envelope.AvailableScopes = []ScopeKind{ScopeGlobal, ScopeAgent}
		section, err := s.buildSkillsSection(
			ctx,
			cfg,
			resolved,
			envelope.Scope,
			envelope.AgentName,
		)
		if err != nil {
			return err
		}
		envelope.Skills = &section
	case SectionAutomation:
		envelope.Scope = ScopeGlobal
		section, err := s.buildAutomationSection(ctx, cfg)
		if err != nil {
			return err
		}
		envelope.Automation = &section
	case SectionNetwork:
		envelope.Scope = ScopeGlobal
		section, err := s.buildNetworkSection(ctx, cfg)
		if err != nil {
			return err
		}
		envelope.Network = &section
	case SectionWindowManager:
		envelope.Scope = ScopeGlobal
		section := buildWindowManagerSection(cfg)
		envelope.WindowManager = &section
	case SectionObservability:
		envelope.Scope = ScopeGlobal
		section, err := s.buildObservabilitySection(ctx, cfg)
		if err != nil {
			return err
		}
		envelope.Observability = &section
	case SectionHooksExtensions:
		envelope.Scope = ScopeGlobal
		section, err := s.buildHooksExtensionsSection(ctx, cfg)
		if err != nil {
			return err
		}
		envelope.HooksExtensions = &section
	default:
		return notFoundError(fmt.Errorf("settings: unknown section %q", envelope.Section))
	}
	return nil
}

func (s *service) finalizeSectionUpdate(
	ctx context.Context,
	result MutationResult,
	err error,
) (MutationResult, error) {
	if err != nil {
		return MutationResult{}, err
	}
	if result.Scope == ScopeWorkspace {
		if invalidator, ok := s.workspaceResolver.(interface{ Invalidate(string) }); ok {
			invalidator.Invalidate(result.WorkspaceID)
		}
	}
	if emitErr := s.emitSettingsChanged(ctx, result, "patch"); emitErr != nil {
		return MutationResult{}, emitErr
	}
	return result, nil
}
