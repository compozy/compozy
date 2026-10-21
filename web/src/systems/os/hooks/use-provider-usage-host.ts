import { useGatewayAccessTier } from "@/systems/gateway";
import { useProfileReadScope } from "@/systems/profiles";
import { reportsKeychainConsent, useProviderUsage } from "@/systems/provider-usage";

import { desktopShellBridge } from "../lib/desktop-shell-bridge";

export interface ProviderUsageHost {
  /** The daemon runs on macOS, where Claude Keychain consent exists. */
  isMacHost: boolean;
  /** This client reaches the daemon through the gateway, not on its own host. */
  remoteClient: boolean;
}

/**
 * Where the daemon runs, from evidence about the daemon rather than the
 * browser: the daemon reports Keychain consent states only on macOS, and the
 * local listener is reachable only from its own machine, so a macOS desktop
 * app attached to it shares the daemon's host. Reads the active profile's
 * cached envelope (the same query the usage surfaces hold).
 */
export function useProviderUsageHost(): ProviderUsageHost {
  const { viewModel } = useProviderUsage(useProfileReadScope().destination);
  const tier = useGatewayAccessTier();
  const remoteClient = tier === "private" || tier === "public";
  const sameMacHost = tier === "local" && desktopShellBridge()?.platform === "darwin";
  return { isMacHost: sameMacHost || reportsKeychainConsent(viewModel), remoteClient };
}
