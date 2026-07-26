package core

import (
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/session"

	"github.com/compozy/agh/internal/skills"
	skillmarketplace "github.com/compozy/agh/internal/skills/marketplace"
)

// SkillPayloadFromSkill converts a skills.Skill into the shared HTTP payload.
func SkillPayloadFromSkill(skill *skills.Skill) contract.SkillPayload {
	if skill == nil {
		return contract.SkillPayload{}
	}

	payload := contract.SkillPayload{
		Name:        skill.Meta.Name,
		Description: skill.Meta.Description,
		Version:     skill.Meta.Version,
		Source:      skills.SkillSourceName(skill.Source),
		Enabled:     skill.Enabled,
		Activation:  skillActivationPayload(skill),
		Dir:         skill.Dir,
		Metadata:    skill.Meta.Metadata,
		Diagnostics: SkillDiagnosticPayloadsFromDiagnostics(skills.DiagnosticsForSkill(skill)),
	}
	payload.Provenance = &contract.ProvenancePayload{
		InstalledFromBundle:    strings.TrimSpace(skill.InstalledFromBundle),
		InstalledFromExtension: strings.TrimSpace(skill.InstalledFromExtension),
		PrecedenceTier:         skills.SkillPrecedenceTierName(skill.Source),
		ShadowedBy:             SkillShadowEntryPayloadsFromRefs(skill.Diagnostics.ShadowedDefinitions),
	}
	if skill.Provenance != nil {
		payload.Provenance.Slug = skill.Provenance.Slug
		payload.Provenance.Registry = skill.Provenance.Registry
		payload.Provenance.Version = skill.Provenance.Version
		if !skill.Provenance.InstalledAt.IsZero() {
			installedAt := skill.Provenance.InstalledAt.UTC()
			payload.Provenance.InstalledAt = &installedAt
		}
	}

	return payload
}

// SkillShadowsResponseFromDomain converts one resolver shadow snapshot into the shared API payload.
func SkillShadowsResponseFromDomain(snapshot skills.SkillShadows) contract.SkillShadowsResponse {
	return contract.SkillShadowsResponse{
		Name:    strings.TrimSpace(snapshot.Name),
		Winner:  SkillShadowEntryPayloadFromDomain(snapshot.Winner),
		Shadows: SkillShadowEntryPayloadsFromDomain(snapshot.Shadows),
	}
}

func SkillShadowEntryPayloadsFromDomain(entries []skills.ShadowEntry) []contract.SkillShadowEntryPayload {
	if len(entries) == 0 {
		return nil
	}
	payloads := make([]contract.SkillShadowEntryPayload, 0, len(entries))
	for _, entry := range entries {
		payloads = append(payloads, SkillShadowEntryPayloadFromDomain(entry))
	}
	return payloads
}

func SkillShadowEntryPayloadFromDomain(entry skills.ShadowEntry) contract.SkillShadowEntryPayload {
	return contract.SkillShadowEntryPayload{
		Path:             strings.TrimSpace(entry.Path),
		Tier:             skillPrecedenceTierFromSourceLabel(entry.Tier),
		ResolvedToWinner: entry.ResolvedToWinner,
		DetectedAt:       skillShadowDetectedAt(entry.DetectedAt),
	}
}

func SkillShadowEntryPayloadsFromRefs(refs []skills.SkillDefinitionRef) []contract.SkillShadowEntryPayload {
	if len(refs) == 0 {
		return nil
	}
	payloads := make([]contract.SkillShadowEntryPayload, 0, len(refs))
	for _, ref := range refs {
		payloads = append(payloads, contract.SkillShadowEntryPayload{
			Path:             strings.TrimSpace(ref.Path),
			Tier:             skillPrecedenceTierFromSourceLabel(ref.Source),
			ResolvedToWinner: false,
			DetectedAt:       skillShadowDetectedAt(ref.DetectedAt),
		})
	}
	return payloads
}

func skillPrecedenceTierFromSourceLabel(source string) string {
	trimmed := strings.TrimSpace(source)
	if trimmed == "agent-local" {
		return "agent_local"
	}
	return trimmed
}

func skillShadowDetectedAt(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	return value.UTC()
}

// SkillPayloadsFromSkills converts a slice of skills into response payloads.
func SkillPayloadsFromSkills(skillList []*skills.Skill) []contract.SkillPayload {
	payload := make([]contract.SkillPayload, 0, len(skillList))
	for _, skill := range skillList {
		if skill == nil {
			continue
		}
		payload = append(payload, SkillPayloadFromSkill(skill))
	}
	return payload
}

// SkillMarketplaceInstallPayloadFromResult converts an install result into the shared payload.
func SkillMarketplaceInstallPayloadFromResult(
	result skillmarketplace.InstallResult,
) contract.SkillMarketplaceInstallPayload {
	return contract.SkillMarketplaceInstallPayload{
		Name:     result.Name,
		Slug:     result.Slug,
		Version:  result.Version,
		Registry: result.Registry,
		Path:     result.Path,
		Hash:     result.Hash,
		Status:   result.Status,
	}
}

// SkillMarketplaceUpdatePayloadsFromResults converts update results into shared payloads.
func SkillMarketplaceUpdatePayloadsFromResults(
	results []skillmarketplace.UpdateResult,
) []contract.SkillMarketplaceUpdatePayload {
	payload := make([]contract.SkillMarketplaceUpdatePayload, 0, len(results))
	for _, result := range results {
		payload = append(payload, contract.SkillMarketplaceUpdatePayload{
			Name:           result.Name,
			Slug:           result.Slug,
			CurrentVersion: result.CurrentVersion,
			LatestVersion:  result.LatestVersion,
			Path:           result.Path,
			Status:         result.Status,
		})
	}
	return payload
}

// SkillMarketplaceRemovePayloadFromResult converts a removal result into the shared payload.
func SkillMarketplaceRemovePayloadFromResult(
	result skillmarketplace.RemoveResult,
) contract.SkillMarketplaceRemovePayload {
	return contract.SkillMarketplaceRemovePayload{
		Name:   result.Name,
		Slug:   result.Slug,
		Path:   result.Path,
		Status: result.Status,
	}
}

func sessionWorkspaceFromInfo(info *session.Info) (string, string) {
	if info == nil {
		return "", ""
	}
	return strings.TrimSpace(info.WorkspaceID), strings.TrimSpace(info.Workspace)
}

func timePointerFromMap(values map[string]*time.Time, id string) *time.Time {
	if len(values) == 0 {
		return nil
	}
	value, ok := values[id]
	if !ok || value == nil {
		return nil
	}
	next := value.UTC()
	return &next
}
