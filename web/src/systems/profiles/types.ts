import type { OperationRequestBody, OperationResponse } from "@/lib/api-contract";

/** A profile row as the daemon returns it. */
export type ProfilePayload = OperationResponse<"listProfiles", 200>[number];

export type CreateProfileParams = OperationRequestBody<"createProfile">;
export type UpdateProfileParams = OperationRequestBody<"updateProfile">;

export type ProfileSelectionPayload = OperationResponse<"putProfileSelection", 200>;
export type ProfileSelectionParams = OperationRequestBody<"putProfileSelection">;

export type RenameProfilePlan = OperationResponse<"prepareProfileRename", 200>;
export type ArchiveProfilePlan = OperationResponse<"prepareProfileArchive", 200>;
export type DeleteProfilePlan = OperationResponse<"prepareProfileDelete", 200>;

export type RenameProfileResult = OperationResponse<"renameProfile", 200>;
export type ArchiveProfileResult = OperationResponse<"archiveProfile", 200>;
export type UnarchiveProfileResult = OperationResponse<"unarchiveProfile", 200>;
export type DeleteProfileResult = OperationResponse<"deleteProfile", 200>;

export type ProfileRemovalSummary = DeleteProfilePlan["removed"];
export type ProfileRepoCandidate = RenameProfilePlan["repo_candidates"][number];
export type ProfilePlacement = RenameProfilePlan["dormant_placements"][number];

export type ProfileOperation = OperationResponse<"listProfileOperations", 200>[number];

/**
 * The lens a selection is remembered against. A workspace lens carries its id;
 * the Global lens is the across-projects slot and carries none (ADR-003).
 */
export type ProfileLens =
  | { scope: "workspace"; workspaceId: string }
  | { scope: "global"; workspaceId?: undefined };

/** The reserved aggregate state. Never persisted as a remembered choice (ADR-003). */
export const PROFILE_AGGREGATE = "@all" as const;

/**
 * What a client is currently looking through: one real profile, or the labeled
 * aggregate. This is per-client view state, not the remembered choice.
 */
export type ProfileView = { kind: "profile"; profile: string } | { kind: "aggregate" };

export type ProfileLifecycleFlow =
  | "create"
  | "update"
  | "rename"
  | "archive"
  | "unarchive"
  | "delete";

/** What the palette or Settings hands to the canonical dialog host. */
export interface ProfileDialogIntent {
  flow: ProfileLifecycleFlow;
  /** Absent for `create`. */
  profile?: string;
}
