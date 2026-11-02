import { KindIcon, StateGlyph } from "@compozy/ui";

import { AgentIdentityMark } from "@/systems/agent/components/agent-identity-mark";

import { subagentDisplay } from "./subagent-format";
import type { SubagentView } from "./types";

/**
 * Who works, with the state riding the corner so a row's right edge stays free for time. A
 * provider-native subagent has no agent identity of its own: `runtime.agent` names its parent.
 */
export function SubagentTile({
  subagent,
  stale = false,
}: {
  subagent: SubagentView;
  stale?: boolean;
}) {
  const { agent, provider } = subagent.runtime;
  const display = subagentDisplay(subagent, { stale });
  return (
    <span className="relative inline-grid size-8 shrink-0 place-items-center rounded-lg bg-surface-2">
      {agent && subagent.origin === "delegated" ? (
        <AgentIdentityMark name={agent} quietIdle size={24} />
      ) : (
        <KindIcon kind={provider ?? undefined} size="sm" />
      )}
      <span className="absolute -right-1 -bottom-1 grid size-4 place-items-center rounded-full bg-elevated">
        <StateGlyph size="sm" state={display.glyph} still={stale} />
      </span>
    </span>
  );
}
