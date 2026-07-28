import { useMutation, useQueryClient } from "@tanstack/react-query";

import { marketplaceKeys } from "@/systems/marketplace";

import {
  deleteSettingsSandbox,
  deleteSettingsHook,
  deleteSettingsMCPServer,
  deleteSettingsProvider,
  createSettingsNotificationPreset,
  deleteSettingsNotificationPreset,
  putSettingsSandbox,
  putSettingsHook,
  putSettingsMCPServer,
  putSettingsProvider,
  reloadSettings,
  updateSettingsNotificationPreset,
  updateSettingsAutomation,
  updateSettingsGeneral,
  updateSettingsHooksExtensions,
  updateSettingsMemory,
  updateSettingsNetwork,
  updateSettingsObservability,
  updateSettingsRoles,
  updateSettingsSkills,
} from "../adapters/settings-api";
import {
  beginSettingsMCPAuth,
  exchangeSettingsMCPAuth,
  logoutSettingsMCPAuth,
} from "../adapters/settings-mcp-auth-api";
import { settingsKeys } from "../lib/query-keys";
import { useSettingsRestartStore } from "../stores/use-settings-restart-store";
import type {
  SettingsSandboxRequest,
  SettingsHookRequest,
  SettingsCreateNotificationPresetRequest,
  SettingsNotificationPresetEntry,
  SettingsMCPAuthExchangeRequest,
  SettingsMCPAuthBeginMode,
  SettingsMCPAuthFilter,
  SettingsMCPServerDeleteFilter,
  SettingsMCPServerPutFilter,
  SettingsMCPServerRequest,
  SettingsMutationResult,
  SettingsUpdateNotificationPresetRequest,
  SettingsProviderRequest,
  SettingsUpdateAutomationRequest,
  SettingsUpdateGeneralRequest,
  SettingsUpdateHooksExtensionsRequest,
  SettingsUpdateMemoryRequest,
  SettingsUpdateNetworkRequest,
  SettingsUpdateObservabilityRequest,
  SettingsRolesSection,
  SettingsUpdateRolesRequest,
  SettingsSectionName,
  SettingsUpdateSkillsFilter,
  SettingsUpdateSkillsRequest,
} from "../types";

function recordMutation(result: SettingsMutationResult) {
  useSettingsRestartStore.getState().recordMutation({
    section: result.section,
    restartRequired: Boolean(result.restart_required),
    restartScope: result.restart_scope,
    warnings: result.warnings ?? [],
    lifecycle: result.lifecycle,
    nextAction: result.next_action,
    applyRecordId: result.apply_record_id,
    activeGeneration: result.active_generation,
    completedAt: new Date().toISOString(),
  });
}

function invalidateApplyRecords(queryClient: ReturnType<typeof useQueryClient>) {
  return queryClient.invalidateQueries({ queryKey: settingsKeys.applyRoot() });
}

function invalidateSection(
  queryClient: ReturnType<typeof useQueryClient>,
  section: SettingsSectionName
) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: settingsKeys.section(section) }),
    invalidateApplyRecords(queryClient),
  ]);
}

function invalidateRoles(queryClient: ReturnType<typeof useQueryClient>) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: settingsKeys.section("roles") }),
    queryClient.invalidateQueries({ queryKey: settingsKeys.rolesStatus() }),
    invalidateApplyRecords(queryClient),
  ]);
}

function invalidateProviders(queryClient: ReturnType<typeof useQueryClient>, name?: string) {
  const tasks = [
    queryClient.invalidateQueries({ queryKey: settingsKeys.providersRoot() }),
    invalidateApplyRecords(queryClient),
  ];

  if (name) {
    tasks.push(queryClient.invalidateQueries({ queryKey: settingsKeys.providerDetail(name) }));
  }

  return Promise.all(tasks);
}

function invalidateSandboxes(queryClient: ReturnType<typeof useQueryClient>, name?: string) {
  const tasks = [
    queryClient.invalidateQueries({ queryKey: settingsKeys.sandboxesRoot() }),
    invalidateApplyRecords(queryClient),
  ];

  if (name) {
    tasks.push(queryClient.invalidateQueries({ queryKey: settingsKeys.sandboxDetail(name) }));
  }

  return Promise.all(tasks);
}

function invalidateHooks(queryClient: ReturnType<typeof useQueryClient>) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: settingsKeys.hooksRoot() }),
    queryClient.invalidateQueries({ queryKey: settingsKeys.section("hooks-extensions") }),
    invalidateApplyRecords(queryClient),
  ]);
}

function invalidateMCPServers(queryClient: ReturnType<typeof useQueryClient>) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: settingsKeys.mcpRoot() }),
    invalidateApplyRecords(queryClient),
  ]);
}

export function useReloadSettings() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: () => reloadSettings(),
    onSuccess: recordMutation,
    onSettled: () => queryClient.invalidateQueries({ queryKey: settingsKeys.all }),
  });
}

export function useUpdateSettingsGeneral() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (body: SettingsUpdateGeneralRequest) => updateSettingsGeneral(body),
    onSuccess: recordMutation,
    onSettled: () => invalidateSection(queryClient, "general"),
  });
}

export function useUpdateSettingsMemory() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (body: SettingsUpdateMemoryRequest) => updateSettingsMemory(body),
    onSuccess: recordMutation,
    onSettled: () => invalidateSection(queryClient, "memory"),
  });
}

