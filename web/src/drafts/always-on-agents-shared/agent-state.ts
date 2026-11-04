export type AgentState =
  | "working"
  | "needs-you"
  | "failed"
  | "paused"
  | "idle"
  | "starting"
  | "unknown";

export type AgentAppearance =
  | { avatar: "generated"; seed: string; color?: string }
  | { avatar: "icon"; icon: string; color: string }
  | { avatar: "emoji"; emoji: string; color: string };

export const AGENT_STATE_LABEL: Record<AgentState, string> = {
  working: "Working",
  "needs-you": "Needs you",
  failed: "Failed",
  paused: "Paused",
  idle: "Idle",
  starting: "Starting",
  unknown: "State unknown",
};
