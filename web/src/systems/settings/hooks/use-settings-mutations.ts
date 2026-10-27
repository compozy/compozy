import { useMutation, useQueryClient } from "@tanstack/react-query";

import { extensionKeys } from "@/systems/extensions";
import { marketplaceKeys } from "@/systems/marketplace";

import {
  deleteSettingsHook,
  deleteSettingsMCPServer,
  deleteSettingsProvider,
  putSettingsHook,
  putSettingsMCPServer,
  putSettingsProvider,
  reloadSettings,
  updateSettingsAutomation,
  updateSettingsGeneral,
  updateSettingsHooksExtensions,
  updateSettingsObservability,
  updateSettingsPersona,
  updateSettingsRoles,
  updateSettingsShell,
  updateSettingsSkills,
} from "../adapters/settings-api";
import {
  beginSettingsMCPAuth,
  exchangeSettingsMCPAuth,
  logoutSettingsMCPAuth,
} from "../adapters/settings-mcp-auth-api";
import { settingsKeys } from "../lib/query-keys";
import type {
  SettingsHookRequest,
  SettingsHookDeleteFilter,
  SettingsHookPutFilter,
  SettingsMCPAuthBeginRequest,
  SettingsMCPAuthExchangeRequest,
  SettingsMCPAuthFilter,
  SettingsMCPServerDeleteFilter,
  SettingsMCPServerPutFilter,
  SettingsMCPServerRequest,
  SettingsProviderRequest,
  SettingsPersonaFilter,
  SettingsUpdateAutomationRequest,
  SettingsUpdateGeneralRequest,
  SettingsUpdateHooksExtensionsRequest,
  SettingsUpdateShellRequest,
  SettingsUpdateObservabilityRequest,
  SettingsUpdatePersonaRequest,
  SettingsRolesSection,
  SettingsUpdateRolesRequest,
  SettingsShellSection,
  SettingsSectionName,
  SettingsUpdateSkillsFilter,
  SettingsUpdateSkillsRequest,
} from "../types";
import {
  invalidateSettingsApplyRecords,
  recordSettingsMutation,
} from "./settings-mutation-helpers";

function invalidateSection(
  queryClient: ReturnType<typeof useQueryClient>,
  section: SettingsSectionName
) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: settingsKeys.section(section) }),
    invalidateSettingsApplyRecords(queryClient),
  ]);
}

function invalidateRoles(queryClient: ReturnType<typeof useQueryClient>) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: settingsKeys.section("roles") }),
    queryClient.invalidateQueries({ queryKey: settingsKeys.rolesStatus() }),
    invalidateSettingsApplyRecords(queryClient),
  ]);
}

function invalidateProviders(queryClient: ReturnType<typeof useQueryClient>, name?: string) {
  const tasks = [
    queryClient.invalidateQueries({ queryKey: settingsKeys.providersRoot() }),
    invalidateSettingsApplyRecords(queryClient),
  ];

  if (name) {
    tasks.push(queryClient.invalidateQueries({ queryKey: settingsKeys.providerDetail(name) }));
  }

  return Promise.all(tasks);
}

function invalidateHooks(queryClient: ReturnType<typeof useQueryClient>) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: settingsKeys.hooksRoot() }),
    queryClient.invalidateQueries({ queryKey: settingsKeys.section("hooks-extensions") }),
    invalidateSettingsApplyRecords(queryClient),
  ]);
}

export function invalidateMCPState(queryClient: ReturnType<typeof useQueryClient>, owner?: string) {
  const tasks = [queryClient.invalidateQueries({ queryKey: settingsKeys.mcpRoot() })];
  if (owner?.startsWith("extension:")) {
    tasks.push(
      queryClient.invalidateQueries({ queryKey: extensionKeys.all }),
      queryClient.invalidateQueries({ queryKey: marketplaceKeys.all })
    );
  }
  return Promise.all(tasks);
}

function invalidateMCPServers(queryClient: ReturnType<typeof useQueryClient>, owner?: string) {
  return Promise.all([
    invalidateMCPState(queryClient, owner),
    invalidateSettingsApplyRecords(queryClient),
  ]);
}

export function useReloadSettings() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: () => reloadSettings(),
    onSuccess: recordSettingsMutation,
    onSettled: () => queryClient.invalidateQueries({ queryKey: settingsKeys.all }),
  });
}

export function useUpdateSettingsGeneral() {
  const queryClient = useQueryClient();

  return useMutation({
    networkMode: "always",
    mutationFn: (body: SettingsUpdateGeneralRequest) => updateSettingsGeneral(body),
    onSuccess: recordSettingsMutation,
    onSettled: () => invalidateSection(queryClient, "general"),
  });
}

