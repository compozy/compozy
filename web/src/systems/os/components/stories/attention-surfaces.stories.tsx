import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn, userEvent, within } from "storybook/test";

import { AgentCreateHostProvider } from "@/systems/agent";
import type { SessionPayload } from "@/systems/session";
import type { WorkspacePayload } from "@/systems/workspace";

import { OsShellContext } from "../../contexts/os-shell-context";
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
    provider: "codex",
    workspace_id: "workspace-agh",
    workspace_path: "/workspace/agh",
    state: badge === "stopped" || badge === "failed" ? "stopped" : "active",
    badge,
    attachable: true,
    available_commands: [],
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

const WORKSPACE: WorkspacePayload = {
  id: "workspace-agh",
  name: "agh",
  root_dir: "/workspace/agh",
  add_dirs: [],
  created_at: "2026-07-20T12:00:00Z",
  updated_at: "2026-07-20T12:00:00Z",
};

const ATTENTION: OsAttentionModel = {
  badges: { sessions: 1, tasks: 1 },
  notificationCount: 2,
  rows: [
    {
      kind: "session",
      id: "session-2",
      title: "Marketplace empty states",
      agentName: "webgen",
    },
    {
      kind: "task",
      id: "task-42",
      title: "Approve runtime contract",
      identifier: "AGH-42",
    },
  ],
  sessions: CATALOG,
  sessionsDisconnected: false,
  tasksDisconnected: false,
  loading: false,
};

function SessionsModalFixture({ collapsedAgent }: { collapsedAgent?: string }) {
  const [shell] = useState(() => createStoryShell({ collapsedAgent }));
  const dockItems = buildDeskItems({
    open: ["sessions"],
    badges: { sessions: 1, tasks: 1 },
  });
  return (
    <OsShellContext.Provider value={shell}>
      <DesktopShell dock={false} dockItems={dockItems}>
        <OsSessionsModal open onOpenChange={fn()} sessions={CATALOG} disconnected={false} />
        <OsDockZone items={dockItems} onSelect={fn()} onNewSession={fn()} />
      </DesktopShell>
    </OsShellContext.Provider>
  );
}

function BellFixture() {
  const [shell] = useState(() => createStoryShell());
  return (
    <OsShellContext.Provider value={shell}>
      <AgentCreateHostProvider openDialog={fn()} openForDuplicate={fn()}>
        <DesktopShell
          menubar={false}
          dockItems={buildDeskItems({ badges: { sessions: 1, tasks: 1 } })}
        >
          <DesktopMenubar
            workspaces={[WORKSPACE]}
            activeWorkspace={WORKSPACE}
            onOpenWorkspaces={fn()}
            onSelectWorkspace={fn()}
            onAddWorkspace={fn()}
            onNewSession={fn()}
            onOpenPalette={fn()}
            onOpenDesktops={fn()}
            onToggleSessions={fn()}
            activeOverlay="bell"
            onOverlayOpenChange={fn()}
            attention={ATTENTION}
          />
        </DesktopShell>
      </AgentCreateHostProvider>
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

/** Modal recent view with the runtime's raw status vocabulary and six-row recent limit. */
export const SessionsRecent: Story = {
  args: { open: true, onOpenChange: fn(), sessions: CATALOG, disconnected: false },
  render: () => <SessionsModalFixture />,
};

/** Grouped catalog with the webgen agent persisted as collapsed. */
export const SessionsGrouped: Story = {
  args: { open: true, onOpenChange: fn(), sessions: CATALOG, disconnected: false },
  render: () => <SessionsModalFixture collapsedAgent="webgen" />,
  play: async ({ canvasElement }) => {
    const body = canvasElement.ownerDocument.body;
    await userEvent.click(within(body).getByRole("button", { name: "Show all sessions" }));
  },
};

/** Open attention bell with one waiting session and one task awaiting approval. */
export const BellPopulated: Story = {
  args: { open: true, onOpenChange: fn(), sessions: CATALOG, disconnected: false },
  render: () => <BellFixture />,
};

/** Projection-backed badges: sessions remains exact, while a large task count caps at 9+. */
export const DockBadges: Story = {
  args: { open: true, onOpenChange: fn(), sessions: CATALOG, disconnected: false },
  render: () => (
    <DesktopShell
      dockItems={buildDeskItems({
        badges: { sessions: 1, tasks: 12 },
      })}
      deskHint
    />
  ),
};
