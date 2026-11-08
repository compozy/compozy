// Sample roster for the manual's live pieces: the agents, harnesses and states the product shows.
// Names follow the product's demo world (Northstar Pay); samples, labelled as such where shown.
// Faces render through @compozy/ui AgentAvatar with seed = agent name, the product's generator.
import type { AgentAvatarState } from "@compozy/ui";

/** The five agent states the manual documents (product AgentAvatarState minus starting/unknown). */
export type AgentState = Extract<
  AgentAvatarState,
  "working" | "needs-you" | "idle" | "paused" | "failed"
>;
/** Provider kinds as the product's kind-icon registry names them. */
export type Harness = "claude" | "codex" | "gemini" | "opencode" | "cursor";

export interface Agent {
  name: string;
  role: string;
  harness: Harness;
  state: AgentState;
}

export const AGENT_STATES: readonly AgentState[] = [
  "working",
  "needs-you",
  "idle",
  "paused",
  "failed",
];

export const harnessLabel: Record<Harness, string> = {
  claude: "Claude Code",
  codex: "Codex",
  gemini: "Gemini CLI",
  opencode: "OpenCode",
  cursor: "Cursor",
};

export const roster: readonly Agent[] = [
  { name: "checkout-lead", role: "Owns the checkout flow", harness: "claude", state: "working" },
  { name: "qa-runner", role: "Runs canaries after deploy", harness: "codex", state: "needs-you" },
  { name: "docs-steward", role: "Keeps the API docs true", harness: "gemini", state: "idle" },
  { name: "release-manager", role: "Cuts and ships releases", harness: "claude", state: "paused" },
  { name: "payments-codex", role: "Payment provider work", harness: "codex", state: "failed" },
  { name: "test-writer", role: "Writes the missing tests", harness: "opencode", state: "working" },
  { name: "platform-engineer", role: "Infra and CI", harness: "claude", state: "idle" },
];

export const agentByName = (name: string): Agent => {
  const a = roster.find(x => x.name === name);
  if (!a) throw new Error(`unknown roster agent ${name}`);
  return a;
};
