import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { AgentCreateHostProvider } from "@/systems/agent";
import type {
  SessionLifecycleActionHandlers,
  SessionListViewModel,
  SessionPayload,
} from "@/systems/session";
import type { WorkspacePayload } from "@/systems/workspace";

import { CmdPaletteRegistryProvider } from "../../contexts/cmd-palette-registry-context";
import { OsShellContext } from "../../contexts/os-shell-context";
import { cmdPaletteStoryRegistry } from "../../mocks/cmd-palette-fixtures";
import type { OsAttentionModel } from "../../hooks/use-os-attention";
import { DesktopMenubar } from "../desktop-menubar";
import { OsSessionsModal } from "../sessions-modal";
import { OsDockZone } from "../os-dock";
import { createStoryShell } from "./_shell-fixture";
import { buildDeskItems, DesktopShell } from "./_desktop";

function session(
  id: string,
  name: string,
  agentName: string,
  badge: SessionPayload["badge"]
): SessionPayload {
  return {
    id,
    name,
    agent_name: agentName,
    runtime: {
      status: "ready",
      transition: "initial_bind",
      effective: { provider: "codex" },
      selection_revision: 0,
    },
    workspace_id: "workspace-compozy",
    workspace_path: "/workspace/compozy",
    state: badge === "stopped" || badge === "failed" ? "stopped" : "active",
    badge,
    attachable: true,
    archived_at: null,
    available_commands: [],
    pending_interactions: [],
    created_at: "2026-07-20T12:00:00Z",
    updated_at: "2026-07-20T12:01:00Z",
  };
}

const CATALOG: SessionPayload[] = [
  session("session-1", "Checkout flow polish", "webgen", "running"),
  session("session-2", "Marketplace empty states", "webgen", "waiting-for-auth"),
  session("session-3", "Dashboard spacing audit", "webgen", "idle"),
  session("session-4", "Competitor pricing scan", "research", "running"),
  session("session-5", "ACP spec digest", "research", "failed"),
  session("session-6", "Nightly dependency sweep", "infra", "stopped"),
  session("session-7", "Provider contract review", "infra", "idle"),
];

const SESSION_ACTIONS: SessionLifecycleActionHandlers = {
  pendingAction: null,
  pendingSessionId: null,
  onArchive: fn(),
  onDelete: fn(),
  onRename: fn(),
  onStop: fn(),
  onUnarchive: fn(),
};

const SESSION_LIST_VIEW: SessionListViewModel = {
  scope: "workspace",
  sort: "last_activity",
  archived: false,
  saving: false,
  setScope: fn(),
  setSort: fn(),
  setArchived: fn(),
  workspaceGroups: [],
  collapsedWorkspaceIds: new Set<string>(),
  toggleWorkspace: fn(),
};

const WORKSPACE: WorkspacePayload = {
  id: "workspace-compozy",
  name: "compozy",
  root_dir: "/workspace/compozy",
  add_dirs: [],
  created_at: "2026-07-20T12:00:00Z",
  updated_at: "2026-07-20T12:00:00Z",
};

const ATTENTION_STORY_NOW = Date.now();
const ATTENTION_STORY_MINUTE = 60_000;

