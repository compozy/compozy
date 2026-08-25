import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  SettingsAttentionSection,
  SettingsAttentionFilter,
  SettingsAutomationSection,
  SettingsCreateNotificationPresetRequest,
  SettingsGeneralSection,
  SettingsHooksExtensionsSection,
  SettingsMemorySection,
  SettingsMutationResult,
  SettingsNetworkSection,
  SettingsNotificationPresetCollection,
  SettingsNotificationPresetEntry,
  SettingsNotificationPresetFilter,
  SettingsObservabilitySection,
  SettingsShellSection,
  SettingsSkillsFilter,
  SettingsSkillsSection,
  SettingsUpdateAttentionRequest,
  SettingsUpdateAttentionFilter,
  SettingsUpdateAutomationRequest,
  SettingsUpdateGeneralRequest,
  SettingsUpdateHooksExtensionsRequest,
  SettingsUpdateMemoryRequest,
  SettingsUpdateNetworkRequest,
  SettingsUpdateNotificationPresetRequest,
  SettingsUpdateObservabilityRequest,
  SettingsUpdateApplyRequest,
  SettingsUpdateApplyResult,
  SettingsUpdateCancelResult,
  SettingsUpdateTargetSet,
  SettingsUpdateShellRequest,
  SettingsUpdateSkillsFilter,
  SettingsUpdateSkillsRequest,
  SettingsUpdateStatus,
} from "../types";
import { normalizeOptionalText, SettingsApiError, settingsErrorDetail } from "./settings-api-error";
import { normalizeSettingsLayerFilter } from "./settings-layer-filter";

export { getSettingsCmdPalette, updateSettingsCmdPalette } from "./settings-cmd-palette-api";
export { getSettingsPersona, updateSettingsPersona } from "./settings-persona-api";

function normalizeNotificationPresetFilter(filter: SettingsNotificationPresetFilter = {}) {
  return {
    enabled: filter.enabled,
    built_in: filter.built_in,
    name: normalizeOptionalText(filter.name),
    limit: filter.limit,
  };
}

function normalizeSettingsSkillsFilter(
  filter: SettingsSkillsFilter | SettingsUpdateSkillsFilter = {}
) {
  return {
    scope: filter.scope,
    workspace_id: normalizeOptionalText(filter.workspace_id),
    profile: normalizeOptionalText(filter.profile),
    agent_name: normalizeOptionalText(filter.agent_name),
  };
}

export async function getSettingsGeneral(signal?: AbortSignal): Promise<SettingsGeneralSection> {
  const { data, error, response } = await apiClient.GET("/api/settings/general", { signal });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to load general settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load general settings");
}

export async function updateSettingsGeneral(
  body: SettingsUpdateGeneralRequest,
  signal?: AbortSignal
): Promise<SettingsMutationResult> {
  const { data, error, response } = await apiClient.PATCH("/api/settings/general", {
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to update general settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to update general settings");
}

export async function getSettingsUpdate(signal?: AbortSignal): Promise<SettingsUpdateStatus> {
  const { data, error, response } = await apiClient.GET("/api/settings/update", { signal });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to load update status", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load update status");
}

/**
 * Requests an apply for all eligible tracks. The daemon answers `accepted` + operation id
 * after durable acquisition, or a deterministic `blocked` naming the holder — it
 * never returns a terminal verdict it cannot yet know. Terminal truth arrives
 * through `getSettingsUpdate`, so callers must not treat a 200 as success.
 */
export async function applySettingsUpdate(
  body: SettingsUpdateApplyRequest,
  signal?: AbortSignal
): Promise<SettingsUpdateApplyResult> {
  const { data, error, response } = await apiClient.POST("/api/settings/update/apply", {
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to start the update", response, error),
      response.status
    );
  }
  const result = requireResponseData(data, response, "Failed to start the update");
  if (!isSettingsUpdateTargetSet(result.targets)) {
    throw new SettingsApiError("Failed to start the update: invalid target set", response.status);
  }
  return { ...result, targets: result.targets };
}

function isSettingsUpdateTargetSet(value: unknown): value is SettingsUpdateTargetSet {
  if (!Array.isArray(value)) return false;
  if (value.length === 1) return value[0] === "runtime" || value[0] === "app";
  return value.length === 2 && value[0] === "runtime" && value[1] === "app";
}

/** Cancels a dormant operation only; a live executor lease declines with its holder. */
export async function cancelSettingsUpdate(
  signal?: AbortSignal
): Promise<SettingsUpdateCancelResult> {
  const { data, error, response } = await apiClient.POST("/api/settings/update/cancel", { signal });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to cancel the update", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to cancel the update");
}

export async function getSettingsMemory(signal?: AbortSignal): Promise<SettingsMemorySection> {
  const { data, error, response } = await apiClient.GET("/api/settings/memory", { signal });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to load memory settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load memory settings");
}