export function useUpdateSettingsPersona() {
  const queryClient = useQueryClient();

  return useMutation({
    networkMode: "always",
    mutationFn: ({
      body,
      filter,
    }: {
      body: SettingsUpdatePersonaRequest;
      filter: SettingsPersonaFilter;
    }) => updateSettingsPersona(body, filter),
    onSuccess: recordSettingsMutation,
    onSettled: (_result, _error, variables) =>
      Promise.all([
        queryClient.invalidateQueries({
          queryKey: settingsKeys.personaSection(variables.filter),
        }),
        invalidateSettingsApplyRecords(queryClient),
      ]),
  });
}

export function useUpdateSettingsRoles() {
  const queryClient = useQueryClient();

  return useMutation({
    networkMode: "always",
    mutationFn: (body: SettingsUpdateRolesRequest) => updateSettingsRoles(body),
    onSuccess: (result, variables) => {
      recordSettingsMutation(result);
      // Reflect the applied section immediately so the saved confirmation shows
      // without a dirty flicker; onSettled refetches both reads to confirm.
      queryClient.setQueryData<SettingsRolesSection>(settingsKeys.section("roles"), previous =>
        previous ? { ...previous, config: variables.config } : previous
      );
    },
    onSettled: () => invalidateRoles(queryClient),
  });
}

export function useUpdateSettingsSkills() {
  const queryClient = useQueryClient();

  return useMutation({
    networkMode: "always",
    mutationFn: ({
      body,
      filter,
    }: {
      body: SettingsUpdateSkillsRequest;
      filter?: SettingsUpdateSkillsFilter;
    }) => updateSettingsSkills(body, filter ?? {}),
    onSuccess: recordSettingsMutation,
    onSettled: () => invalidateSection(queryClient, "skills"),
  });
}

export function useUpdateSettingsAutomation() {
  const queryClient = useQueryClient();

  return useMutation({
    networkMode: "always",
    mutationFn: (body: SettingsUpdateAutomationRequest) => updateSettingsAutomation(body),
    onSuccess: recordSettingsMutation,
    onSettled: () => invalidateSection(queryClient, "automation"),
  });
}

export function useUpdateSettingsShell() {
  const queryClient = useQueryClient();

  const key = settingsKeys.section("shell");

  // Optimistic: an Appearance font repaints the whole client at once. Omitted
  // keys are preserved by the daemon, so the cache merges the patch the same way.
  return useMutation({
    mutationFn: (body: SettingsUpdateShellRequest) => updateSettingsShell(body),
    onMutate: async variables => {
      await queryClient.cancelQueries({ queryKey: key });
      const previous = queryClient.getQueryData<SettingsShellSection>(key);
      queryClient.setQueryData<SettingsShellSection>(key, current =>
        current
          ? { ...current, config: mergeShellConfig(current.config, variables.config) }
          : current
      );
      return { previous };
    },
    onError: (_error, variables, context) => {
      const previous = context?.previous;
      if (!previous) return;
      queryClient.setQueryData<SettingsShellSection>(key, current =>
        current
          ? {
              ...current,
              config: revertShellPatch(current.config, previous.config, variables.config),
            }
          : current
      );
    },
    onSuccess: result => recordSettingsMutation(result),
    // Not awaited: a caller's error feedback must not wait on the reread, which
    // retries with backoff while the daemon is unreachable.
    onSettled: () => {
      void invalidateSection(queryClient, "shell");
    },
  });
}

type ShellConfigSection = "sessions" | "appearance";

/**
 * Undoes only the keys this patch wrote, and only where the cache still holds
 * its optimistic value. Restoring the whole snapshot would also revert (or
 * resurrect) a concurrent change to another key: two overlapping saves that
 * both fail must leave neither choice behind.
 */
function revertShellPatch(
  current: SettingsShellSection["config"],
  previous: SettingsShellSection["config"],
  patch: SettingsUpdateShellRequest["config"]
): SettingsShellSection["config"] {
  const next = { ...current };
  for (const section of ["sessions", "appearance"] as const satisfies ShellConfigSection[]) {
    const written = Object.entries(patch[section] ?? {});
    if (written.length === 0) continue;
    const values: Record<string, unknown> = { ...current[section] };
    const before: Record<string, unknown> = { ...previous[section] };
    for (const [field, value] of written) {
      if (value !== undefined && values[field] === value) values[field] = before[field];
    }
    Object.assign(next, { [section]: values });
  }
  return next;
}

