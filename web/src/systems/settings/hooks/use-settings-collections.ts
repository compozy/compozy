import { useQuery } from "@tanstack/react-query";

import {
  settingsSandboxDetailOptions,
  settingsSandboxesListOptions,
  settingsHooksListOptions,
  settingsMCPServersListOptions,
  settingsNotificationPresetsOptions,
  settingsProviderDetailOptions,
  settingsProvidersListOptions,
} from "../lib/query-options";
import type { SettingsMCPServerListFilter, SettingsNotificationPresetFilter } from "../types";

interface QueryEnabledOptions {
  enabled?: boolean;
}

interface MCPQueryOptions extends QueryEnabledOptions {
  refetchInterval?: number;
}

export function useSettingsProviders() {
  return useQuery(settingsProvidersListOptions());
}

export function useSettingsProvider(name: string, options: QueryEnabledOptions = {}) {
  return useQuery(settingsProviderDetailOptions(name, options.enabled ?? true));
}

export function useSettingsSandboxes() {
  return useQuery(settingsSandboxesListOptions());
}

export function useSettingsSandbox(name: string, options: QueryEnabledOptions = {}) {
  return useQuery(settingsSandboxDetailOptions(name, options.enabled ?? true));
}

export function useSettingsHooks() {
  return useQuery(settingsHooksListOptions());
}

export function useSettingsMCPServers(
  filter: SettingsMCPServerListFilter = {},
  options: MCPQueryOptions = {}
) {
  return useQuery(
    settingsMCPServersListOptions(filter, options.enabled ?? true, options.refetchInterval)
  );
}

export function useSettingsNotificationPresets(filter: SettingsNotificationPresetFilter = {}) {
  return useQuery(settingsNotificationPresetsOptions(filter));
}