export async function updateSettingsMemory(
  body: SettingsUpdateMemoryRequest,
  signal?: AbortSignal
): Promise<SettingsMutationResult> {
  const { data, error, response } = await apiClient.PATCH("/api/settings/memory", {
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to update memory settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to update memory settings");
}

export async function getSettingsSkills(
  filter: SettingsSkillsFilter = {},
  signal?: AbortSignal
): Promise<SettingsSkillsSection> {
  const { data, error, response } = await apiClient.GET("/api/settings/skills", {
    params: { query: normalizeSettingsSkillsFilter(filter) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to load skills settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load skills settings");
}

export async function updateSettingsSkills(
  body: SettingsUpdateSkillsRequest,
  filter: SettingsUpdateSkillsFilter = {},
  signal?: AbortSignal
): Promise<SettingsMutationResult> {
  const { data, error, response } = await apiClient.PATCH("/api/settings/skills", {
    body,
    params: { query: normalizeSettingsSkillsFilter(filter) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    // Source validation answers with a coded body; keep it so the section can
    // render the daemon's own sentence and code instead of a status number.
    const detail = settingsErrorDetail(error);
    throw new SettingsApiError(
      detail?.message ??
        defaultApiErrorMessage("Failed to update skills settings", response, error),
      response.status,
      detail
    );
  }
  return requireResponseData(data, response, "Failed to update skills settings");
}

export async function listSettingsNotificationPresets(
  filter: SettingsNotificationPresetFilter = {},
  signal?: AbortSignal
): Promise<SettingsNotificationPresetCollection> {
  const { data, error, response } = await apiClient.GET("/api/notifications/presets", {
    params: { query: normalizeNotificationPresetFilter(filter) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to load notification presets", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load notification presets");
}

export async function createSettingsNotificationPreset(
  body: SettingsCreateNotificationPresetRequest,
  signal?: AbortSignal
): Promise<SettingsNotificationPresetEntry> {
  const { data, error, response } = await apiClient.POST("/api/notifications/presets", {
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to create notification preset", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to create notification preset").preset;
}

export async function updateSettingsNotificationPreset(
  name: string,
  body: SettingsUpdateNotificationPresetRequest,
  signal?: AbortSignal
): Promise<SettingsNotificationPresetEntry> {
  const { data, error, response } = await apiClient.PUT("/api/notifications/presets/{name}", {
    params: { path: { name } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to update notification preset", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to update notification preset").preset;
}

export async function deleteSettingsNotificationPreset(
  name: string,
  signal?: AbortSignal
): Promise<void> {
  const { error, response } = await apiClient.DELETE("/api/notifications/presets/{name}", {
    params: { path: { name } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to delete notification preset", response, error),
      response.status
    );
  }
}

export async function getSettingsAutomation(
  signal?: AbortSignal
): Promise<SettingsAutomationSection> {
  const { data, error, response } = await apiClient.GET("/api/settings/automation", { signal });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to load automation settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load automation settings");
}

export async function updateSettingsAutomation(
  body: SettingsUpdateAutomationRequest,
  signal?: AbortSignal
): Promise<SettingsMutationResult> {
  const { data, error, response } = await apiClient.PATCH("/api/settings/automation", {
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to update automation settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to update automation settings");
}

export async function getSettingsNetwork(signal?: AbortSignal): Promise<SettingsNetworkSection> {
  const { data, error, response } = await apiClient.GET("/api/settings/network", { signal });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to load network settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load network settings");
}

export async function updateSettingsNetwork(
  body: SettingsUpdateNetworkRequest,
  signal?: AbortSignal
): Promise<SettingsMutationResult> {
  const { data, error, response } = await apiClient.PATCH("/api/settings/network", {
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to update network settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to update network settings");
}

export async function getSettingsAttention(
  filter: SettingsAttentionFilter,
  signal?: AbortSignal
): Promise<SettingsAttentionSection> {
  const { data, error, response } = await apiClient.GET("/api/settings/attention", {
    params: { query: normalizeSettingsLayerFilter(filter) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to load attention settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load attention settings");
}

/** Typed PATCH: global delivery fields are required; profile-owned mutes are replaced when present. */
export async function updateSettingsAttention(
  body: SettingsUpdateAttentionRequest,
  filter: SettingsUpdateAttentionFilter,
  signal?: AbortSignal
): Promise<SettingsMutationResult> {
  const { data, error, response } = await apiClient.PATCH("/api/settings/attention", {
    body,
    params: { query: normalizeSettingsLayerFilter(filter) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to update attention settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to update attention settings");
}

export async function getSettingsShell(signal?: AbortSignal): Promise<SettingsShellSection> {
  const { data, error, response } = await apiClient.GET("/api/settings/shell", { signal });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to load shell settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load shell settings");
}

export async function updateSettingsShell(
  body: SettingsUpdateShellRequest,
  signal?: AbortSignal
): Promise<SettingsMutationResult> {
  const { data, error, response } = await apiClient.PATCH("/api/settings/shell", {
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to update shell settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to update shell settings");
}

export async function getSettingsObservability(
  signal?: AbortSignal
): Promise<SettingsObservabilitySection> {
  const { data, error, response } = await apiClient.GET("/api/settings/observability", { signal });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to load observability settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load observability settings");
}

export async function updateSettingsObservability(
  body: SettingsUpdateObservabilityRequest,
  signal?: AbortSignal
): Promise<SettingsMutationResult> {
  const { data, error, response } = await apiClient.PATCH("/api/settings/observability", {
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to update observability settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to update observability settings");
}

export async function getSettingsHooksExtensions(
  signal?: AbortSignal
): Promise<SettingsHooksExtensionsSection> {
  const { data, error, response } = await apiClient.GET("/api/settings/hooks-extensions", {
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to load hooks and extensions settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load hooks and extensions settings");
}

export async function updateSettingsHooksExtensions(
  body: SettingsUpdateHooksExtensionsRequest,
  signal?: AbortSignal
): Promise<SettingsMutationResult> {
  const { data, error, response } = await apiClient.PATCH("/api/settings/hooks-extensions", {
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to update hooks and extensions settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to update hooks and extensions settings");
}
