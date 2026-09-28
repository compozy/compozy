import { useQuery } from "@tanstack/react-query";

import {
  settingsHooksListOptions,
  settingsMCPServersListOptions,
  settingsMCPServerDetailOptions,
  settingsProviderDetailOptions,
  settingsProvidersListOptions,
} from "../lib/query-options";
import type {
  SettingsHookListFilter,
  SettingsMCPServerListFilter,
  SettingsMCPServerGetFilter,
} from "../types";

interface QueryEnabledOptions {
  enabled?: boolean;
}

interface MCPQueryOptions extends QueryEnabledOptions {
  refetchInterval?: number;
}

export function useSettingsProviders(options: QueryEnabledOptions = {}) {
  return useQuery(settingsProvidersListOptions(options.enabled ?? true));
}

export function useSettingsProvider(name: string, options: QueryEnabledOptions = {}) {
  return useQuery(settingsProviderDetailOptions(name, options.enabled ?? true));
}

export function useSettingsHooks(filter: SettingsHookListFilter = {}) {
  return useQuery(settingsHooksListOptions(filter));
}

export function useSettingsMCPServer(
  name: string,
  filter: SettingsMCPServerGetFilter = {},
  options: MCPQueryOptions = {}
) {
  return useQuery(
    settingsMCPServerDetailOptions(name, filter, options.enabled ?? true, options.refetchInterval)
  );
}

export function useSettingsMCPServers(
  filter: SettingsMCPServerListFilter = {},
  options: MCPQueryOptions = {}
) {
  return useQuery(
    settingsMCPServersListOptions(filter, options.enabled ?? true, options.refetchInterval)
  );
}
