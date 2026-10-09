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
