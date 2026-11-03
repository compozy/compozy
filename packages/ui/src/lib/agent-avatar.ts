export type AgentAvatarState =
  | "working"
  | "needs-you"
  | "failed"
  | "paused"
  | "idle"
  | "starting"
  | "unknown";

/**
 * Whether a symbol sits on its tinted plate (`plate`, the default when absent) or stands alone
 * with the color as its ink (`none`).
 */
export type AgentAvatarBackground = "plate" | "none";

export type AgentAvatarAppearance =
  | { avatar: "generated"; seed: string; generation?: number; color?: string }
  | { avatar: "icon"; icon: string; color?: string; background?: AgentAvatarBackground }
  | { avatar: "emoji"; emoji: string; color?: string; background?: AgentAvatarBackground };

export const AGENT_AVATAR_STATE_LABEL: Record<AgentAvatarState, string> = {
  working: "Working",
  "needs-you": "Needs you",
  failed: "Failed",
  paused: "Paused",
  idle: "Idle",
  starting: "Starting",
  unknown: "State unknown",
};