export function useUpdateSettingsRoles() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (body: SettingsUpdateRolesRequest) => updateSettingsRoles(body),
    onSuccess: (result, variables) => {
      recordMutation(result);
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
    mutationFn: ({
      body,
      filter,
    }: {
      body: SettingsUpdateSkillsRequest;
      filter?: SettingsUpdateSkillsFilter;
    }) => updateSettingsSkills(body, filter ?? {}),
    onSuccess: recordMutation,
    onSettled: () => invalidateSection(queryClient, "skills"),
  });
}

export function useUpdateSettingsAutomation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (body: SettingsUpdateAutomationRequest) => updateSettingsAutomation(body),
    onSuccess: recordMutation,
    onSettled: () => invalidateSection(queryClient, "automation"),
  });
}

export function useUpdateSettingsNetwork() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (body: SettingsUpdateNetworkRequest) => updateSettingsNetwork(body),
    onSuccess: recordMutation,
    onSettled: () => invalidateSection(queryClient, "network"),
  });
}

export function useUpdateSettingsObservability() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (body: SettingsUpdateObservabilityRequest) => updateSettingsObservability(body),
    onSuccess: recordMutation,
    onSettled: () => invalidateSection(queryClient, "observability"),
  });
}

export function useUpdateSettingsHooksExtensions() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (body: SettingsUpdateHooksExtensionsRequest) => updateSettingsHooksExtensions(body),
    onSuccess: recordMutation,
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
    onSuccess: recordMutation,
    onSettled: (_result, _error, variables) => invalidateProviders(queryClient, variables?.name),
  });
}

export function useDeleteSettingsProvider() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (name: string) => deleteSettingsProvider(name),
    onSuccess: recordMutation,
    onSettled: (_result, _error, name) => invalidateProviders(queryClient, name),
  });
}

export function usePutSettingsSandbox() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name, body }: NameBodyParams<SettingsSandboxRequest>) =>
      putSettingsSandbox(name, body),
    onSuccess: recordMutation,
    onSettled: (_result, _error, variables) => invalidateSandboxes(queryClient, variables?.name),
  });
}

export function useDeleteSettingsSandbox() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (name: string) => deleteSettingsSandbox(name),
    onSuccess: recordMutation,
    onSettled: (_result, _error, name) => invalidateSandboxes(queryClient, name),
  });
}

export function usePutSettingsHook() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name, body }: NameBodyParams<SettingsHookRequest>) =>
      putSettingsHook(name, body),
    onSuccess: recordMutation,
    onSettled: () => invalidateHooks(queryClient),
  });
}

export function useDeleteSettingsHook() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (name: string) => deleteSettingsHook(name),
    onSuccess: recordMutation,
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

interface SettingsNotificationPresetUpdateParams {
  name: string;
  body: SettingsUpdateNotificationPresetRequest;
}

export function usePutSettingsMCPServer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name, body, filter }: MCPPutParams) =>
      putSettingsMCPServer(name, body, filter ?? {}),
    onSuccess: recordMutation,
    onSettled: () => invalidateMCPServers(queryClient),
  });
}

export function useDeleteSettingsMCPServer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name, filter }: MCPDeleteParams) => deleteSettingsMCPServer(name, filter ?? {}),
    onSuccess: recordMutation,
    onSettled: () => invalidateMCPServers(queryClient),
  });
}

interface MCPAuthParams {
  name: string;
  filter: SettingsMCPAuthFilter;
}

interface MCPAuthBeginParams extends MCPAuthParams {
  mode: SettingsMCPAuthBeginMode;
}

interface MCPAuthExchangeParams extends MCPAuthParams {
  body: SettingsMCPAuthExchangeRequest;
}

export function useBeginMCPAuth() {
  // react-doctor-disable-next-line react-doctor/query-mutation-missing-invalidation -- Begin creates only an ephemeral PKCE session; no cached server state changes.
  return useMutation({
    mutationFn: ({ name, filter, mode }: MCPAuthBeginParams) =>
      beginSettingsMCPAuth(name, filter, { mode }),
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
    onSettled: () => queryClient.invalidateQueries({ queryKey: settingsKeys.mcpRoot() }),
  });
}

export function useLogoutMCPAuth() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name, filter }: MCPAuthParams) => logoutSettingsMCPAuth(name, filter),
    onSettled: () => queryClient.invalidateQueries({ queryKey: settingsKeys.mcpRoot() }),
  });
}

function invalidateNotificationPresets(queryClient: ReturnType<typeof useQueryClient>) {
  return queryClient.invalidateQueries({ queryKey: settingsKeys.notificationsRoot() });
}

export function useCreateSettingsNotificationPreset() {
  const queryClient = useQueryClient();

  return useMutation<
    SettingsNotificationPresetEntry,
    Error,
    SettingsCreateNotificationPresetRequest
  >({
    mutationFn: body => createSettingsNotificationPreset(body),
    onSettled: () => invalidateNotificationPresets(queryClient),
  });
}

export function useUpdateSettingsNotificationPreset() {
  const queryClient = useQueryClient();

  return useMutation<
    SettingsNotificationPresetEntry,
    Error,
    SettingsNotificationPresetUpdateParams
  >({
    mutationFn: ({ name, body }) => updateSettingsNotificationPreset(name, body),
    onSettled: () => invalidateNotificationPresets(queryClient),
  });
}

export function useDeleteSettingsNotificationPreset() {
  const queryClient = useQueryClient();

  return useMutation<void, Error, string>({
    mutationFn: name => deleteSettingsNotificationPreset(name),
    onSettled: () => invalidateNotificationPresets(queryClient),
  });
}
