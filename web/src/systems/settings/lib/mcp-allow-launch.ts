import { formatCommandLine } from "./mcp-editor-paste";

/** What a server waiting for approval would run (a local command) or reach (a remote address). */
export interface MCPAllowPanelLaunch {
  kind: "command" | "url";
  value: string;
}

/**
 * The launch line a server waiting for approval would run, from its definition. Arguments with
 * spaces or quotes are quoted so the line reads back as the exact words the server runs.
 */
export function mcpAllowPanelLaunch(entry: {
  command?: string | null;
  args?: readonly string[] | null;
  url?: string | null;
}): MCPAllowPanelLaunch | null {
  if (entry.command?.trim()) {
    return { kind: "command", value: formatCommandLine(entry.command, entry.args ?? []) };
  }
  if (entry.url?.trim()) return { kind: "url", value: entry.url };
  return null;
}
