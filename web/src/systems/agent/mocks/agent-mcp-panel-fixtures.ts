import type { AgentPayload } from "../types";
import type { AgentMCPPickerRow } from "../lib/agent-mcp-picker-model";

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

/** Picker rows as `agentMCPPickerRows` derives them from the settings MCP list. */
export const MCP_PICKER_ROWS: AgentMCPPickerRow[] = [
  {
    name: "linear",
    transport: "remote",
    source: "Remote · Added by you",
    status: { kind: "needs-sign-in" },
    checked: true,
    saved: false,
  },
  {
    name: "sentry",
    transport: "remote",
    source: "Remote · Added by a file in this project",
    status: { kind: "tools", count: 18 },
    checked: true,
    saved: true,
  },
  {
    name: "github-local",
    transport: "local",
    source: "Runs on this computer · Added by a file in this project",
    status: { kind: "tools", count: 12 },
    checked: false,
    saved: false,
  },
  {
    name: "filesystem",
    transport: "local",
    source: "Runs on this computer · Added by you",
    status: { kind: "none" },
    checked: false,
    saved: false,
  },
];
