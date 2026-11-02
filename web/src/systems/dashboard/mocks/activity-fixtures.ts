import type { PermissionEntry, PermissionRollupRow } from "../lib/activity-view";

export const ACTIVITY_NOW = "2026-11-01T12:00:00.000Z";

const AT = (hoursAgo: number) =>
  new Date(Date.parse(ACTIVITY_NOW) - hoursAgo * 60 * 60 * 1000).toISOString();

function entry(
  id: string,
  hoursAgo: number,
  agent: string,
  action: string,
  resource: string,
  decision: string,
  kind: string,
  source = ""
): PermissionEntry {
  return {
    id,
    session_id: `sess-${id}`,
    agent_name: agent,
    action,
    resource,
    decision,
    initiator_kind: kind,
    initiator_id: source,
    timestamp: AT(hoursAgo),
  };
}

const ENTRIES: PermissionEntry[] = [
  entry("1", 2, "reviewer", "execute", "bun test", "allow-once", "automation", "Nightly review"),
  entry("2", 3, "triage", "read", "internal/task/store.go", "allow-once", "daemon"),
  entry("3", 5, "code-engineer", "edit", "internal/daemon/boot.go", "timeout", "daemon"),
  entry(
    "4",
    6,
    "orchestrator",
    "fetch",
    "github.com/compozy/compozy",
    "allow-once",
    "extension",
    "spec-cycle"
  ),
  entry(
    "5",
    8,
    "code-engineer",
    "execute",
    "make migrate",
    "reject-once",
    "automation",
    "Token rollout"
  ),
  entry("6", 14, "general", "edit", "web/src/app.tsx", "allow-once", "human"),
  entry("7", 150, "general", "execute", "bun test", "allow-once", ""),
];

const UNATTENDED = new Set(["automation", "daemon", "extension"]);

export function activityEntriesFixture(unattended: boolean): PermissionEntry[] {
  return unattended ? ENTRIES.filter(row => UNATTENDED.has(row.initiator_kind)) : ENTRIES;
}

export function activityRollupFixture(unattended: boolean) {
  const rollups: PermissionRollupRow[] = activityEntriesFixture(unattended).map(row => ({
    day: row.timestamp.slice(0, 10),
    agent_name: row.agent_name,
    decision: row.decision,
    count: 1,
  }));
  return { since: AT(14 * 24), until: ACTIVITY_NOW, rollups };
}
