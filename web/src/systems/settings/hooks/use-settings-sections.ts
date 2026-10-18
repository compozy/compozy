import { hashKey, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";

import { useWorkspaces, workspaceKeys, type WorkspacePayload } from "@/systems/workspace";

import {
  settingsAttentionOptions,
  settingsAutomationOptions,
  settingsApplyRecordsOptions,
  settingsGeneralOptions,
  settingsPersonaOptions,
  settingsHooksExtensionsOptions,
  settingsObservabilityOptions,
  settingsRolesOptions,
  settingsRolesStatusOptions,
  settingsShellOptions,
  settingsSkillsOptions,
  settingsUpdateOptions,
} from "../lib/query-options";
import type {
  SettingsApplyRecordsFilter,
  SettingsAttentionFilter,
  SettingsPersonaFilter,
  SettingsSkillsFilter,
} from "../types";

export function useSettingsGeneral() {
  return useQuery(settingsGeneralOptions());
}

export function useSettingsPersona(filter: SettingsPersonaFilter) {
  return useQuery(settingsPersonaOptions(filter));
}

export function useSettingsUpdate() {
  return useQuery(settingsUpdateOptions());
}

export function useSettingsApplyRecords(filter: SettingsApplyRecordsFilter = {}) {
  return useQuery(settingsApplyRecordsOptions(filter));
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

export function useSettingsAttention(filter: SettingsAttentionFilter) {
  const queryClient = useQueryClient();
  const query = useQuery(settingsAttentionOptions(filter));
  const { scope, profile } = filter;
  useWorkspaces();

  useEffect(() => {
    const workspaceKey = workspaceKeys.list();
    const workspaceHash = hashKey(workspaceKey);
    const attentionKey = settingsAttentionOptions({ scope, profile }).queryKey;
    const reconcile = () => {
      const workspaces = queryClient.getQueryData<WorkspacePayload[]>(workspaceKey);
      const mutes = queryClient.getQueryData(attentionKey)?.config.muted_workspaces;
      if (!workspaces || !mutes?.length) return;
      const registered = new Set(workspaces.map(workspace => workspace.id));
      if (mutes.some(id => !registered.has(id))) {
        void queryClient.invalidateQueries(
          { queryKey: attentionKey, exact: true },
          { cancelRefetch: false }
        );
      }
    };
    const unsubscribe = queryClient.getQueryCache().subscribe(event => {
      if (
        event.type === "updated" &&
        event.action.type === "success" &&
        event.query.queryHash === workspaceHash
      ) {
        reconcile();
      }
    });
    reconcile();
    return unsubscribe;
  }, [queryClient, scope, profile]);

  return query;
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
