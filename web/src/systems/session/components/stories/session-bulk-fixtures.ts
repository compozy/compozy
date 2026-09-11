import { primarySessionFixture } from "../../testing";
import type { SessionPayload } from "../../types";

/** Normative board fixture: one idle root and seven children in general. */
export const BULK_SESSION_FAMILY: SessionPayload[] = [
  ["orchestrator", "Orchestrator", "idle"],
  ["issue-613", "Issue 613 — Session bulk actions", "running"],
  ["integrate", "Integrate PRs 602–606", "stopped"],
  ["issue-606", "Issue 606 — Clear stale worktrees", "stopped"],
  ["issue-605", "Issue 605 — Rename session inline", "stopped"],
  ["issue-602", "Issue 602 — Tail logs in terminal", "stopped"],
  ["issue-603", "Issue 603 — Dropdown focus trap", "stopped"],
  ["issue-604", "Issue 604 — Skip empty turns", "stopped"],
].map(([id, name, badge], index) => ({
  ...primarySessionFixture,
  id: id!,
  name: name!,
  badge: badge!,
  agent_name: "general",
  state: badge === "stopped" ? "stopped" : "active",
  archived_at: null,
  updated_at: index < 2 ? "2026-09-11T15:00:00Z" : "2026-09-10T21:00:00Z",
  lineage:
    index === 0
      ? undefined
      : {
          parent_session_id: "orchestrator",
          root_session_id: "orchestrator",
          spawn_depth: 1,
          auto_stop_on_parent: false,
          notify_creator: true,
          spawn_budget: { max_children: 0, max_depth: 0, ttl_seconds: 0 },
          permission_policy: {
            tools: [],
            skills: [],
            mcp_servers: [],
            workspace_paths: [],
            network_channels: [],
            sandbox_profiles: [],
          },
        },
}));
export const BULK_SELECTED_SESSIONS = BULK_SESSION_FAMILY.slice(1, 4);
