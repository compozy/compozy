import type { SessionProviderOption } from "../types";
import type { RuntimeProviderOption } from "@/systems/runtime";

/** Adapt one workspace provider into the shared runtime-selector contract. */
export function workspaceProviderToOption(provider: SessionProviderOption): RuntimeProviderOption {
  const displayName = provider.display_name?.trim();
  const harness = provider.harness?.trim();
  const runtimeProvider = provider.runtime_provider?.trim();
  return {
    id: provider.name,
    name: displayName || provider.name,
    ...(harness ? { harness } : {}),
    runtime_provider: runtimeProvider || provider.name,
    runtime_strategy: provider.runtime_strategy,
  };
}
