// Sample roster for the live agent faces the manual renders from @compozy/ui (ADR-005): the
// agents and harnesses of the product's demo world (Northstar Pay). Samples, labelled as such.
import type { AgentAvatarState } from "@compozy/ui/lib/agent-avatar";

export type AgentState = Extract<
  AgentAvatarState,
  "working" | "needs-you" | "failed" | "paused" | "idle"
>;
export type Harness = "claude" | "codex" | "gemini" | "opencode" | "cursor";

export interface Agent {
  name: string;
  role: string;
  harness: Harness;
}

export const harnessLabel: Record<Harness, string> = {
  claude: "Claude Code",
  codex: "Codex",
  gemini: "Gemini CLI",
  opencode: "OpenCode",
  cursor: "Cursor",
};

export const roster: Agent[] = [
  { name: "checkout-lead", role: "Owns the checkout flow", harness: "claude" },
  { name: "qa-runner", role: "Runs canaries after deploy", harness: "codex" },
  { name: "docs-steward", role: "Keeps the API docs true", harness: "gemini" },
  { name: "release-manager", role: "Cuts and ships releases", harness: "claude" },
  { name: "payments-codex", role: "Payment provider work", harness: "codex" },
  { name: "test-writer", role: "Writes the missing tests", harness: "opencode" },
  { name: "platform-engineer", role: "Infra and CI", harness: "claude" },
];

export const agentByName = (n: string): Agent => roster.find(a => a.name === n) ?? roster[0]!;
