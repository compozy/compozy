export type AgentAvatarState =
  | "working"
  | "needs-you"
  | "failed"
  | "paused"
  | "idle"
  | "starting"
  | "unknown";

export type AgentAvatarAppearance =
  | { avatar: "generated"; seed: string; generation?: number; color?: string }
  | { avatar: "icon"; icon: string; color?: string }
  | { avatar: "emoji"; emoji: string; color?: string };

export const AGENT_AVATAR_STATE_LABEL: Record<AgentAvatarState, string> = {
  working: "Working",
  "needs-you": "Needs you",
  failed: "Failed",
  paused: "Paused",
  idle: "Idle",
  starting: "Starting",
  unknown: "State unknown",
};
