// Live product pieces for the chapters, composed ONLY from @compozy/ui exports (ADR-005 §2,
// US-004.AC-3): agent faces, the state glyph row and the approval row. No hand-drawn redraws, no
// hard-coded colour; the product's own components carry every mark.
//   <AgentFace name="qa-runner" />                 roster agent, its state as the expression
//   <AgentFace name="qa-runner" state="paused" />  override the state
//   <StateRow />                                   working / needs-you / idle / paused / failed
//   <ApprovalRow />                                the permission dock, captured from web/
import {
  AGENT_AVATAR_STATE_LABEL,
  AgentAvatar,
  StateGlyph,
  type StateGlyphState,
} from "@compozy/ui";
import { cn } from "@compozy/ui/utils";

import { productShotSrc, type ShotTheme } from "./product-pins";
import { AGENT_STATES, agentByName, harnessLabel, type AgentState } from "./roster";

interface AgentFaceProps {
  name: string;
  state?: AgentState;
  size?: number;
  /** show name and role beside the face */
  withLabel?: boolean;
  className?: string;
}

export function AgentFace({
  name,
  state,
  size = 40,
  withLabel = false,
  className,
}: AgentFaceProps) {
  const agent = agentByName(name);
  const current = state ?? agent.state;
  const face = (
    <AgentAvatar
      appearance={{ avatar: "generated", seed: agent.name }}
      label={agent.name}
      scope={harnessLabel[agent.harness]}
      state={current}
      provider={agent.harness}
      size={size}
    />
  );
  if (!withLabel) return face;
  return (
    <span data-agent-face={name} className={cn("inline-flex items-center gap-3", className)}>
      {face}
      <span className="min-w-0">
        <span className="block truncate text-[14px] font-medium text-(--b-fg)">{agent.name}</span>
        <span className="block truncate text-[12.5px] text-(--b-muted)">{agent.role}</span>
      </span>
    </span>
  );
}

/** Agent state → the product's status glyph for that state (_uiux.md signal & state mapping). */
export const STATE_GLYPH: Record<AgentState, StateGlyphState> = {
  working: "running",
  "needs-you": "attention",
  idle: "idle",
  paused: "stopped",
  failed: "failed",
};

/** One row per documented state: glyph, face and label, all from @compozy/ui. */
export function StateRow({ className, faces = true }: { className?: string; faces?: boolean }) {
  return (
    <ul data-state-row className={cn("m-0 grid list-none gap-2 p-0", className)}>
      {AGENT_STATES.map(state => (
        <li
          key={state}
          data-agent-state={state}
          className="flex items-center gap-3 rounded-[10px] border border-(--b-line) bg-(--b-card) px-3 py-2"
        >
          <StateGlyph state={STATE_GLYPH[state]} still />
          {faces && (
            <AgentAvatar
              appearance={{ avatar: "generated", seed: `state-${state}` }}
              label={`Sample agent`}
              state={state}
              size={28}
              quietIdle
            />
          )}
          <span className="text-[14px] text-(--b-fg)">{AGENT_AVATAR_STATE_LABEL[state]}</span>
        </li>
      ))}
    </ul>
  );
}

/**
 * The permission dock is a web/ domain composite (not exported by @compozy/ui), so the approval
 * row is its pinned capture (ADR-005: no redraws; packages/brand never imports web/).
 */
export function ApprovalRow({
  theme = "dark",
  className,
}: {
  theme?: ShotTheme;
  className?: string;
}) {
  return (
    <img
      data-approval-row
      src={productShotSrc("approval", theme)}
      alt="Permission dock: allow once, always allow, don't allow"
      decoding="async"
      loading="lazy"
      className={cn("block h-auto w-full rounded-card", className)}
    />
  );
}
