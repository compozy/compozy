// One live agent face: the product's own AgentAvatar (Blobatar pose, state mark, harness badge)
// for a sample roster agent, so the manual shows the shipping component, not a redraw (ADR-005).
import { AgentAvatar } from "@compozy/ui/components/custom/agent-avatar";

import { agentByName, harnessLabel, type AgentState } from "./roster";

interface AgentFaceProps {
  name: string;
  state?: AgentState;
  /** px, 16 to 112 (AgentAvatar bounds) */
  size?: number;
  /** harness badge in the corner (from 24 px) */
  chip?: boolean;
  className?: string;
}

export function AgentFace({
  name,
  state = "idle",
  size = 40,
  chip = true,
  className,
}: AgentFaceProps) {
  const a = agentByName(name);
  return (
    <AgentAvatar
      appearance={{ avatar: "generated", seed: a.name }}
      label={`${a.name} · ${harnessLabel[a.harness]}`}
      state={state}
      provider={chip ? a.harness : null}
      size={size}
      className={className}
    />
  );
}
