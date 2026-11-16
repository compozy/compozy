import { authorizeLabel, type SettingsMCPServerEntry } from "@/systems/settings";

import type { AgentPayload, UpdateAgentParams } from "../types";

export type ServerInput = NonNullable<UpdateAgentParams["agent"]["mcp_servers"]>[number];

export type AgentMCPPickerStatus =
  | { kind: "needs-sign-in" }
  | { kind: "tools"; count: number }
  | { kind: "none" };

export interface AgentMCPPickerRow {
  name: string;
  transport: "remote" | "local";
  source: string;
  status: AgentMCPPickerStatus;
  checked: boolean;
  /** Saved on the agent: the agent update only adds, so the picker cannot remove it. */
  saved: boolean;
}

/**
 * Agent references resolve against configured servers only, so extension-owned
 * servers are not attachable; staged agent-local definitions keep their name.
 */
export function attachableServers(
  entries: SettingsMCPServerEntry[],
  additions: ServerInput[]
): SettingsMCPServerEntry[] {
  const stagedDefinitions = new Set(additions.flatMap(a => (a.reference ? [] : [a.name])));
  return entries.filter(
    entry => (!entry.owner || entry.owner === "manual") && !stagedDefinitions.has(entry.name)
  );
}

/** Plain-words origin line built from the effective source the daemon reports. */
export function agentMCPSourceLabel(entry: SettingsMCPServerEntry): string {
  const where = entry.transport === "http" ? "Remote" : "Runs on this computer";
  const kind = entry.source_metadata.effective_source.kind;
  return kind.startsWith("workspace-")
    ? `${where} · Added by a file in this project`
    : `${where} · Added by you`;
}

function pickerStatus(entry: SettingsMCPServerEntry): AgentMCPPickerStatus {
  if (authorizeLabel(entry) !== null) return { kind: "needs-sign-in" };
  const count = entry.runtime_status?.tool_count ?? 0;
  return count > 0 ? { kind: "tools", count } : { kind: "none" };
}

/** Checked = saved on the agent, else the person's toggle, else the staged reference. */
export function agentMCPPickerRows({
  entries,
  agent,
  additions,
  overrides,
}: {
  entries: SettingsMCPServerEntry[];
  agent: AgentPayload;
  additions: ServerInput[];
  overrides: Record<string, boolean>;
}): AgentMCPPickerRow[] {
  const saved = new Set((agent.mcp_servers ?? []).map(server => server.name));
  const staged = new Set(additions.flatMap(a => (a.reference ? [a.name] : [])));
  return attachableServers(entries, additions).map(entry => {
    const isSaved = saved.has(entry.name);
    return {
      name: entry.name,
      transport: entry.transport === "http" ? "remote" : "local",
      source: agentMCPSourceLabel(entry),
      status: pickerStatus(entry),
      // Own keys only: a server named like an inherited property (`constructor`) has no toggle.
      checked:
        isSaved ||
        (Object.hasOwn(overrides, entry.name) ? overrides[entry.name]! : staged.has(entry.name)),
      saved: isSaved,
    };
  });
}

export function filterAgentMCPPickerRows(rows: AgentMCPPickerRow[], query: string) {
  const needle = query.trim().toLowerCase();
  if (!needle) return rows;
  return rows.filter(
    row => row.name.toLowerCase().includes(needle) || row.source.toLowerCase().includes(needle)
  );
}

/**
 * Applies the picker toggles to the staged additions: an unchecked staged
 * reference leaves staging, a newly checked server becomes a name reference,
 * and agent-local definitions are untouched.
 */
export function nextAgentMCPAdditions(
  additions: ServerInput[],
  overrides: Record<string, boolean>,
  saved: ReadonlySet<string>
): ServerInput[] {
  const kept = additions.filter(a => !(a.reference && overrides[a.name] === false));
  const present = new Set(kept.map(a => a.name));
  const added = Object.entries(overrides).flatMap(([name, checked]) =>
    checked && !present.has(name) && !saved.has(name) ? [{ name, reference: true }] : []
  );
  return [...kept, ...added];
}
