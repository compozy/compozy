import { useGatewayAccessTier } from "@/systems/gateway";
import type { ProviderUsageViewModel } from "@/systems/provider-usage";

export interface ProviderUsageHost {
  /** The daemon runs on macOS, where Claude Keychain consent exists. */
  isMacHost: boolean;
  /** This client reaches the daemon through the gateway, not on its own host. */
  remoteClient: boolean;
}

/**
 * Where the daemon runs, as the daemon reports it (`keychain_supported`), and
 * whether this client reaches it remotely (the gateway listener tier).
 */
export function useProviderUsageHost(viewModel: ProviderUsageViewModel | null): ProviderUsageHost {
  const tier = useGatewayAccessTier();
  return {
    isMacHost: viewModel?.keychainSupported ?? false,
    remoteClient: tier === "private" || tier === "public",
  };
}
