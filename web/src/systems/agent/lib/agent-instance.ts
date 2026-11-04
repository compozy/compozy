import type { AgentAvatarAppearance, AgentAvatarState } from "@compozy/ui";

import { formatRelativeClock } from "./agent-time";
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

type CountedReason = "" | "fatigue" | "unattended";

const PAUSE_COPY: Record<Exclude<AgentInstancePauseReason, CountedReason>, string> = {
  manual: "Paused by you.",
  definition_not_always_on: "Paused because this agent is no longer always-on.",
  definition_missing: "Paused because this agent's definition is gone. Its memory is kept.",
};

export type AgentPauseFacts = Pick<
  AgentInstancePayload,
  "pause_reason" | "fatigue_limit" | "unattended_turn_limit" | "unattended_turns"
>;

/** The steps the brake counted: the limit the daemon applied, else the count it reached. */
export function agentUnattendedSteps({
  unattended_turn_limit: limit,
  unattended_turns: turns,
}: AgentPauseFacts) {
  return limit || turns || 0;
}

/** Why it's paused, with the limit the daemon applied. Empty when it isn't paused. */
export function agentPauseCopy(instance: AgentPauseFacts) {
  const reason = instance.pause_reason;
  if (!reason) return "";
  if (reason === "fatigue") return `Paused after ${instance.fatigue_limit} failed checks in a row.`;
  if (reason === "unattended") {
    const steps = agentUnattendedSteps(instance);
    return steps
      ? `Paused after ${steps} steps without you.`
      : "Paused after many steps without you.";
  }
  return PAUSE_COPY[reason];
}

const WAKE_KIND: Record<string, string> = {
  check: "Its next heartbeat",
  timer: "A timer it set",
  terminal: "A terminal it's watching",
  pull_request: "A pull request it's watching",
  session: "A conversation it's waiting on",
};

/** What wakes it next and when, as one line; null when nothing is scheduled. */
export function agentNextWakeLine(
  instance: Pick<AgentInstancePayload, "next_wake">,
  now = Date.now()
) {
  const wake = instance.next_wake;
  if (!wake) return null;
  const what = wake.description.trim() || WAKE_KIND[wake.kind] || "Something it's waiting on";
  const when = formatRelativeClock(wake.at, now);
  return when ? `${what}, ${when}` : what;
}

/** Keeps a link in the instance's own scope, whatever window it opens from. */
export function agentInstanceBinding(
  instance: Pick<AgentInstancePayload, "workspace_id" | "profile_name">
) {
  return {
    workspace: instance.workspace_id || GLOBAL_SCOPE,
    ...(instance.profile_name ? { profile: instance.profile_name } : {}),
  };
}

/** Paused by the brake, so Resume is the operator's to give. */
export function agentInstancePausedUnattended(
  instance: Pick<AgentInstancePayload, "state" | "pause_reason"> | undefined
) {
  return instance?.state === "paused" && instance.pause_reason === "unattended";
}

/** These reasons resume only once the definition is back, so Resume stays disabled. */
export function agentInstanceResumeBlocked(instance: AgentInstancePayload) {
  return (
    instance.pause_reason === "definition_missing" ||
    instance.pause_reason === "definition_not_always_on"
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
  return `${active} of ${limit} always-on agents are active`;
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
  // Only `none` changes the mark; any other value keeps the plate.
  const background = appearance?.background === "none" ? { background: "none" as const } : {};
  if (appearance?.avatar === "icon" && appearance.icon) {
    return { avatar: "icon", icon: appearance.icon, color, ...background };
  }
  if (appearance?.avatar === "emoji" && appearance.emoji) {
    return { avatar: "emoji", emoji: appearance.emoji, color, ...background };
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
