import { queryOptions } from "@tanstack/react-query";

import {
  SettingsApiError,
  getSettingsAutomation,
  getSettingsSandbox,
  getSettingsGeneral,
  getSettingsHooksExtensions,
  getSettingsMemory,
  getSettingsNetwork,
  listSettingsNotificationPresets,
  getSettingsObservability,
  getSettingsProvider,
  getSettingsRestartStatus,
  getSettingsRoles,
  getSettingsSkills,
  getSettingsUpdate,
  getRolesStatus,
  listSettingsApplyRecords,
  listSettingsSandboxes,
  listSettingsHooks,
  listSettingsMCPServers,
  listSettingsProviders,
} from "../adapters/settings-api";
import { settingsKeys } from "./query-keys";
import { isTerminalRestartStatus } from "./restart-status";
import type {
  SettingsApplyRecordsFilter,
  SettingsMCPServerListFilter,
  SettingsNotificationPresetFilter,
  SettingsSkillsFilter,
} from "../types";

const SECTION_STALE_TIME = 15_000;
const SECTION_REFETCH_INTERVAL = 60_000;
const COLLECTION_STALE_TIME = 15_000;
const COLLECTION_REFETCH_INTERVAL = 45_000;
const MCP_AUTH_STATUS_POLL_INTERVAL = 1_000;
const APPLY_RECORDS_STALE_TIME = 5_000;
const APPLY_RECORDS_REFETCH_INTERVAL = 30_000;
const RESTART_POLL_INTERVAL = 2_000;
const SETTINGS_QUERY_RETRY_LIMIT = 2;

export function shouldRetrySettingsQuery(failureCount: number, error: Error): boolean {
  if (error instanceof SettingsApiError && error.status === 403) {
    return false;
  }

  return failureCount < SETTINGS_QUERY_RETRY_LIMIT;
}

export function settingsGeneralOptions() {
  return queryOptions({
    queryKey: settingsKeys.section("general"),
    queryFn: ({ signal }) => getSettingsGeneral(signal),
    staleTime: SECTION_STALE_TIME,
    refetchInterval: SECTION_REFETCH_INTERVAL,
    retry: shouldRetrySettingsQuery,
  });
}

export function settingsUpdateOptions() {
  return queryOptions({
    queryKey: settingsKeys.updateStatus(),
    queryFn: ({ signal }) => getSettingsUpdate(signal),
    staleTime: SECTION_STALE_TIME,
    refetchInterval: SECTION_REFETCH_INTERVAL,
    retry: shouldRetrySettingsQuery,
  });
}

export function settingsMemoryOptions() {
  return queryOptions({
    queryKey: settingsKeys.section("memory"),
    queryFn: ({ signal }) => getSettingsMemory(signal),
    staleTime: SECTION_STALE_TIME,
    refetchInterval: SECTION_REFETCH_INTERVAL,
    retry: shouldRetrySettingsQuery,
  });
}

export function settingsRolesStatusOptions() {
  return queryOptions({
    queryKey: settingsKeys.rolesStatus(),
    queryFn: ({ signal }) => getRolesStatus(signal),
    staleTime: SECTION_STALE_TIME,
    refetchInterval: SECTION_REFETCH_INTERVAL,
    retry: shouldRetrySettingsQuery,
  });
}

export function settingsRolesOptions() {
  return queryOptions({
    queryKey: settingsKeys.section("roles"),
    queryFn: ({ signal }) => getSettingsRoles(signal),
    staleTime: SECTION_STALE_TIME,
    refetchInterval: SECTION_REFETCH_INTERVAL,
    retry: shouldRetrySettingsQuery,
  });
}

export function settingsSkillsOptions(filter: SettingsSkillsFilter = {}) {
  return queryOptions({
    queryKey: settingsKeys.skillsSection(filter),
    queryFn: ({ signal }) => getSettingsSkills(filter, signal),
    staleTime: SECTION_STALE_TIME,
    refetchInterval: SECTION_REFETCH_INTERVAL,
    retry: shouldRetrySettingsQuery,
  });
}

export function settingsAutomationOptions() {
  return queryOptions({
    queryKey: settingsKeys.section("automation"),
    queryFn: ({ signal }) => getSettingsAutomation(signal),
    staleTime: SECTION_STALE_TIME,
    refetchInterval: SECTION_REFETCH_INTERVAL,
    retry: shouldRetrySettingsQuery,
  });
}

export function settingsNetworkOptions() {
  return queryOptions({
    queryKey: settingsKeys.section("network"),
    queryFn: ({ signal }) => getSettingsNetwork(signal),
    staleTime: SECTION_STALE_TIME,
    refetchInterval: SECTION_REFETCH_INTERVAL,
    retry: shouldRetrySettingsQuery,
  });
}

export function settingsObservabilityOptions() {
  return queryOptions({
    queryKey: settingsKeys.section("observability"),
    queryFn: ({ signal }) => getSettingsObservability(signal),
    staleTime: SECTION_STALE_TIME,
    refetchInterval: SECTION_REFETCH_INTERVAL,
    retry: shouldRetrySettingsQuery,
  });
}

