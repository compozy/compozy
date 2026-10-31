import { sessionRuntime } from "@/systems/session/mocks/fixtures";
import type { SessionPayload } from "@/systems/session/types";

export function sidebarSession(
  id: string,
  name: string,
  agentName: string,
  badge: SessionPayload["badge"],
  parentId?: string
): SessionPayload {
  return {
    supervision: null,
    profile_id: "00000000000000000000000000",
    profile_name: "default",
    scope: "workspace",
    effective_workspace_access: "",
    id,
    name,
    agent_name: agentName,
    runtime: sessionRuntime("claude"),
    workspace_id: "ws-product",
    state: badge === "stopped" ? "stopped" : "active",
    badge,
    attachable: true,
    archived_at: null,
    available_commands: [],
    pending_interactions: [],
    board_column: "done",
    pull_request: null,
    pull_request_lookup: null,
    ...(parentId
      ? {
          lineage: {
            parent_session_id: parentId,
            root_session_id: parentId,
            spawn_depth: 1,
            auto_stop_on_parent: false,
            notify_creator: true,
            spawn_budget: { max_children: 0, max_depth: 0, ttl_seconds: 0 },
            permission_policy: {
              tools: [],
              skills: [],
              mcp_servers: [],
              workspace_paths: [],
            },
          },
        }
      : {}),
    created_at: "2026-08-06T12:00:00Z",
    updated_at: "2026-08-06T18:09:00Z",
  };
}

export const PROVENANCE_FAMILY: SessionPayload[] = [
  sidebarSession("sess-migration", "Calm transcript migration", "webgen", "running"),
  sidebarSession(
    "sess-digest",
    "Digest calm-surface references",
    "research",
    "stopped",
    "sess-migration"
  ),
  sidebarSession(
    "sess-markers",
    "Migrate marker components",
    "webgen",
    "running",
    "sess-migration"
  ),
  sidebarSession("sess-empty-states", "Marketplace empty states", "webgen", "waiting-for-auth"),
  sidebarSession("sess-pricing", "Competitor pricing scan", "research", "running"),
  sidebarSession("sess-scrape", "Scrape pricing pages", "research", "failed", "sess-pricing"),
  sidebarSession("sess-deps", "Nightly deps sweep", "infra", "running"),
];

const FOLDER_AT = "2026-10-17T00:00:00Z";
export const STORY_FOLDERS = [
  { id: "fld-calm", name: "Calm surfaces", position: 0, session_count: 2 },
  { id: "fld-pricing", name: "Pricing research", position: 1, session_count: 0 },
].map(folder => ({
  ...folder,
  workspace_id: "ws-product",
  profile_id: "00000000000000000000000000",
  created_at: FOLDER_AT,
  updated_at: FOLDER_AT,
}));

export function withSubagents(
  session: SessionPayload,
  live: number,
  total: number,
  failed = 0,
  attention = 0
): SessionPayload {
  return {
    ...session,
    subagent_summary: { live, total, failed, attention, most_urgent: "running" },
  };
}

export const VIEWED_SUBAGENT: SessionPayload = {
  ...minutesAgo(
    sidebarSession("sess-webhooks", "Check webhook retries", "reviewer", "running", "sess-ship"),
    1
  ),
  lineage: {
    ...sidebarSession("sess-webhooks", "", "reviewer", "running", "sess-ship").lineage!,
    kind: "spawn",
    spawn_role: "subagent",
  },
};

/** Recent activity so rows read relative times (`now`, `4m`), as the sidebar shows live work. */
export function minutesAgo(session: SessionPayload, minutes: number): SessionPayload {
  return { ...session, updated_at: new Date(Date.now() - minutes * 60_000).toISOString() };
}

export const SUBAGENT_PARENTS: SessionPayload[] = [
  minutesAgo(
    withSubagents(
      sidebarSession("sess-ship", "Ship checkout v2", "claude", "running"),
      3,
      10,
      1,
      1
    ),
    0
  ),
  minutesAgo(
    withSubagents(sidebarSession("sess-cart", "Refactor cart totals", "claude", "done"), 2, 4),
    4
  ),
  minutesAgo(sidebarSession("sess-nightly", "Nightly delivery", "claude", "done"), 120),
  minutesAgo(
    sidebarSession("sess-task-04", "Implement task_04", "codex", "done", "sess-nightly"),
    121
  ),
  minutesAgo(
    sidebarSession("sess-review-04", "Review task_04", "claude", "done", "sess-nightly"),
    122
  ),
  minutesAgo(sidebarSession("sess-refund", "Fix flaky refund test", "codex", "done"), 300),
  minutesAgo(
    withSubagents(sidebarSession("sess-sdk", "Upgrade payment SDK", "claude", "stopped"), 0, 6, 2),
    1440
  ),
];

