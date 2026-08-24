import type { QueryClient } from "@tanstack/react-query";

import { resolveActiveWorkspaceId, settleRouteQueries } from "./-route-preload";
import { agentsListOptions } from "@/systems/agent";
import { notificationPresetsOptions } from "@/systems/notifications";
import { actingProfile, readProfileLens, readProfileView } from "@/systems/profiles";
import {
  settingsApplyRecordsOptions,
  settingsGeneralOptions,
  settingsPersonaFilterForProfile,
  settingsPersonaOptions,
  settingsHooksExtensionsOptions,
  settingsMemoryOptions,
  settingsAttentionOptions,
  settingsAttentionFilterForProfile,
  settingsObservabilityOptions,
  settingsProvidersListOptions,
  settingsRolesOptions,
  settingsRolesStatusOptions,
  settingsSandboxesListOptions,
  settingsSkillsOptions,
  settingsUpdateOptions,
} from "@/systems/settings";
import { workspacesListOptions } from "@/systems/workspace";

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

export function preloadSettingsDefaultsRoute(queryClient: QueryClient): Promise<void> {
  const profileName = actingProfile(readProfileView(queryClient, readProfileLens()));
  return settleRouteQueries([
    queryClient.ensureQueryData(
      settingsPersonaOptions(settingsPersonaFilterForProfile(profileName))
    ),
    queryClient.ensureQueryData(settingsProvidersListOptions()),
    queryClient.ensureQueryData(settingsSandboxesListOptions()),
  ]);
}

export function preloadSettingsSkillsRoute(queryClient: QueryClient): Promise<void> {
  return settleRouteQueries([
    queryClient.ensureQueryData(agentsListOptions()),
    queryClient.ensureQueryData(workspacesListOptions()),
    queryClient.ensureQueryData(settingsSkillsOptions({ scope: "user" })),
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

export function preloadSettingsAttentionRoute(queryClient: QueryClient): Promise<void> {
  const profileName = actingProfile(readProfileView(queryClient, readProfileLens()));
  return settleRouteQueries([
    queryClient.ensureQueryData(
      settingsAttentionOptions(settingsAttentionFilterForProfile(profileName))
    ),
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
