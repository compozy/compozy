import { definePrototype, type PrototypeProps } from "@compozy/proto";
import { MCP_PANEL_AGENT } from "../mocks/agent-mcp-panel-fixtures";
import { AgentMcpServersPanel } from "./agent-mcp-servers-panel";

export const meta = definePrototype({
  title: "Agent MCP servers",
  route: "/agents/ops-assistant",
  states: [
    { id: "empty", label: "Add a server" },
    { id: "attached", label: "Configured reference" },
    { id: "broken", label: "Broken reference" },
    { id: "read-only", label: "Package-owned agent" },
  ],
});

export default function Prototype({ state }: PrototypeProps) {
  return (
    <AgentMcpServersPanel
      agent={{
        ...MCP_PANEL_AGENT,
        mcp_servers:
          state === "empty"
            ? []
            : [{ name: "team-docs", reference: true, broken_reference: state === "broken" }],
      }}
      readOnly={state === "read-only"}
      onAdd={() => {}}
    />
  );
}