const ATTENTION: OsAttentionModel = {
  badges: { sessions: 1, tasks: 1, loops: 2 },
  notificationCount: 4,
  sections: {
    needsYou: [
      {
        kind: "session",
        id: "session-2",
        title: "Marketplace empty states",
        agentName: "webgen",
        workspaceId: "workspace-1",
        workspaceLabel: "compozy",
        badge: "waiting-for-input",
        reason: "Which empty state copy should ship?",
        changedAt: "2026-07-20T12:04:00Z",
        muted: false,
        stale: false,
      },
      {
        kind: "task",
        id: "task-42",
        title: "Approve runtime contract",
        identifier: "CompozyOS-42",
      },
      {
        kind: "loop-request",
        id: "workspace-compozy:looprun-release:confirm-rollout:0",
        title: "confirm-rollout",
        workspaceId: "workspace-compozy",
        workspaceLabel: "compozy",
        runId: "looprun-release",
        loopName: "release-train",
        nodeId: "confirm-rollout",
        itemIndex: 0,
        requestKind: "ask",
        openedAt: new Date(ATTENTION_STORY_NOW - 12 * ATTENTION_STORY_MINUTE).toISOString(),
        stale: false,
      },
      {
        kind: "loop-request",
        id: "workspace-compozy:looprun-release:apply-migration:0",
        title: "apply-migration",
        workspaceId: "workspace-compozy",
        workspaceLabel: "compozy",
        runId: "looprun-release",
        loopName: "release-train",
        nodeId: "apply-migration",
        itemIndex: 0,
        requestKind: "review",
        openedAt: new Date(ATTENTION_STORY_NOW - 6 * ATTENTION_STORY_MINUTE).toISOString(),
        expiresAt: new Date(ATTENTION_STORY_NOW + 4 * ATTENTION_STORY_MINUTE).toISOString(),
        stale: false,
      },
      {
        kind: "loop-node",
        id: "waiting",
        title: "Loop nodes waiting on you",
        state: "waiting",
      },
      {
        kind: "loop-node",
        id: "attention",
        title: "Loop nodes needing attention",
        state: "attention",
      },
    ],
    finished: [
      {
        kind: "session",
        id: "session-9",
        title: "Release notes draft",
        agentName: "hermes",
        workspaceId: "workspace-2",
        workspaceLabel: "infra",
        badge: "done",
        reason: "done",
        changedAt: "2026-07-20T11:38:00Z",
        muted: false,
        stale: false,
      },
    ],
  },
  sessions: CATALOG,
  attentionSessionsDisconnected: false,
  sessionsDisconnected: false,
  tasksDisconnected: false,
  loopRequestsDisconnected: false,
  loading: false,
};

function SessionsModalFixture() {
  const [shell] = useState(() => createStoryShell());
  const dockItems = buildDeskItems({
    open: ["sessions"],
    badges: { sessions: 1, tasks: 1 },
  });
  return (
    <OsShellContext.Provider value={shell}>
      <DesktopShell dock={false} dockItems={dockItems}>
        <OsSessionsModal
          open
          onOpenChange={fn()}
          sessions={CATALOG}
          disconnected={false}
          view={SESSION_LIST_VIEW}
          sessionActions={SESSION_ACTIONS}
          onNewSession={fn()}
        />
        <OsDockZone items={dockItems} onSelect={fn()} onNewSession={fn()} />
      </DesktopShell>
    </OsShellContext.Provider>
  );
}

function BellFixture() {
  const [shell] = useState(() => createStoryShell());
  return (
    <OsShellContext.Provider value={shell}>
      <CmdPaletteRegistryProvider registry={cmdPaletteStoryRegistry}>
        <AgentCreateHostProvider openDialog={fn()} openForDuplicate={fn()}>
          <DesktopShell
            menubar={false}
            dockItems={buildDeskItems({ badges: { sessions: 1, tasks: 1 } })}
          >
            <DesktopMenubar
              workspaces={[WORKSPACE]}
              activeWorkspace={WORKSPACE}
              onSelectWorkspace={fn()}
              onAddWorkspace={fn()}
              onRunCommand={fn()}
              activeOverlay="bell"
              onOverlayOpenChange={fn()}
              attention={ATTENTION}
              updateAvailable={false}
            />
          </DesktopShell>
        </AgentCreateHostProvider>
      </CmdPaletteRegistryProvider>
    </OsShellContext.Provider>
  );
}

const meta: Meta<typeof OsSessionsModal> = {
  title: "systems/os/components/AttentionSurfaces",
  component: OsSessionsModal,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Production attention surfaces: the session-catalog modal, focus-only menubar bell, and projection-backed dock badges.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Modal workspace view with the runtime's raw status vocabulary. */
export const SessionsWorkspace: Story = {
  args: {
    open: true,
    onOpenChange: fn(),
    sessions: CATALOG,
    disconnected: false,
    onNewSession: fn(),
    sessionActions: SESSION_ACTIONS,
  },
  render: () => <SessionsModalFixture />,
};

export const BellPopulated: Story = {
  args: {
    open: true,
    onOpenChange: fn(),
    sessions: CATALOG,
    disconnected: false,
    onNewSession: fn(),
    sessionActions: SESSION_ACTIONS,
  },
  render: () => <BellFixture />,
};

/** Projection-backed badges: sessions remains exact, while a large task count caps at 9+. */
export const DockBadges: Story = {
  args: {
    open: true,
    onOpenChange: fn(),
    sessions: CATALOG,
    disconnected: false,
    onNewSession: fn(),
    sessionActions: SESSION_ACTIONS,
  },
  render: () => (
    <DesktopShell
      dockItems={buildDeskItems({
        badges: { sessions: 1, tasks: 12 },
      })}
      deskHint
    />
  ),
};