export const STABLE_FAMILY = [
  sidebarSession("stable-parent", "Parent", "general", "running"),
  sidebarSession("stable-beta", "Beta", "codex", "running", "stable-parent"),
  sidebarSession("stable-alpha", "Alpha", "claude", "waiting-for-input", "stable-parent"),
  sidebarSession("stable-older", "Earlier", "general", "running"),
].map((session, index) => ({ ...session, created_at: `2026-10-16T12:0${4 - index}:00Z` }));

export const GOAL_OWNER = sidebarSession(
  "sess-goal-owner",
  "Ship the scoped fix",
  "general",
  "running"
);
export const HISTORICAL_JUDGES = ["one", "two", "three"].map(suffix => ({
  ...sidebarSession(
    `sess-judge-${suffix}`,
    "loop gate general objective_satisfied",
    "general",
    "needs-attention"
  ),
  type: "system" as const,
  state: "stopped" as const,
}));

/** The same row on another runtime; `null` is a session that has not bound one yet. */
export function onRuntime(
  session: SessionPayload,
  provider: string | null,
  model = ""
): SessionPayload {
  return {
    ...session,
    runtime:
      provider === null
        ? { status: "unbound", selection_revision: 0 }
        : { ...sessionRuntime(provider), effective: { provider, ...(model ? { model } : {}) } },
  };
}

export const MIXED_RUNTIMES: SessionPayload[] = [
  onRuntime(
    minutesAgo(
      sidebarSession("sess-orch", "Orchestrate the October fixes", "general", "running"),
      0
    ),
    "claude",
    "claude-opus-5-5"
  ),
  onRuntime(
    minutesAgo(
      sidebarSession(
        "sess-sort",
        "Stable session sort under parallel catalog writes",
        "general",
        "running",
        "sess-orch"
      ),
      1
    ),
    "codex",
    "gpt-6.1-sol"
  ),
  onRuntime(
    minutesAgo(
      {
        ...sidebarSession(
          "sess-origin",
          "Tailscale gateway origin",
          "general",
          "needs-attention",
          "sess-orch"
        ),
        // A child whose turn ended without delivering (#1065).
        attention: "ended_early",
      },
      2
    ),
    "codex",
    "gpt-6.1-sol"
  ),
  onRuntime(
    minutesAgo(
      sidebarSession("sess-viewer", "Image viewer return path", "general", "running", "sess-orch"),
      3
    ),
    "claude",
    "claude-opus-5-5"
  ),
  onRuntime(
    minutesAgo(
      sidebarSession("sess-claim", "Loop claim token", "general", "failed", "sess-orch"),
      6
    ),
    "gemini",
    "gemini-3.1-pro"
  ),
  onRuntime(
    minutesAgo(
      sidebarSession("sess-cards", "Session message cards", "general", "done", "sess-orch"),
      9
    ),
    "cursor"
  ),
  onRuntime(minutesAgo(sidebarSession("sess-new", "New session", "general", "idle"), 0), null),
  onRuntime(
    minutesAgo(
      sidebarSession("sess-cart-2", "Refactor cart totals", "general", "waiting-for-input"),
      14
    ),
    "claude",
    "claude-sonnet-5-5"
  ),
  onRuntime(
    minutesAgo(sidebarSession("sess-refund-2", "Fix flaky refund test", "general", "stopped"), 300),
    "codex",
    "gpt-6.1-sol"
  ),
];

export const PROFILE_OWNERS = {
  "00000000000000000000000000": {
    id: "00000000000000000000000000",
    name: "Personal",
    color: "#8a8f98",
    icon: "user-round",
    archived: false,
  },
  "profile-design": {
    id: "profile-design",
    name: "Design",
    color: "#c26ad6",
    icon: "megaphone",
    archived: false,
  },
} as const;

export function inProfile(
  session: SessionPayload,
  profileId: keyof typeof PROFILE_OWNERS
): SessionPayload {
  return { ...session, profile_id: profileId, profile_name: PROFILE_OWNERS[profileId].name };
}

export const LONG_AGENT = "open-design-designer-with-extended-review-loop";
