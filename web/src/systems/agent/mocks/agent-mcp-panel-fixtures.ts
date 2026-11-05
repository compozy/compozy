import type { AgentPayload } from "../types";

export const MCP_PANEL_AGENT: AgentPayload = {
  name: "ops-assistant",
  label: "Ops Assistant",
  kind: "on_demand",
  provider: "codex",
  prompt: "Help organize project work.",
  origin: "global",
  definition_digest: "d".repeat(64),
  resolved_appearance: {
    version: 1,
    avatar: "generated",
    seed: "ops-assistant",
    generation: 1,
    color: "#7c5cff",
  },
  mcp_servers: [],
};
