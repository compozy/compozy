import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

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
              network_channels: [],
              sandbox_profiles: [],
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
