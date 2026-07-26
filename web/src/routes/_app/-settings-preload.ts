import type { QueryClient } from "@tanstack/react-query";

import { agentsListOptions } from "@/systems/agent";
import { notificationPresetsOptions } from "@/systems/notifications";
import {
  settingsApplyRecordsOptions,
  settingsGeneralOptions,
  settingsHooksExtensionsOptions,
  settingsMemoryOptions,
  settingsObservabilityOptions,
  settingsProvidersListOptions,
  settingsRolesOptions,
  settingsRolesStatusOptions,
  settingsSandboxesListOptions,
  settingsSkillsOptions,
  settingsUpdateOptions,
} from "@/systems/settings";
import { workspacesListOptions } from "@/systems/workspace";

import { resolveActiveWorkspaceId, settleRouteQueries } from "./-route-preload";

export function preloadSandboxRoute(queryClient: QueryClient): Promise<void> {
  return settleRouteQueries([queryClient.ensureQueryData(settingsSandboxesListOptions())]);
}

export async function preloadSettingsGeneralRoute(queryClient: QueryClient): Promise<void> {
  await Promise.all([
    resolveActiveWorkspaceId(queryClient),
    settleRouteQueries([
      queryClient.ensureQueryData(settingsGeneralOptions()),
      queryClient.ensureQueryData(settingsUpdateOptions()),
      queryClient.ensureQueryData(settingsApplyRecordsOptions({ limit: 8 })),
    ]),
  ]);
}

export function preloadSettingsProvidersRoute(queryClient: QueryClient): Promise<void> {
  return settleRouteQueries([queryClient.ensureQueryData(settingsProvidersListOptions())]);
}

export function preloadSettingsSkillsRoute(queryClient: QueryClient): Promise<void> {
  return settleRouteQueries([
    queryClient.ensureQueryData(agentsListOptions()),
    queryClient.ensureQueryData(workspacesListOptions()),
    queryClient.ensureQueryData(settingsSkillsOptions({ scope: "global" })),
  ]);
}

export function preloadSettingsMemoryRoute(queryClient: QueryClient): Promise<void> {
  return settleRouteQueries([queryClient.ensureQueryData(settingsMemoryOptions())]);
}

export function preloadSettingsRolesRoute(queryClient: QueryClient): Promise<void> {
  return settleRouteQueries([
    queryClient.ensureQueryData(settingsRolesStatusOptions()),
    queryClient.ensureQueryData(settingsRolesOptions()),
  ]);
}

export function preloadSettingsObservabilityRoute(queryClient: QueryClient): Promise<void> {
  return settleRouteQueries([queryClient.ensureQueryData(settingsObservabilityOptions())]);
}

export function preloadSettingsHooksRoute(queryClient: QueryClient): Promise<void> {
  return settleRouteQueries([
    queryClient.ensureQueryData(settingsHooksExtensionsOptions()),
    queryClient.ensureQueryData(notificationPresetsOptions()),
  ]);
}

export function preloadSettingsExtensionsRoute(queryClient: QueryClient): Promise<void> {
  return settleRouteQueries([queryClient.ensureQueryData(settingsHooksExtensionsOptions())]);
}
