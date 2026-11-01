import type { AgentAvatarAppearance, AgentAvatarState } from "@compozy/ui";

import type {
  AgentAppearancePayload,
  AgentInstancePauseReason,
  AgentInstancePayload,
} from "../types";

export const GLOBAL_SCOPE = "@user";

/** What the daemon resolves for an agent that saved no appearance. */
const DERIVED_APPEARANCE_COLOR = "#7c5cff";

export function agentScopeLabel(scope: string) {
  return scope === GLOBAL_SCOPE ? "Global" : scope;
}

const PAUSE_COPY: Record<Exclude<AgentInstancePauseReason, "" | "fatigue">, string> = {
  manual: "Paused by you.",
  definition_not_stays_on: "Paused because this agent no longer stays on.",
  definition_missing: "Paused because this agent's definition is gone. Its memory is kept.",
};

/** Why it's paused, with the limit the daemon applied. Empty when it isn't paused. */
export function agentPauseCopy({
  pause_reason: reason,
  fatigue_limit: limit,
}: Pick<AgentInstancePayload, "pause_reason" | "fatigue_limit">) {
  if (!reason) return "";
  return reason === "fatigue"
    ? `Paused after ${limit} failed checks in a row.`
    : PAUSE_COPY[reason];
}

/** Keeps a link in the instance's own scope, whatever window it opens from. */
export function agentInstanceBinding(instance: Pick<AgentInstancePayload, "workspace_id">) {
  return { workspace: instance.workspace_id || GLOBAL_SCOPE };
}

/**
 * Where a focused editor reads a definition: the window's project, else the first
 * project the agent works in. Authored files need a registered project, so a window
 * under All projects or User borrows the agent's own placement.
 */
export function agentEditorWorkspace(
  workspace: string | null,
  agentName: string,
  instances: readonly Pick<AgentInstancePayload, "agent_name" | "workspace_id">[]
) {
  if (workspace && workspace !== GLOBAL_SCOPE) return workspace;
  const placed = instances.find(item => item.agent_name === agentName && item.workspace_id);
  return placed?.workspace_id ?? workspace;
}

/** These reasons resume only once the definition is back, so Resume stays disabled. */
export function agentInstanceResumeBlocked(instance: AgentInstancePayload) {
  return (
    instance.pause_reason === "definition_missing" ||
    instance.pause_reason === "definition_not_stays_on"
  );
}

/**
 * The avatar's state. Attention leads, because a paused agent can still hold a failure report.
 * `waiting` means it's waiting on the operator, so it reads the same as needs-you.
 */
export function agentInstanceAvatarState(instance: AgentInstancePayload): AgentAvatarState {
  if (instance.needs_attention) return "needs-you";
  if (instance.state === "paused") return "paused";
  if (instance.state === "starting") return "starting";
  switch (instance.activity) {
    case "working":
      return "working";
    case "waiting":
      return "needs-you";
    case "failed":
      return "failed";
    case "idle":
      return "idle";
    default:
      return "unknown";
  }
}

const URGENCY: Record<AgentAvatarState, number> = {
  "needs-you": 0,
  failed: 1,
  working: 2,
  starting: 3,
  idle: 4,
  unknown: 5,
  paused: 6,
};

/** Anyone who needs the operator leads the row; ties keep the daemon's order. */
export function agentInstancesByUrgency(instances: readonly AgentInstancePayload[]) {
  return [...instances].sort(
    (a, b) => URGENCY[agentInstanceAvatarState(a)] - URGENCY[agentInstanceAvatarState(b)]
  );
}

/** Whether an instance holds a slot of `max_active_instances`. */
export function agentInstanceHoldsSlot(instance: Pick<AgentInstancePayload, "state">) {
  return instance.state === "active" || instance.state === "starting";
}

/** The destination profile's slots, as the daemon counts them. */
export interface AgentInstanceCapacity {
  limit: number;
  active: number;
  reached: boolean;
}

/** Nothing is blocked until the daemon has said so; its 429 stays the authority. */
export function agentInstanceCapacity(
  count: { limit: number; active: number } | undefined
): AgentInstanceCapacity {
  const { limit, active } = count ?? { limit: 0, active: 0 };
  return { limit, active, reached: limit > 0 && active >= limit };
}

export function agentLimitLine({ active, limit }: AgentInstanceCapacity) {
  return `${active} of ${limit} agents are staying on`;
}

/**
 * The wire block as `AgentAvatar` reads it. An agent that saved no appearance resolves to the
 * daemon's one fallback color; dropping it lets the seed pick the hue, so unsaved agents differ.
 */
export function agentAvatarAppearance(
  appearance: AgentAppearancePayload | null | undefined,
  name: string
): AgentAvatarAppearance {
  const color = appearance?.color;
  if (appearance?.avatar === "icon" && appearance.icon) {
    return { avatar: "icon", icon: appearance.icon, color };
  }
  if (appearance?.avatar === "emoji" && appearance.emoji) {
    return { avatar: "emoji", emoji: appearance.emoji, color };
  }
  const seed = appearance?.seed || name;
  const derived = seed === name && color === DERIVED_APPEARANCE_COLOR;
  return {
    avatar: "generated",
    seed,
    generation: appearance?.generation,
    ...(color && !derived ? { color } : {}),
  };
}
