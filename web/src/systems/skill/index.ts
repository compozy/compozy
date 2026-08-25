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

export type {
  SkillExposeRequest,
  SkillExposeResponse,
  SkillExposeResult,
  SkillExposurePayload,
  SkillExposureStatus,
  SkillUnexposeResponse,
} from "./types";

// Adapters
export {
  disableSkill,
  enableSkill,
  exposeSkill,
  getSkill,
  SkillExposeError,
  unexposeSkill,
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
  skillOriginLabel,
  skillSourceTone,
  type SkillRecentCall,
} from "./lib/skill-formatters";
export {
  isSkillExposable,
  skillExposeResultViews,
  skillExposureViews,
  toSkillExposureView,
  type SkillExposeResultView,
  type SkillExposureView,
} from "./lib/skill-exposure-view";
export {
  SkillActivationPill,
  SkillActivationReasons,
  SkillActivationSection,
} from "./components/skill-activation-status";
export { SkillExposePanel } from "./components/skill-expose-panel";
export {
  SkillExposeTargetPicker,
  type SkillExposeTarget,
} from "./components/skill-expose-target-picker";

// Hooks
export { useSkill, useSkillContent, useSkillShadows, useSkills } from "./hooks/use-skills";
export { useSkillExpose, type SkillExposeModel } from "./hooks/use-skill-expose";
export { useSkillExposures, type SkillExposuresModel } from "./hooks/use-skill-exposures";
export {
  useDisableSkill,
  useEnableSkill,
  useInstallSkillMarketplace,
  useRemoveSkillMarketplace,
  useUpdateSkillMarketplace,
} from "./hooks/use-skill-actions";
