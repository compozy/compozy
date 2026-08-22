import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn, userEvent, within } from "storybook/test";

import { CenteredSurface } from "@/storybook/story-layout";
import type {
  SessionLifecycleActionHandlers,
  SessionListViewModel,
  SessionPayload,
} from "@/systems/session";
import { SessionList } from "@/systems/session/components/session-list/session-list";

function session(id: string, title: string, workspaceId: string): SessionPayload {
  return {
    profile_id: "00000000000000000000000000",
    profile_name: "default",
    id,
    name: title,
    agent_name: "codex",
    runtime: {
      status: "ready",
      transition: "initial_bind",
      effective: { provider: "codex" },
      selection_revision: 0,
    },
    workspace_id: workspaceId,
    workspace_path: `/workspace/${workspaceId}`,
    state: "active",
    badge: "running",
    attachable: true,
    archived_at: null,
    available_commands: [],
    pending_interactions: [],
    created_at: "2026-08-16T12:00:00Z",
    updated_at: "2026-08-16T12:10:00Z",
  };
}

const ACTIONS: SessionLifecycleActionHandlers = {
  pendingAction: null,
  pendingSessionId: null,
  onArchive: fn(),
  onDelete: fn(),
  onRename: fn(),
  onStop: fn(),
  onUnarchive: fn(),
};

type CatalogState = "sort" | "group-failure" | "group-collapsed";

function SessionCatalogContract({ state }: { state: CatalogState }) {
  const allWorkspaces = state !== "sort";
  const workspaceGroups: SessionListViewModel["workspaceGroups"] = [
    {
      workspaceId: "workspace-compozy",
      workspaceName: "compozy",
      sessions: [session("session-runtime", "Runtime parity pass", "workspace-compozy")],
      total: 1,
      loading: false,
      failed: false,
      retry: fn(),
    },
    {
      workspaceId: "workspace-infra",
      workspaceName: "infra",
      sessions: [session("session-release", "Release gate", "workspace-infra")],
      total: 4,
      loading: false,
      failed: state === "group-failure",
      retry: fn(),
    },
  ];
  const view: SessionListViewModel = {
    scope: allWorkspaces ? "all-workspaces" : "workspace",
    sort: "attention",
    archived: false,
    saving: false,
    setScope: fn(),
    setSort: fn(),
    setArchived: fn(),
    aggregate: false,
    scopeLabel: "default",
    ownerOf: () => ({ id: "00000000000000000000000000", name: "default", archived: false }),
    workspaceGroups,
    collapsedWorkspaceIds: state === "group-collapsed" ? new Set(["workspace-infra"]) : new Set(),
    toggleWorkspace: fn(),
  };
  return (
    <div className="flex h-160 w-120 overflow-hidden rounded-lg border border-line bg-canvas-soft">
      <SessionList
        sessions={allWorkspaces ? [] : workspaceGroups.flatMap(group => group.sessions)}
        disconnected={false}
        collapsedThreadIds={[]}
        view={view}
        onToggleThread={fn()}
        onSelectSession={fn()}
        onNewSession={fn()}
        sessionActions={ACTIONS}
        testIdPrefix="contract-catalog"
      />
    </div>
  );
}

const meta: Meta<typeof SessionCatalogContract> = {
  title: "systems/os/components/SessionCatalogContract",
  component: SessionCatalogContract,
  parameters: { layout: "fullscreen" },
  decorators: [
    Story => (
      <CenteredSurface>
        <Story />
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const SortMenu: Story = {
  args: { state: "sort" },
  play: async ({ canvasElement }) => {
    await userEvent.click(
      within(canvasElement.ownerDocument.body).getByTestId("contract-catalog-sort-trigger")
    );
  },
};

export const WorkspaceGroupFailure: Story = {
  args: { state: "group-failure" },
};

export const WorkspaceGroupCollapsed: Story = {
  args: { state: "group-collapsed" },
};
