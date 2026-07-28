// Types
export type {
  ProvenancePayload,
  SkillActionResponse,
  SkillContentResponse,
  SkillMarketplaceInstallPayload,
  SkillMarketplaceInstallRequest,
  SkillMarketplaceInstallResponse,
  SkillMarketplaceRemovePayload,
  SkillMarketplaceRemoveResponse,
  SkillMarketplaceUpdatePayload,
  SkillMarketplaceUpdateRequest,
  SkillMarketplaceUpdateResponse,
  SkillPayload,
  SkillShadowEntryPayload,
  SkillShadowsResponse,
  SkillResponse,
  SkillsResponse,
} from "./types";

// Adapters
export {
  disableSkill,
  enableSkill,
  getSkill,
  getSkillContent,
  getSkillShadows,
  installSkillMarketplace,
  listSkills,
  removeSkillMarketplace,
  SkillApiError,
  updateSkillMarketplace,
} from "./adapters/skill-api";

// Query infrastructure
export { skillKeys } from "./lib/query-keys";
export {
  skillContentOptions,
  skillDetailOptions,
  skillShadowsOptions,
  skillsListOptions,
} from "./lib/query-options";
export {
  deriveSkillCapabilities,
  deriveSkillRecentCalls,
  skillSourceTone,
  type SkillRecentCall,
} from "./lib/skill-formatters";
export {
  SkillActivationPill,
  SkillActivationReasons,
  SkillActivationSection,
} from "./components/skill-activation-status";

// Hooks
export { useSkill, useSkillContent, useSkillShadows, useSkills } from "./hooks/use-skills";
export {
  useDisableSkill,
  useEnableSkill,
  useInstallSkillMarketplace,
  useRemoveSkillMarketplace,
  useUpdateSkillMarketplace,
} from "./hooks/use-skill-actions";