function mergeShellConfig(
  current: SettingsShellSection["config"],
  patch: SettingsUpdateShellRequest["config"]
): SettingsShellSection["config"] {
  return {
    ...current,
    sessions: { ...current.sessions, ...patch.sessions },
    appearance: { ...current.appearance, ...patch.appearance },
  };
}

export function useUpdateSettingsObservability() {
  const queryClient = useQueryClient();

  return useMutation({
    networkMode: "always",
    mutationFn: (body: SettingsUpdateObservabilityRequest) => updateSettingsObservability(body),
    onSuccess: recordSettingsMutation,
    onSettled: () => invalidateSection(queryClient, "observability"),
  });
}

export function useUpdateSettingsHooksExtensions() {
  const queryClient = useQueryClient();

  return useMutation({
    networkMode: "always",
    mutationFn: (body: SettingsUpdateHooksExtensionsRequest) => updateSettingsHooksExtensions(body),
    onSuccess: recordSettingsMutation,
    onSettled: () =>
      Promise.all([
        invalidateSection(queryClient, "hooks-extensions"),
        queryClient.invalidateQueries({ queryKey: marketplaceKeys.all }),
      ]),
  });
}

interface NameBodyParams<Body> {
  name: string;
  body: Body;
}

export function usePutSettingsProvider() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name, body }: NameBodyParams<SettingsProviderRequest>) =>
      putSettingsProvider(name, body),
    onSuccess: recordSettingsMutation,
    onSettled: (_result, _error, variables) => invalidateProviders(queryClient, variables?.name),
  });
}

export function useDeleteSettingsProvider() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (name: string) => deleteSettingsProvider(name),
    onSuccess: recordSettingsMutation,
    onSettled: (_result, _error, name) => invalidateProviders(queryClient, name),
  });
}

export function usePutSettingsHook() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      name,
      body,
      filter,
    }: NameBodyParams<SettingsHookRequest> & { filter?: SettingsHookPutFilter }) =>
      putSettingsHook(name, body, filter ?? {}),
    onSuccess: recordSettingsMutation,
    onSettled: () => invalidateHooks(queryClient),
  });
}

export function useDeleteSettingsHook() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name, filter }: { name: string; filter?: SettingsHookDeleteFilter }) =>
      deleteSettingsHook(name, filter ?? {}),
    onSuccess: recordSettingsMutation,
    onSettled: () => invalidateHooks(queryClient),
  });
}

interface MCPPutParams {
  name: string;
  body: SettingsMCPServerRequest;
  filter?: SettingsMCPServerPutFilter;
}

interface MCPDeleteParams {
  name: string;
  filter?: SettingsMCPServerDeleteFilter;
}

export function usePutSettingsMCPServer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name, body, filter }: MCPPutParams) =>
      putSettingsMCPServer(name, body, filter ?? {}),
    onSuccess: recordSettingsMutation,
    onSettled: (_data, _error, variables) =>
      invalidateMCPServers(queryClient, variables.filter?.owner),
  });
}

export function useDeleteSettingsMCPServer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name, filter }: MCPDeleteParams) => deleteSettingsMCPServer(name, filter ?? {}),
    onSuccess: recordSettingsMutation,
    onSettled: (_data, _error, variables) =>
      invalidateMCPServers(queryClient, variables.filter?.owner),
  });
}

interface MCPAuthParams {
  name: string;
  filter: SettingsMCPAuthFilter;
}

interface MCPAuthBeginParams extends MCPAuthParams {
  body: SettingsMCPAuthBeginRequest;
}

interface MCPAuthExchangeParams extends MCPAuthParams {
  body: SettingsMCPAuthExchangeRequest;
}

export function useBeginMCPAuth() {
  // react-doctor-disable-next-line react-doctor/query-mutation-missing-invalidation -- Begin creates only an ephemeral PKCE session; no cached server state changes.
  return useMutation({
    mutationFn: ({ name, filter, body }: MCPAuthBeginParams) =>
      beginSettingsMCPAuth(name, filter, body),
  });
}

// Exchange/logout change credential state; refetch the canonical scoped list so
// auth_status re-reads. These are runtime auth ops, not config edits, so they do
// not record a pending restart.
export function useExchangeMCPAuth() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name, filter, body }: MCPAuthExchangeParams) =>
      exchangeSettingsMCPAuth(name, filter, body),
    onSettled: (_data, _error, variables) =>
      invalidateMCPState(queryClient, variables.filter.owner),
  });
}

export function useLogoutMCPAuth() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name, filter }: MCPAuthParams) => logoutSettingsMCPAuth(name, filter),
    onSettled: (_data, _error, variables) =>
      invalidateMCPState(queryClient, variables.filter.owner),
  });
}
export { useUpdateSettingsAttention } from "./use-settings-attention-mutation";
