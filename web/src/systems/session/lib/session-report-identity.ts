import type { AgentAvatarAppearance } from "@compozy/ui";

import { agentAvatarAppearance, type AgentInstancePayload } from "@/systems/agent";

/** Who a report or request speaks as. An archived agent keeps a neutral face. */
export interface ReportAgent {
  label: string;
  appearance: AgentAvatarAppearance;
  provider?: string;
}

export function reportAgent(
  instance: AgentInstancePayload | undefined,
  fallbackName: string
): ReportAgent {
  return {
    label: instance?.label ?? fallbackName,
    appearance: agentAvatarAppearance(
      instance?.resolved_appearance,
      instance?.agent_name ?? fallbackName
    ),
    provider: instance?.provider,
  };
}