export function settingsHooksExtensionsOptions() {
  return queryOptions({
    queryKey: settingsKeys.section("hooks-extensions"),
    queryFn: ({ signal }) => getSettingsHooksExtensions(signal),
    staleTime: SECTION_STALE_TIME,
    refetchInterval: SECTION_REFETCH_INTERVAL,
    retry: shouldRetrySettingsQuery,
  });
}

export function settingsProvidersListOptions() {
  return queryOptions({
    queryKey: settingsKeys.providersList(),
    queryFn: ({ signal }) => listSettingsProviders(signal),
    staleTime: COLLECTION_STALE_TIME,
    refetchInterval: COLLECTION_REFETCH_INTERVAL,
    retry: shouldRetrySettingsQuery,
  });
}

export function settingsProviderDetailOptions(name: string, enabled = true) {
  return queryOptions({
    queryKey: settingsKeys.providerDetail(name),
    queryFn: ({ signal }) => getSettingsProvider(name, signal),
    staleTime: COLLECTION_STALE_TIME,
    refetchInterval: COLLECTION_REFETCH_INTERVAL,
    enabled: Boolean(name) && enabled,
    retry: shouldRetrySettingsQuery,
  });
}

export function settingsSandboxesListOptions() {
  return queryOptions({
    queryKey: settingsKeys.sandboxesList(),
    queryFn: ({ signal }) => listSettingsSandboxes(signal),
    staleTime: COLLECTION_STALE_TIME,
    refetchInterval: COLLECTION_REFETCH_INTERVAL,
    retry: shouldRetrySettingsQuery,
  });
}

export function settingsSandboxDetailOptions(name: string, enabled = true) {
  return queryOptions({
    queryKey: settingsKeys.sandboxDetail(name),
    queryFn: ({ signal }) => getSettingsSandbox(name, signal),
    staleTime: COLLECTION_STALE_TIME,
    refetchInterval: COLLECTION_REFETCH_INTERVAL,
    enabled: Boolean(name) && enabled,
    retry: shouldRetrySettingsQuery,
  });
}

export function settingsHooksListOptions() {
  return queryOptions({
    queryKey: settingsKeys.hooksList(),
    queryFn: ({ signal }) => listSettingsHooks(signal),
    staleTime: COLLECTION_STALE_TIME,
    refetchInterval: COLLECTION_REFETCH_INTERVAL,
    retry: shouldRetrySettingsQuery,
  });
}

export function settingsMCPServersListOptions(
  filter: SettingsMCPServerListFilter = {},
  enabled = true,
  refetchInterval = COLLECTION_REFETCH_INTERVAL
) {
  return queryOptions({
    queryKey: settingsKeys.mcpList(filter),
    queryFn: ({ signal }) => listSettingsMCPServers(filter, signal),
    staleTime: COLLECTION_STALE_TIME,
    refetchInterval,
    enabled,
    retry: shouldRetrySettingsQuery,
  });
}

export function settingsNotificationPresetsOptions(filter: SettingsNotificationPresetFilter = {}) {
  return queryOptions({
    queryKey: settingsKeys.notificationPresetsList(filter),
    queryFn: ({ signal }) => listSettingsNotificationPresets(filter, signal),
    staleTime: COLLECTION_STALE_TIME,
    refetchInterval: COLLECTION_REFETCH_INTERVAL,
    retry: shouldRetrySettingsQuery,
  });
}

export function settingsRestartStatusOptions(operationId: string | null, enabled = true) {
  const active = Boolean(operationId) && enabled;

  return queryOptions({
    queryKey: settingsKeys.restartStatus(operationId ?? ""),
    queryFn: ({ signal }) => getSettingsRestartStatus(operationId ?? "", signal),
    enabled: active,
    staleTime: 0,
    refetchInterval: query =>
      isTerminalRestartStatus(query.state.data?.status) ? false : RESTART_POLL_INTERVAL,
    refetchIntervalInBackground: true,
    retry: shouldRetrySettingsQuery,
  });
}

export function settingsApplyRecordsOptions(filter: SettingsApplyRecordsFilter = {}) {
  return queryOptions({
    queryKey: settingsKeys.applyRecords(filter),
    queryFn: ({ signal }) => listSettingsApplyRecords(filter, signal),
    staleTime: APPLY_RECORDS_STALE_TIME,
    refetchInterval: APPLY_RECORDS_REFETCH_INTERVAL,
    retry: shouldRetrySettingsQuery,
  });
}

export const SETTINGS_QUERY_INTERVALS = {
  sectionStaleTime: SECTION_STALE_TIME,
  sectionRefetchInterval: SECTION_REFETCH_INTERVAL,
  collectionStaleTime: COLLECTION_STALE_TIME,
  collectionRefetchInterval: COLLECTION_REFETCH_INTERVAL,
  mcpAuthStatusPollInterval: MCP_AUTH_STATUS_POLL_INTERVAL,
  applyRecordsStaleTime: APPLY_RECORDS_STALE_TIME,
  applyRecordsRefetchInterval: APPLY_RECORDS_REFETCH_INTERVAL,
  restartPollInterval: RESTART_POLL_INTERVAL,
} as const;
