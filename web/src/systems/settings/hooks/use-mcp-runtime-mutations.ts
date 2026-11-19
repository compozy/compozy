import { useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query";

import {
  checkSettingsMCPServer,
  importRuntimeMCPServer,
  putSettingsMCPApproval,
  putSettingsProvider,
  setSettingsMCPServerEnabled,
  SettingsApiError,
} from "../adapters/settings-api";
import { providerDraftFromEntry, providerDraftToRequest } from "../lib/provider-draft";
import { settingsKeys } from "../lib/query-keys";
import { settingsProvidersListOptions } from "../lib/query-options";
import { mcpDefinitionKey } from "../lib/mcp-management-target";
import type {
  RuntimeMCPImportRequest,
  SettingsMCPApprovalRequest,
  SettingsMCPAuthFilter,
  SettingsMCPEnablementFilter,
  SettingsMCPServerCollection,
  SettingsMCPServerEntry,
} from "../types";
import {
  invalidateSettingsApplyRecords,
  recordSettingsMutation,
} from "./settings-mutation-helpers";
import { invalidateMCPState } from "./use-settings-mutations";

export type RuntimeMCPMode = "load" | "off";

/** Approval, copy and the runtime switch all change what the MCP list and runtime groups show. */
function invalidateMCPAndApply(queryClient: QueryClient) {
  return Promise.all([
    invalidateMCPState(queryClient),
    invalidateSettingsApplyRecords(queryClient),
  ]);
}

interface AllowParams {
  name: string;
  body: SettingsMCPApprovalRequest;
}

/**
 * Allows one waiting project server with the digest the row showed. Settled either way: a
 * `409 mcp_approval_definition_changed` rereads the list so the row shows the new definition.
 */
export function useAllowMCPServer() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ name, body }: AllowParams) => putSettingsMCPApproval(name, body),
    onSettled: () => invalidateMCPState(queryClient),
  });
}

interface CheckParams {
  entry: SettingsMCPServerEntry;
  filter: SettingsMCPAuthFilter;
}

export function useSetMCPServerEnabled() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      entry,
      filter,
      enabled,
    }: {
      entry: SettingsMCPServerEntry;
      filter: SettingsMCPEnablementFilter;
      enabled: boolean;
    }) => setSettingsMCPServerEnabled(entry.name, filter, enabled),
    onSettled: () => invalidateMCPAndApply(queryClient),
  });
}

/**
 * Check again for one definition. The daemon answers with the item and its new runtime status;
 * every cached list keeps its own row (identity, scope, owner and source as that lens shows it)
 * and takes only the new status, so the row reads the answer at once. The lists then reread.
 */
export function useCheckMCPServer() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ entry, filter }: CheckParams) => checkSettingsMCPServer(entry.name, filter),
    onSuccess: (item, { entry }) => {
      const key = mcpDefinitionKey(entry);
      queryClient.setQueriesData<SettingsMCPServerCollection>(
        { queryKey: settingsKeys.mcpLists() },
        list => {
          if (!list?.mcp_servers.some(server => mcpDefinitionKey(server) === key)) return list;
          return {
            ...list,
            mcp_servers: list.mcp_servers.map(server =>
              mcpDefinitionKey(server) === key
                ? { ...server, runtime_status: item.runtime_status }
                : server
            ),
          };
        }
      );
      void queryClient.invalidateQueries({ queryKey: settingsKeys.mcpLists() });
    },
  });
}

interface ImportParams {
  runtime: string;
  name: string;
  body: RuntimeMCPImportRequest;
}

/** Copies a runtime server into CompozyOS; the CompozyOS list and the runtime groups reread. */
export function useImportRuntimeMCPServer() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ runtime, name, body }: ImportParams) =>
      importRuntimeMCPServer(runtime, name, body),
    onSettled: () => invalidateMCPAndApply(queryClient),
  });
}

interface RuntimeModeParams {
  /** Provider config key (`claude`, `copilot`, `qwen`), not the inventory's runtime id. */
  provider: string;
  mode: RuntimeMCPMode;
}

/**
 * Writes `providers.<name>.runtime_mcp`. The provider write is a full table rebuild, so the request
 * starts from the provider's current settings and changes only this key.
 */
export function useSetRuntimeMCPMode() {
  const queryClient = useQueryClient();
  return useMutation({
    networkMode: "always",
    mutationFn: async ({ provider, mode }: RuntimeModeParams) => {
      const collection = await queryClient.fetchQuery(settingsProvidersListOptions());
      const entry = collection.providers.find(candidate => candidate.name === provider);
      if (!entry) {
        throw new SettingsApiError(`Couldn't find ${provider} in Settings › Providers.`, 404);
      }
      const request = providerDraftToRequest(providerDraftFromEntry(entry));
      return putSettingsProvider(provider, {
        ...request,
        settings: { ...request.settings, runtime_mcp: mode },
      });
    },
    onSuccess: recordSettingsMutation,
    onSettled: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: settingsKeys.providersRoot() }),
        invalidateMCPAndApply(queryClient),
      ]),
  });
}
