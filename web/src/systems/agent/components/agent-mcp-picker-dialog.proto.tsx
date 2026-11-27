import { definePrototype, type PrototypeProps } from "@protohq/cli";
import { MCP_PICKER_ROWS } from "../mocks/agent-mcp-panel-fixtures";
import { AgentMCPPickerDialog } from "./agent-mcp-picker-dialog";

export const meta = definePrototype({
  title: "Agent MCP picker",
  route: "/agents/ops-assistant",
  states: [
    { id: "picked", label: "Saved and staged servers" },
    { id: "no-match", label: "Search with no match" },
    { id: "empty", label: "No servers yet" },
    { id: "loading", label: "Loading" },
  ],
});

export default function Prototype({ state }: PrototypeProps) {
  const rows = state === "empty" || state === "loading" ? [] : MCP_PICKER_ROWS;
  const query = state === "no-match" ? "stripe" : "";
  return (
    <AgentMCPPickerDialog
      agentLabel="Ops Assistant"
      query={query}
      onQueryChange={() => {}}
      rows={rows}
      visibleRows={query ? [] : rows}
      checkedCount={rows.filter(row => row.checked).length}
      loading={state === "loading"}
      error={null}
      onToggleServer={() => {}}
      onNewServer={() => {}}
      onSave={() => {}}
      onClose={() => {}}
    />
  );
}
