import { isHostedBrowserMode } from "@/lib/hosted-browser-auth";
import type { ProviderUsageViewModel } from "@/systems/provider-usage";

export interface ProviderUsageHost {
  /** The daemon runs on macOS, where Claude Keychain consent exists. */
  isMacHost: boolean;
  /** This client reaches the daemon through a hosted route. */
  remoteClient: boolean;
}

/**
 * Where the daemon runs, as the daemon reports it (`keychain_supported`), and
 * whether this client reaches it remotely through a hosted route.
 */
export function useProviderUsageHost(viewModel: ProviderUsageViewModel | null): ProviderUsageHost {
  return {
    isMacHost: viewModel?.keychainSupported ?? false,
    remoteClient: isHostedBrowserMode(),
  };
}
