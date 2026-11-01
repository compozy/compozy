import type { AgentAvatarAppearance, AgentAvatarState } from "@compozy/ui";

import type {
  AgentAppearancePayload,
  AgentInstancePauseReason,
  AgentInstancePayload,
} from "../types";

export const GLOBAL_SCOPE = "@user";

/** `agents.stays_on.max_active_instances`' default; the daemon's 429 stays the authority. */
export const ACTIVE_INSTANCE_LIMIT = 10;

/** What the daemon resolves for an agent that saved no appearance. */
const DERIVED_APPEARANCE_COLOR = "#7c5cff";

export function agentScopeLabel(scope: string) {
  return scope === GLOBAL_SCOPE ? "Global" : scope;
}

export const AGENT_PAUSE_COPY: Record<Exclude<AgentInstancePauseReason, "">, string> = {
  manual: "Paused by you.",
  fatigue: "Paused after 10 failed checks in a row.",
  definition_not_stays_on: "Paused because this agent no longer stays on.",
  definition_missing: "Paused because this agent's definition is gone. Its memory is kept.",
};

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
export function agentInstanceHoldsSlot(instance: AgentInstancePayload) {
  return instance.state === "active" || instance.state === "starting";
}

export function agentInstanceLimitReached(instances: readonly AgentInstancePayload[]) {
  return instances.filter(agentInstanceHoldsSlot).length >= ACTIVE_INSTANCE_LIMIT;
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
