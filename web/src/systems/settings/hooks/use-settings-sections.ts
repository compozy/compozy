import { useQuery } from "@tanstack/react-query";

import {
  settingsAttentionOptions,
  settingsAutomationOptions,
  settingsApplyRecordsOptions,
  settingsGeneralOptions,
  settingsHooksExtensionsOptions,
  settingsMemoryOptions,
  settingsNetworkOptions,
  settingsObservabilityOptions,
  settingsRolesOptions,
  settingsRolesStatusOptions,
  settingsShellOptions,
  settingsSkillsOptions,
  settingsUpdateOptions,
} from "../lib/query-options";
import type { SettingsApplyRecordsFilter, SettingsSkillsFilter } from "../types";

export function useSettingsGeneral() {
  return useQuery(settingsGeneralOptions());
}

export function useSettingsUpdate() {
  return useQuery(settingsUpdateOptions());
}

export function useSettingsApplyRecords(filter: SettingsApplyRecordsFilter = {}) {
  return useQuery(settingsApplyRecordsOptions(filter));
}

export function useSettingsMemory() {
  return useQuery(settingsMemoryOptions());
}

export function useRolesStatus() {
  return useQuery(settingsRolesStatusOptions());
}

export function useSettingsRoles() {
  return useQuery(settingsRolesOptions());
}

export function useSettingsSkills(filter: SettingsSkillsFilter = {}) {
  return useQuery(settingsSkillsOptions(filter));
}

export function useSettingsAutomation() {
  return useQuery(settingsAutomationOptions());
}

export function useSettingsNetwork() {
  return useQuery(settingsNetworkOptions());
}

export function useSettingsAttention() {
  return useQuery(settingsAttentionOptions());
}

export function useSettingsShell() {
  return useQuery(settingsShellOptions());
}

export function useSettingsObservability() {
  return useQuery(settingsObservabilityOptions());
}

export function useSettingsHooksExtensions() {
  return useQuery(settingsHooksExtensionsOptions());
}
