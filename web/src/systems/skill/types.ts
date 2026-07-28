import type { OperationRequestBody, OperationResponse } from "@/lib/api-contract";

export type SkillsResponse = OperationResponse<"listSkills", 200>;
export type SkillPayload = SkillsResponse["skills"][number];
export type SkillActivationPayload = SkillPayload["activation"];
export type SkillActivationReasonPayload = NonNullable<SkillActivationPayload["reasons"]>[number];
export type SkillResponse = OperationResponse<"getSkill", 200>;
export type SkillContentResponse = OperationResponse<"getSkillContent", 200>;
export type SkillShadowsResponse = OperationResponse<"getSkillShadows", 200>;
export type SkillShadowEntryPayload = SkillShadowsResponse["shadows"][number];
export type SkillActionResponse = OperationResponse<"enableSkill", 200>;
export type ProvenancePayload = NonNullable<SkillPayload["provenance"]>;

export type SkillMarketplaceInstallResponse = OperationResponse<"installSkillMarketplace", 200>;
export type SkillMarketplaceInstallPayload = SkillMarketplaceInstallResponse["skill"];
export type SkillMarketplaceInstallRequest = OperationRequestBody<"installSkillMarketplace">;

export type SkillMarketplaceUpdateResponse = OperationResponse<"updateSkillMarketplace", 200>;
export type SkillMarketplaceUpdatePayload = SkillMarketplaceUpdateResponse["skills"][number];
export type SkillMarketplaceUpdateRequest = OperationRequestBody<"updateSkillMarketplace">;

export type SkillMarketplaceRemoveResponse = OperationResponse<"removeSkillMarketplace", 200>;
export type SkillMarketplaceRemovePayload = SkillMarketplaceRemoveResponse["skill"];
