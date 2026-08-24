import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  ArchiveProfilePlan,
  ArchiveProfileResult,
  CreateProfileParams,
  DeleteProfilePlan,
  DeleteProfileResult,
  ProfileLens,
  ProfileOperation,
  ProfilePayload,
  ProfileSelectionParams,
  ProfileSelectionPayload,
  ProfileSelectionResult,
  RenameProfilePlan,
  RenameProfileParams,
  RenameProfileResult,
  UnarchiveProfileResult,
  UpdateProfileParams,
} from "../types";

/**
 * Carries the daemon's stable machine code alongside the message.
 *
 * Every profile refusal is typed (`profile_plan_stale`, `profile_owns_work`,
 * `profile_remote_management_forbidden`, …), and the dialogs branch on the code
 * rather than on prose, so the message stays free to change.
 */
export class ProfileApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly code: string,
    public readonly action: string
  ) {
    super(message);
    this.name = "ProfileApiError";
  }
}

interface ProfileErrorEnvelope {
  error?: { code?: string; message?: string; action?: string };
}

function profileError(fallback: string, response: Response, error: unknown): ProfileApiError {
  const envelope = (error ?? {}) as ProfileErrorEnvelope;
  const message = envelope.error?.message?.trim();
  return new ProfileApiError(
    message && message !== "" ? message : defaultApiErrorMessage(fallback, response, error),
    response.status,
    envelope.error?.code?.trim() ?? "",
    envelope.error?.action?.trim() ?? ""
  );
}

export async function fetchProfiles(signal?: AbortSignal): Promise<ProfilePayload[]> {
  const { data, error, response } = await apiClient.GET("/api/profiles", { signal });
  if (apiRequestFailed(response, error)) {
    throw profileError("Failed to load profiles", response, error);
  }
  return requireResponseData(data, response, "Failed to load profiles");
}

export async function fetchProfile(name: string, signal?: AbortSignal): Promise<ProfilePayload> {
  const { data, error, response } = await apiClient.GET("/api/profiles/{name}", {
    params: { path: { name } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw profileError(`Failed to load profile ${name}`, response, error);
  }
  return requireResponseData(data, response, `Failed to load profile ${name}`);
}

export async function createProfile(
  params: CreateProfileParams,
  signal?: AbortSignal
): Promise<ProfilePayload> {
  const { data, error, response } = await apiClient.POST("/api/profiles", {
    body: params,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw profileError("Failed to create the profile", response, error);
  }
  return requireResponseData(data, response, "Failed to create the profile");
}

export async function updateProfileIdentity(
  name: string,
  params: UpdateProfileParams,
  signal?: AbortSignal
): Promise<ProfilePayload> {
  const { data, error, response } = await apiClient.PATCH("/api/profiles/{name}", {
    params: { path: { name } },
    body: params,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw profileError(`Failed to update ${name}`, response, error);
  }
  return requireResponseData(data, response, `Failed to update ${name}`);
}

export async function fetchRenamePlan(
  name: string,
  newName: string,
  signal?: AbortSignal
): Promise<RenameProfilePlan> {
  const { data, error, response } = await apiClient.GET("/api/profiles/{name}/rename-plan", {
    params: { path: { name }, query: { new_name: newName } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw profileError(`Failed to plan the rename of ${name}`, response, error);
  }
  return requireResponseData(data, response, `Failed to plan the rename of ${name}`);
}

export async function renameProfile(
  name: string,
  body: RenameProfileParams,
  signal?: AbortSignal
): Promise<RenameProfileResult> {
  const { data, error, response } = await apiClient.POST("/api/profiles/{name}/rename", {
    params: { path: { name } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw profileError(`Failed to rename ${name}`, response, error);
  }
  return requireResponseData(data, response, `Failed to rename ${name}`);
}

export async function fetchArchivePlan(
  name: string,
  signal?: AbortSignal
): Promise<ArchiveProfilePlan> {
  const { data, error, response } = await apiClient.GET("/api/profiles/{name}/archive-plan", {
    params: { path: { name } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw profileError(`Failed to plan the archive of ${name}`, response, error);
  }
  return requireResponseData(data, response, `Failed to plan the archive of ${name}`);
}

export async function archiveProfile(
  name: string,
  planRevision: string,
  signal?: AbortSignal
): Promise<ArchiveProfileResult> {
  const { data, error, response } = await apiClient.POST("/api/profiles/{name}/archive", {
    params: { path: { name } },
    body: { plan_revision: planRevision },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw profileError(`Failed to archive ${name}`, response, error);
  }
  return requireResponseData(data, response, `Failed to archive ${name}`);
}

export async function unarchiveProfile(
  name: string,
  signal?: AbortSignal
): Promise<UnarchiveProfileResult> {
  const { data, error, response } = await apiClient.POST("/api/profiles/{name}/unarchive", {
    params: { path: { name } },
    body: {},
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw profileError(`Failed to unarchive ${name}`, response, error);
  }
  return requireResponseData(data, response, `Failed to unarchive ${name}`);
}

export async function fetchDeletePlan(
  name: string,
  signal?: AbortSignal
): Promise<DeleteProfilePlan> {
  const { data, error, response } = await apiClient.GET("/api/profiles/{name}/delete-plan", {
    params: { path: { name } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw profileError(`Failed to plan the deletion of ${name}`, response, error);
  }
  return requireResponseData(data, response, `Failed to plan the deletion of ${name}`);
}

export async function deleteProfile(
  name: string,
  planRevision: string,
  signal?: AbortSignal
): Promise<DeleteProfileResult> {
  const { data, error, response } = await apiClient.DELETE("/api/profiles/{name}", {
    params: { path: { name }, query: { plan_revision: planRevision } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw profileError(`Failed to delete ${name}`, response, error);
  }
  return requireResponseData(data, response, `Failed to delete ${name}`);
}

/** The full project → profile map that Settings inspects. */
export async function fetchProfileSelections(
  signal?: AbortSignal
): Promise<ProfileSelectionPayload[]> {
  const { data, error, response } = await apiClient.GET("/api/profiles/selection", { signal });
  if (apiRequestFailed(response, error)) {
    throw profileError("Failed to load the profile selection map", response, error);
  }
  const payload = requireResponseData(data, response, "Failed to load the profile selection map");
  return Array.isArray(payload) ? payload : [payload];
}

/** The remembered choice for one lens. The daemon answers `default` when none is stored. */
export async function fetchProfileSelection(
  lens: ProfileLens,
  signal?: AbortSignal
): Promise<ProfileSelectionPayload> {
  const selections = await fetchProfileSelections(signal);
  const match = selections.find(
    row => row.scope === lens.scope && (row.workspace_id ?? "") === (lens.workspaceId ?? "")
  );
  return match ?? { scope: lens.scope, profile: "default" };
}

export async function putProfileSelection(
  params: ProfileSelectionParams,
  signal?: AbortSignal
): Promise<ProfileSelectionResult> {
  const { data, error, response } = await apiClient.PUT("/api/profiles/selection", {
    body: params,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw profileError("Failed to remember the profile", response, error);
  }
  return requireResponseData(data, response, "Failed to remember the profile");
}

export async function fetchProfileOperations(signal?: AbortSignal): Promise<ProfileOperation[]> {
  const { data, error, response } = await apiClient.GET("/api/profiles/ops", { signal });
  if (apiRequestFailed(response, error)) {
    throw profileError("Failed to load profile operations", response, error);
  }
  return requireResponseData(data, response, "Failed to load profile operations");
}
