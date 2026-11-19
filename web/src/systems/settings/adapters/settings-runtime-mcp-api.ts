import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  RuntimeMCPImportRequest,
  RuntimeMCPImportResult,
  RuntimeMCPInventory,
  RuntimeMCPInventoryFilter,
  SettingsMCPApprovalRequest,
  SettingsMCPApprovalResult,
  SettingsMCPAuthFilter,
  SettingsMCPServerEntry,
} from "../types";
import { normalizeOptionalText, SettingsApiError } from "./settings-api-error";
import { normalizeSettingsLayerFilter } from "./settings-layer-filter";

/** Every runtime's own MCP servers, read from its files by the daemon. Never probes a server. */
export async function listRuntimeMCPServers(
  filter: RuntimeMCPInventoryFilter = {},
  signal?: AbortSignal
): Promise<RuntimeMCPInventory> {
  const { data, error, response } = await apiClient.GET("/api/settings/runtime-mcp-servers", {
    params: {
      query: {
        ...normalizeSettingsLayerFilter(filter),
        runtime: filter.runtime?.trim() || undefined,
      },
    },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Couldn't read your coding agents' MCP servers", response, error),
      response.status,
      undefined,
      error?.code
    );
  }
  return requireResponseData(data, response, "Couldn't read your coding agents' MCP servers");
}

/**
 * Copies one runtime server into CompozyOS. Values move into CompozyOS secrets daemon-side; the
 * request carries only the target scope and name. `409 mcp_import_name_taken` and
 * `404 mcp_server_not_found` keep their code for the dialog.
 */
export async function importRuntimeMCPServer(
  runtime: string,
  name: string,
  body: RuntimeMCPImportRequest,
  signal?: AbortSignal
): Promise<RuntimeMCPImportResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/settings/runtime-mcp-servers/{runtime}/{name}/import",
    { params: { path: { runtime, name } }, body, signal }
  );
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage(`Couldn't copy ${name} to CompozyOS`, response, error),
      response.status,
      undefined,
      error?.code
    );
  }
  return requireResponseData(data, response, `Couldn't copy ${name} to CompozyOS`);
}

/**
 * Allows one project-file server for the current profile, bound to the definition digest the row
 * showed. A changed file answers `409 mcp_approval_definition_changed` and records nothing.
 */
export async function putSettingsMCPApproval(
  name: string,
  body: SettingsMCPApprovalRequest,
  signal?: AbortSignal
): Promise<SettingsMCPApprovalResult> {
  const { data, error, response } = await apiClient.PUT(
    "/api/settings/mcp-servers/{name}/approval",
    { params: { path: { name } }, body, signal }
  );
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage(`Couldn't allow ${name}`, response, error),
      response.status,
      undefined,
      error?.code
    );
  }
  return requireResponseData(data, response, `Couldn't allow ${name}`);
}

/**
 * Check again: the daemon probes one definition (its exact scope and owner) and answers with the
 * Settings item carrying the new `runtime_status`.
 */
export async function checkSettingsMCPServer(
  name: string,
  filter: SettingsMCPAuthFilter,
  signal?: AbortSignal
): Promise<SettingsMCPServerEntry> {
  const { data, error, response } = await apiClient.POST("/api/settings/mcp-servers/{name}/check", {
    params: {
      path: { name },
      query: {
        ...normalizeSettingsLayerFilter(filter),
        owner: normalizeOptionalText(filter.owner),
      },
    },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage(`Couldn't check ${name}`, response, error),
      response.status,
      undefined,
      error?.code
    );
  }
  return requireResponseData(data, response, `Couldn't check ${name}`);
}

export async function setSettingsMCPServerEnabled(
  name: string,
  filter: SettingsMCPAuthFilter,
  enabled: boolean
): Promise<SettingsMCPServerEntry> {
  const { data, error, response } = await apiClient.PUT(
    "/api/settings/mcp-servers/{name}/enabled",
    {
      params: {
        path: { name },
        query: {
          ...normalizeSettingsLayerFilter(filter),
          owner: normalizeOptionalText(filter.owner),
        },
      },
      body: { enabled },
    }
  );
  const message = `Couldn't turn ${name} ${enabled ? "on" : "off"}`;
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage(message, response, error),
      response.status,
      undefined,
      error?.code
    );
  }
  return requireResponseData(data, response, message).server;
}
