import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";
import { BULK_SESSION_FAMILY, BULK_SELECTED_SESSIONS } from "./session-bulk-fixtures";

import { CenteredSurface } from "@/storybook/story-layout";

import { sessionRuntime } from "../../mocks/fixtures";
import type { SessionPayload } from "../../types";
import { SessionSidebar } from "../session-sidebar";

function sidebarSession(
  id: string,
  name: string,
  agentName: string,
  badge: string,
  parentId?: string
): SessionPayload {
  return {
    supervision: null,
    profile_id: "00000000000000000000000000",
    profile_name: "default",
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

const PROVENANCE_FAMILY: SessionPayload[] = [
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

const meta: Meta<typeof SessionSidebar> = {
  title: "systems/session/components/SessionSidebar",
  component: SessionSidebar,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "In-window sessions rail with provenance threads: child sessions nest under their origin behind a hairline connector.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="flex h-[560px] w-full max-w-3xl overflow-hidden rounded-lg border border-line-strong bg-canvas">
          <Story />
          <div className="flex-1" />
        </div>
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Open rail with two provenance threads; the active session carries the accent
 * marker and child sessions keep their own agent identity in the sub line.
 */
export const ProvenanceThreads: Story = {
  args: {
    view: {
      scope: "workspace",
      sort: "last_activity",
      archived: false,
      saving: false,
      setScope: fn(),
      setSort: fn(),
      setArchived: fn(),
      workspaceGroups: [],
      collapsedWorkspaceIds: new Set(),
      toggleWorkspace: fn(),
      aggregate: false,
      scopeLabel: "default",
      ownerOf: () => ({ id: "default", name: "default", archived: false }),
    },
    open: true,
    sessions: PROVENANCE_FAMILY,
    disconnected: false,
    collapsedThreadIds: [],
    currentSessionId: "sess-migration",
    onToggleThread: fn(),
    onSelectSession: fn(),
    onNewSession: fn(),
    sessionActions: {
      pendingAction: null,
      pendingSessionId: null,
      onArchive: fn(),
      onDelete: fn(),
      onRename: fn(),
      onStop: fn(),
      onUnarchive: fn(),
    },
  },
};

/**
 * A collapsed thread keeps the most urgent child state on its toggle — the
 * failed scrape surfaces as a danger dot beside the child count.
 */
export const CollapsedThreadSignal: Story = {
  args: {
    ...ProvenanceThreads.args,
    collapsedThreadIds: ["sess-pricing"],
    currentSessionId: "sess-deps",
  },
};

async function selectBoardSessions(canvasElement: HTMLElement) {
  const canvas = within(canvasElement);
  const user = userEvent.setup();
  await user.keyboard("{Control>}");
  for (const session of BULK_SELECTED_SESSIONS) {
    await user.click(canvas.getByTestId(`session-sidebar-session-${session.id}`));
  }
  await user.keyboard("{/Control}");
  await expect(canvas.getByTestId("session-sidebar-selection-count")).toHaveTextContent(
    "3 selected"
  );
}

export const SelectionMode: Story = {
  tags: ["play-fn"],
  args: {
    ...ProvenanceThreads.args,
    sessions: BULK_SESSION_FAMILY,
    currentSessionId: "orchestrator",
    sessionActions: {
      ...ProvenanceThreads.args!.sessionActions!,
      onStopMany: fn(),
      onArchiveMany: fn(),
      onUnarchiveMany: fn(),
      onDeleteMany: fn(),
    },
  },
  play: async ({ canvasElement }) => selectBoardSessions(canvasElement),
};

export const SelectionMenuOpen: Story = {
  ...SelectionMode,
  play: async ({ canvasElement }) => {
    await selectBoardSessions(canvasElement);
    await userEvent.click(within(canvasElement).getByTestId("session-sidebar-selection-more"));
  },
};

function withSubagents(
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

const VIEWED_SUBAGENT: SessionPayload = {
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
function minutesAgo(session: SessionPayload, minutes: number): SessionPayload {
  return { ...session, updated_at: new Date(Date.now() - minutes * 60_000).toISOString() };
}

const SUBAGENT_PARENTS: SessionPayload[] = [
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

/**
 * Subagents (nav VC-02): subagent sessions take no rows; each parent carries one
 * chip — attention, delegated while its turn is idle, or the failed total — and
 * plain spawned children keep their thread.
 */
export const SubagentParents: Story = {
  args: {
    ...ProvenanceThreads.args,
    sessions: SUBAGENT_PARENTS,
    currentSessionId: "sess-ship",
  },
};

/** Viewing a subagent reveals only that session, nested under its parent row. */
export const RevealedSubagent: Story = {
  args: {
    ...SubagentParents.args,
    currentSessionId: VIEWED_SUBAGENT.id,
    revealedSession: VIEWED_SUBAGENT,
  },
};

/** The same row on another runtime; `null` is a session that has not bound one yet. */
function onRuntime(session: SessionPayload, provider: string | null, model = ""): SessionPayload {
  return {
    ...session,
    runtime:
      provider === null
        ? { status: "unbound", selection_revision: 0 }
        : { ...sessionRuntime(provider), effective: { provider, ...(model ? { model } : {}) } },
  };
}

const MIXED_RUNTIMES: SessionPayload[] = [
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
      sidebarSession(
        "sess-origin",
        "Tailscale gateway origin",
        "general",
        "needs-attention",
        "sess-orch"
      ),
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

/**
 * Each row names its runtime with the provider's mark before the agent, in
 * neutral ink so only live state carries color; the unbound session shows none.
 */
export const MixedRuntimes: Story = {
  args: {
    ...ProvenanceThreads.args,
    sessions: MIXED_RUNTIMES,
    currentSessionId: "sess-orch",
  },
};

/** Selection keeps every provider mark; the checkbox covers only the state mark. */
export const MixedRuntimesSelection: Story = {
  tags: ["play-fn"],
  args: {
    ...MixedRuntimes.args,
    sessionActions: SelectionMode.args!.sessionActions!,
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const user = userEvent.setup();
    await user.keyboard("{Control>}");
    await user.click(canvas.getByTestId("session-sidebar-session-sess-sort"));
    await user.click(canvas.getByTestId("session-sidebar-session-sess-claim"));
    await user.keyboard("{/Control}");
    await expect(canvas.getByTestId("session-sidebar-selection-count")).toHaveTextContent(
      "2 selected"
    );
  },
};
