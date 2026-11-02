import type { AgentAppearancePayload, AgentPayload } from "../types";
import { agentAvatarAppearance } from "./agent-instance";

/** The props every `AgentMark` on an editor page shares. */
export function agentEditorMark(agent: AgentPayload) {
  return {
    appearance: agentAvatarAppearance(
      agent.resolved_appearance as unknown as AgentAppearancePayload,
      agent.name
    ),
    label: agent.label || agent.name,
  };
}
