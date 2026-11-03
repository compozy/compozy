import { providerReviewer, useProviders } from "@/systems/providers";

import type { AgentPayload } from "../types";

/** The provider the agent resolves to, its display name and whether it declares a reviewer. */
export function usePermissionsProvider(agent: AgentPayload) {
  const providers = useProviders();
  const providerId = agent.provider || agent.effective_runtime?.provider || "";
  const summary = providers.data?.providers.find(provider => provider.name === providerId);
  return {
    providerId,
    providerName: summary?.display_name || providerId || "This provider",
    reviewer: providerReviewer(summary),
  };
}
