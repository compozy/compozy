import type { QueryClient } from "@tanstack/react-query";

import { resolveActiveWorkspaceId, settleRouteQueries } from "./-route-preload";
import { accountStatusOptions } from "@/systems/account";
import { agentsListOptions } from "@/systems/agent";
import { computersListOptions, remoteStatusOptions } from "@/systems/computers";
import { readActingProfile } from "@/systems/profiles";
import {
  settingsApplyRecordsOptions,
  settingsGeneralOptions,
  settingsPersonaFilterForProfile,
  settingsPersonaOptions,
  settingsHooksExtensionsOptions,
  settingsMCPServersListOptions,
  settingsAttentionOptions,
  settingsAttentionFilterForProfile,
  settingsObservabilityOptions,
  settingsProvidersListOptions,
  settingsRolesOptions,
  settingsRolesStatusOptions,
  settingsSkillsOptions,
  settingsUpdateOptions,
} from "@/systems/settings";
import { workspacesListOptions } from "@/systems/workspace";

export async function preloadSettingsGeneralRoute(queryClient: QueryClient): Promise<void> {
  await Promise.all([
    resolveActiveWorkspaceId(queryClient),
    settleRouteQueries([
      queryClient.ensureQueryData(settingsGeneralOptions()),
      queryClient.ensureQueryData(
        settingsPersonaOptions(settingsPersonaFilterForProfile(readActingProfile(queryClient)))
      ),
      queryClient.ensureQueryData(settingsProvidersListOptions()),
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
    queryClient.ensureQueryData(agentsListOptions(undefined, readActingProfile(queryClient))),
    queryClient.ensureQueryData(workspacesListOptions()),
    queryClient.ensureQueryData(settingsSkillsOptions({ scope: "user" })),
  ]);
}

export function preloadSettingsRolesRoute(queryClient: QueryClient): Promise<void> {
  return settleRouteQueries([
    queryClient.ensureQueryData(settingsRolesStatusOptions()),
    queryClient.ensureQueryData(settingsRolesOptions()),
  ]);
}

export function preloadSettingsAttentionRoute(queryClient: QueryClient): Promise<void> {
  const profileName = readActingProfile(queryClient);
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
  return settleRouteQueries([queryClient.ensureQueryData(settingsHooksExtensionsOptions())]);
}

export function preloadSettingsExtensionsRoute(queryClient: QueryClient): Promise<void> {
  return settleRouteQueries([queryClient.ensureQueryData(settingsHooksExtensionsOptions())]);
}

export function preloadSettingsMCPRoute(queryClient: QueryClient): Promise<void> {
  const profile = readActingProfile(queryClient);
  return settleRouteQueries([
    queryClient.ensureQueryData(workspacesListOptions()),
    queryClient.ensureQueryData(
      settingsMCPServersListOptions(
        profile === "default" ? { scope: "user" } : { scope: "profile", profile }
      )
    ),
  ]);
}

export async function preloadSettingsComputersRoute(queryClient: QueryClient): Promise<void> {
  const account = await queryClient.ensureQueryData(accountStatusOptions()).catch(() => undefined);
  if (!account?.signed_in) return;
  await settleRouteQueries([
    queryClient.ensureQueryData(computersListOptions()),
    queryClient.ensureQueryData(remoteStatusOptions()),
  ]);
}

export function preloadSettingsAccountRoute(queryClient: QueryClient): Promise<void> {
  return settleRouteQueries([queryClient.ensureQueryData(accountStatusOptions())]);
}
