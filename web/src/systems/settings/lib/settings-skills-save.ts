import type { SettingsSkillsSection } from "../types";

type SkillsConfig = SettingsSkillsSection["config"];

/**
 * Each save carries only the fields its own section owns, laid over the server's
 * current config. Sending the whole draft would let an unsaved edit in one
 * section ride along with an unrelated Apply.
 */

export function skillsDisabledConfig(baseline: SkillsConfig, draft: SkillsConfig): SkillsConfig {
  return { ...baseline, disabled_skills: draft.disabled_skills ?? [] };
}

export function skillsPolicyConfig(baseline: SkillsConfig, draft: SkillsConfig): SkillsConfig {
  return {
    ...baseline,
    enabled: draft.enabled,
    poll_interval: draft.poll_interval,
    marketplace: draft.marketplace,
    allowed_marketplace_mcp: draft.allowed_marketplace_mcp,
    allowed_marketplace_hooks: draft.allowed_marketplace_hooks,
  };
}

export function skillsSourcesConfig(baseline: SkillsConfig, draft: SkillsConfig): SkillsConfig {
  return { ...baseline, sources: draft.sources, custom_sources: draft.custom_sources };
}

export function sameStringList(left: readonly string[], right: readonly string[]): boolean {
  if (left.length !== right.length) return false;
  return left.every((value, index) => value === right[index]);
}

export function sameSourcesConfig(left: SkillsConfig, right: SkillsConfig): boolean {
  return (
    sameStringList(left.sources, right.sources) &&
    sameStringList(left.custom_sources, right.custom_sources)
  );
}
