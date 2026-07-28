import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  SettingsHookCollection,
  SettingsHookRequest,
  SettingsMCPServerCollection,
  SettingsMCPServerDeleteFilter,
  SettingsMCPServerListFilter,
  SettingsMCPServerPutFilter,
  SettingsMCPServerRequest,
  SettingsMutationResult,
  SettingsProviderCollection,
  SettingsProviderDetail,
  SettingsProviderRequest,
  SettingsSandboxCollection,
  SettingsSandboxDetail,
  SettingsSandboxRequest,
} from "../types";
import { normalizeOptionalText, SettingsApiError } from "./settings-api-error";

function normalizeMCPListFilter(filter: SettingsMCPServerListFilter = {}) {
  return {
    scope: filter.scope,
    workspace_id: normalizeOptionalText(filter.workspace_id),
  };
}

function normalizeMCPMutationFilter(
  filter: SettingsMCPServerPutFilter | SettingsMCPServerDeleteFilter = {}
) {
  return {
    scope: filter.scope,
    workspace_id: normalizeOptionalText(filter.workspace_id),
    target: filter.target,
  };
}

export async function listSettingsProviders(
  signal?: AbortSignal
): Promise<SettingsProviderCollection> {
  const { data, error, response } = await apiClient.GET("/api/settings/providers", { signal });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to list settings providers", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to list settings providers");
}

export async function getSettingsProvider(
  name: string,
  signal?: AbortSignal
): Promise<SettingsProviderDetail> {
  const { data, error, response } = await apiClient.GET("/api/settings/providers/{name}", {
    params: { path: { name } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new SettingsApiError(`Provider not found: ${name}`, 404);
    throw new SettingsApiError(
      defaultApiErrorMessage(`Failed to load provider "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to load provider "${name}"`).provider;
}

export async function putSettingsProvider(
  name: string,
  body: SettingsProviderRequest,
  signal?: AbortSignal
): Promise<SettingsMutationResult> {
  const { data, error, response } = await apiClient.PUT("/api/settings/providers/{name}", {
    params: { path: { name } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage(`Failed to save provider "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to save provider "${name}"`);
}

export async function deleteSettingsProvider(
  name: string,
  signal?: AbortSignal
): Promise<SettingsMutationResult> {
  const { data, error, response } = await apiClient.DELETE("/api/settings/providers/{name}", {
    params: { path: { name } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new SettingsApiError(`Provider not found: ${name}`, 404);
    throw new SettingsApiError(
      defaultApiErrorMessage(`Failed to delete provider "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to delete provider "${name}"`);
}

export async function listSettingsSandboxes(
  signal?: AbortSignal
): Promise<SettingsSandboxCollection> {
  const { data, error, response } = await apiClient.GET("/api/settings/sandboxes", { signal });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to list settings sandboxes", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to list settings sandboxes");
}

export async function getSettingsSandbox(
  name: string,
  signal?: AbortSignal
): Promise<SettingsSandboxDetail> {
  const { data, error, response } = await apiClient.GET("/api/settings/sandboxes/{name}", {
    params: { path: { name } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new SettingsApiError(`Sandbox not found: ${name}`, 404);
    throw new SettingsApiError(
      defaultApiErrorMessage(`Failed to load sandbox "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to load sandbox "${name}"`).sandbox;
}

export async function putSettingsSandbox(
  name: string,
  body: SettingsSandboxRequest,
  signal?: AbortSignal
): Promise<SettingsMutationResult> {
  const { data, error, response } = await apiClient.PUT("/api/settings/sandboxes/{name}", {
    params: { path: { name } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage(`Failed to save sandbox "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to save sandbox "${name}"`);
}

export async function deleteSettingsSandbox(
  name: string,
  signal?: AbortSignal
): Promise<SettingsMutationResult> {
  const { data, error, response } = await apiClient.DELETE("/api/settings/sandboxes/{name}", {
    params: { path: { name } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new SettingsApiError(`Sandbox not found: ${name}`, 404);
    throw new SettingsApiError(
      defaultApiErrorMessage(`Failed to delete sandbox "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to delete sandbox "${name}"`);
}

export async function listSettingsHooks(signal?: AbortSignal): Promise<SettingsHookCollection> {
  const { data, error, response } = await apiClient.GET("/api/settings/hooks", { signal });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to list settings hooks", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to list settings hooks");
}

export async function putSettingsHook(
  name: string,
  body: SettingsHookRequest,
  signal?: AbortSignal
): Promise<SettingsMutationResult> {
  const { data, error, response } = await apiClient.PUT("/api/settings/hooks/{name}", {
    params: { path: { name } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage(`Failed to save hook "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to save hook "${name}"`);
}

export async function deleteSettingsHook(
  name: string,
  signal?: AbortSignal
): Promise<SettingsMutationResult> {
  const { data, error, response } = await apiClient.DELETE("/api/settings/hooks/{name}", {
    params: { path: { name } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new SettingsApiError(`Hook not found: ${name}`, 404);
    throw new SettingsApiError(
      defaultApiErrorMessage(`Failed to delete hook "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to delete hook "${name}"`);
}

export async function listSettingsMCPServers(
  filter: SettingsMCPServerListFilter = {},
  signal?: AbortSignal
): Promise<SettingsMCPServerCollection> {
  const { data, error, response } = await apiClient.GET("/api/settings/mcp-servers", {
    params: { query: normalizeMCPListFilter(filter) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to list MCP servers", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to list MCP servers");
}

export async function putSettingsMCPServer(
  name: string,
  body: SettingsMCPServerRequest,
  filter: SettingsMCPServerPutFilter = {},
  signal?: AbortSignal
): Promise<SettingsMutationResult> {
  const { data, error, response } = await apiClient.PUT("/api/settings/mcp-servers/{name}", {
    params: { path: { name }, query: normalizeMCPMutationFilter(filter) },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage(`Failed to save MCP server "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to save MCP server "${name}"`);
}

export async function deleteSettingsMCPServer(
  name: string,
  filter: SettingsMCPServerDeleteFilter = {},
  signal?: AbortSignal
): Promise<SettingsMutationResult> {
  const { data, error, response } = await apiClient.DELETE("/api/settings/mcp-servers/{name}", {
    params: { path: { name }, query: normalizeMCPMutationFilter(filter) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new SettingsApiError(`MCP server not found: ${name}`, 404);
    throw new SettingsApiError(
      defaultApiErrorMessage(`Failed to delete MCP server "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to delete MCP server "${name}"`);
}
