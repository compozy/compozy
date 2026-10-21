import { useProfileReadScope } from "@/systems/profiles";
import {
  useProviderUsage,
  useProviderUsageActions,
  useProviderUsageOpenRefresh,
  useProviderUsageStream,
  type ProviderUsageActions,
  type ProviderUsageState,
} from "@/systems/provider-usage";
import { useAllowProviderUsageKeychain } from "@/systems/settings";

import { useOsAppLauncher } from "./use-os-app-launcher";
import { useProviderUsageHost, type ProviderUsageHost } from "./use-provider-usage-host";

export interface ProviderUsageSurfaceModel extends ProviderUsageHost {
  profile: string;
  usage: ProviderUsageState;
  actions: ProviderUsageActions;
  /** Popover open: a background refresh for rows older than 60 s. */
  openRefresh: () => void;
  openUsage: () => void;
  openSettings: () => void;
}

/**
 * The provider usage wiring the tray popover and the Usage app share: the
 * shell's active profile, its live envelope, the operator actions, and the
 * OS-owned destinations (Usage window, Provider usage settings). The stream
 * lease keeps one connection per profile however many surfaces mount this.
 */
export function useProviderUsageSurface(): ProviderUsageSurfaceModel {
  const profile = useProfileReadScope().destination;
  const usage = useProviderUsage(profile);
  useProviderUsageStream(profile);
  const host = useProviderUsageHost();
  const launcher = useOsAppLauncher();
  const openSettings = () => launcher.openSettingsSection("provider-usage");
  const allowKeychain = useAllowProviderUsageKeychain();
  const actions = useProviderUsageActions(profile, {
    allowKeychain,
    configure: openSettings,
  });
  const openRefresh = useProviderUsageOpenRefresh(profile);
  return {
    ...host,
    profile,
    usage,
    actions,
    openRefresh,
    openUsage: launcher.openUsage,
    openSettings,
  };
}
