// Suite: OS sessions modal
// Invariant: the modal filters catalog truth, retains group state, and dismisses via Dialog.
// Boundary IN: OsSessionsModal, OS controller, and routing coordinator.
// Boundary OUT: session catalog transport and full browser window journeys.
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type {
  SessionLifecycleActionHandlers,
  SessionListViewModel,
  SessionPayload,
} from "@/systems/session";

import { OsShellContext, type OsShellHandle } from "../../contexts/os-shell-context";
import { WindowManagerRuntime } from "../../runtime/window-manager-runtime";
import { RoutingCoordinator, type OsRouterPort } from "../../lib/routing-coordinator";
import { OsSessionsModal } from "../sessions-modal";

const jumpToSession = vi.hoisted(() => vi.fn());
vi.mock("../../hooks/use-attention-jump", () => ({
  useAttentionJump: () => jumpToSession,
}));

function session(overrides: Partial<SessionPayload> = {}): SessionPayload {
  return {
    profile_name: "default",
    profile_id: "00000000000000000000000000",
    id: "session-1",
    name: "Web shell polish",
    agent_name: "codex",
    runtime: {
      status: "ready",
      transition: "initial_bind",
      effective: { provider: "codex" },
      selection_revision: 0,
    },
    workspace_id: "workspace-1",
    workspace_path: "/workspace/compozy",
    state: "active",
    badge: "running",
    attachable: true,
    archived_at: null,
    available_commands: [],
    created_at: "2026-07-20T12:00:00Z",
    updated_at: "2026-07-20T12:01:00Z",
    ...overrides,
    pending_interactions: overrides.pending_interactions ?? [],
  };
}

const SESSIONS: SessionPayload[] = [
  session(),
  session({ id: "session-2", name: "Runtime audit", agent_name: "webgen", badge: "idle" }),
  session({ id: "session-3", name: "Release notes", agent_name: "codex", badge: "stopped" }),
];

const SESSION_ACTIONS: SessionLifecycleActionHandlers = {
  pendingAction: null,
  pendingSessionId: null,
  onArchive: () => {},
  onDelete: () => {},
  onRename: () => {},
  onStop: () => {},
  onUnarchive: () => {},
};

function listView(overrides: Partial<SessionListViewModel> = {}): SessionListViewModel {
  return {
    scope: "workspace",
    sort: "last_activity",
    archived: false,
    saving: false,
    setScope: vi.fn(),
    setSort: vi.fn(),
    setArchived: vi.fn(),
    workspaceGroups: [],
    aggregate: false,
    scopeLabel: "default",
    ownerOf: () => ({ id: "00000000000000000000000000", name: "default", archived: false }),
    collapsedWorkspaceIds: new Set<string>(),
    toggleWorkspace: vi.fn(),
    ...overrides,
  };
}

const managers: WindowManagerRuntime[] = [];

function createShell(): OsShellHandle {
  const manager = new WindowManagerRuntime(new QueryClient());
  managers.push(manager);
  const port: OsRouterPort = { navigate: () => {}, replace: () => {} };
  const coordinator = new RoutingCoordinator(manager, port);
  coordinator.completeHydration();
  return { projection: manager.projectionAtom, manager, coordinator };
}

function renderModal(
  shell: OsShellHandle,
  open = true,
  view: SessionListViewModel = listView(),
  onNewSession: () => void = vi.fn(),
  sessions: readonly SessionPayload[] = SESSIONS
) {
  return render(
    <OsShellContext.Provider value={shell}>
      <OsSessionsModal
        open={open}
        onOpenChange={() => {}}
        sessions={sessions}
        disconnected={false}
        view={view}
        onNewSession={onNewSession}
        sessionActions={SESSION_ACTIONS}
      />
    </OsShellContext.Provider>
  );
}

describe("OsSessionsModal", () => {
  afterEach(() => {
    jumpToSession.mockClear();
    for (const manager of managers.splice(0)) manager.destroy();
  });

  it("Should coordinate a foreign workspace row without exposing mis-scoped actions", async () => {
    const user = userEvent.setup();
    const foreign = session({
      id: "session-foreign",
      agent_name: "claude",
      workspace_id: "workspace-2",
      workspace_path: "/workspace/other",
    });
    renderModal(
      createShell(),
      true,
      listView({
        scope: "all-workspaces",
        workspaceGroups: [
          {
            workspaceId: "workspace-2",
            workspaceName: "Other",
            sessions: [foreign],
            total: 1,
            loading: false,
            failed: false,
            retry: vi.fn(),
          },
        ],
      })
    );

    await user.click(screen.getByTestId("os-sessions-modal-session-session-foreign"));

    expect(jumpToSession).toHaveBeenCalledExactlyOnceWith({
      sessionId: "session-foreign",
      agentName: "claude",
      workspaceId: "workspace-2",
    });
    expect(screen.queryByTestId("session-row-actions-session-foreign")).toBeNull();
  });

  it("Should nest provenance children under their parent thread and fold them behind the toggle", async () => {
    const user = userEvent.setup();
    const shell = createShell();
    const child = session({
      id: "session-child",
      name: "Digest references",
      agent_name: "webgen",
      badge: "failed",
      lineage: {
        parent_session_id: "session-1",
        root_session_id: "session-1",
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
    });
    render(
      <OsShellContext.Provider value={shell}>
        <OsSessionsModal
          open
          onOpenChange={() => {}}
          sessions={[...SESSIONS, child]}
          disconnected={false}
          view={listView()}
          onNewSession={vi.fn()}
          sessionActions={SESSION_ACTIONS}
        />
      </OsShellContext.Provider>
    );

    const [thread] = screen.getAllByTestId("os-sessions-modal-thread-session-1");
    expect(thread).toBeDefined();
    expect(
      within(thread!).getByTestId("os-sessions-modal-session-session-child")
    ).toBeInTheDocument();

    const [toggle] = screen.getAllByTestId("os-sessions-modal-thread-toggle-session-1");
    expect(toggle).toBeDefined();
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    await user.click(toggle!);
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(
      within(thread!).getByTestId("os-sessions-modal-session-session-child").closest("[inert]")
    ).not.toBeNull();
    await user.click(toggle!);
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(
      within(thread!).getByTestId("os-sessions-modal-session-session-child").closest("[inert]")
    ).toBeNull();
  });

  it("Should keep a thread's full ancestor path when only a descendant matches", async () => {
    const user = userEvent.setup();
    const shell = createShell();
    const parent = session({
      id: "session-child",
      name: "Digest references",
      agent_name: "webgen",
      lineage: {
        parent_session_id: "session-1",
        root_session_id: "session-1",
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
    });
    const grandchild = session({
      id: "session-grandchild",
      name: "Scrape pricing pages",
      agent_name: "webgen",
      lineage: {
        parent_session_id: parent.id,
        root_session_id: "session-1",
        spawn_depth: 2,
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
    });
    render(
      <OsShellContext.Provider value={shell}>
        <OsSessionsModal
          open
          onOpenChange={() => {}}
          sessions={[...SESSIONS, parent, grandchild]}
          disconnected={false}
          view={listView()}
          onNewSession={vi.fn()}
          sessionActions={SESSION_ACTIONS}
        />
      </OsShellContext.Provider>
    );

    const filter = screen.getByRole("searchbox", { name: "Filter sessions" });
    await user.type(filter, "scrape");
    expect(screen.getAllByTestId("os-sessions-modal-session-session-1")).not.toHaveLength(0);
    expect(screen.getAllByTestId("os-sessions-modal-session-session-child")).not.toHaveLength(0);
    expect(screen.getAllByTestId("os-sessions-modal-session-session-grandchild")).not.toHaveLength(
      0
    );
    expect(screen.queryByTestId("os-sessions-modal-session-session-2")).toBeNull();
  });

  it("Should filter live by title or agent and restore the full catalog when cleared (UT-067)", async () => {
    const user = userEvent.setup();
    const shell = createShell();
    renderModal(shell);

    const filter = screen.getByRole("searchbox", { name: "Filter sessions" });
    await user.type(filter, "web");
    expect(screen.getAllByTestId("os-sessions-modal-session-session-1")).not.toHaveLength(0);
    expect(screen.getAllByTestId("os-sessions-modal-session-session-2")).not.toHaveLength(0);
    expect(screen.queryByTestId("os-sessions-modal-session-session-3")).toBeNull();

    await user.clear(filter);
    expect(screen.getAllByTestId("os-sessions-modal-session-session-3")).not.toHaveLength(0);
  });

  it("Should name the active profile in filtered and archived empty states", async () => {
    const shell = createShell();
    renderModal(shell, true, listView({ scopeLabel: "marketing" }));

    fireEvent.change(screen.getByLabelText("Filter sessions"), {
      target: { value: "does-not-exist" },
    });
    expect(screen.getByText("No matching sessions in marketing yet.")).toBeInTheDocument();

    renderModal(
      createShell(),
      true,
      listView({ archived: true, scopeLabel: "marketing" }),
      vi.fn(),
      []
    );
    expect(screen.getByText("No archived sessions in marketing yet.")).toBeInTheDocument();
  });

  it("Should write the widest breadth through the globe toggle (UT-068)", async () => {
    const user = userEvent.setup();
    const shell = createShell();
    // Breadth is an operator preference the shell owns, so it arrives as a prop
    // and the toggle only reports the next value.
    const narrow = listView();
    const first = renderModal(shell, true, narrow);

    const toggle = screen.getByTestId("os-sessions-modal-scope");
    expect(toggle).toHaveAccessibleName("All workspaces");
    expect(toggle).toHaveAttribute("aria-pressed", "false");
    await user.click(toggle);
    expect(narrow.setScope).toHaveBeenCalledWith("all-workspaces");

    first.unmount();
    const wide = listView({
      scope: "all-workspaces",
      workspaceGroups: [
        {
          workspaceId: "workspace-2",
          workspaceName: "Other",
          sessions: [SESSIONS[0]!],
          total: 1,
          loading: false,
          failed: false,
          retry: vi.fn(),
        },
      ],
    });
    renderModal(shell, true, wide);
    const pressed = screen.getByTestId("os-sessions-modal-scope");
    expect(pressed).toHaveAttribute("aria-pressed", "true");
    await user.click(pressed);
    expect(wide.setScope).toHaveBeenCalledWith("workspace");
  });

  it("Should dismiss the Dialog and restore focus to the opener (UT-084)", async () => {
    const user = userEvent.setup();
    const shell = createShell();

    function Harness() {
      const [open, setOpen] = useState(false);
      return (
        <OsShellContext.Provider value={shell}>
          <button type="button" onClick={() => setOpen(true)}>
            Open sessions
          </button>
          <OsSessionsModal
            open={open}
            onOpenChange={setOpen}
            sessions={SESSIONS}
            disconnected={false}
            view={listView()}
            onNewSession={vi.fn()}
            sessionActions={SESSION_ACTIONS}
          />
        </OsShellContext.Provider>
      );
    }

    render(<Harness />);

    const trigger = screen.getByRole("button", { name: "Open sessions" });
    await user.click(trigger);
    expect(screen.getByTestId("os-sessions-modal")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Close sessions" }));

    await waitFor(() => expect(screen.queryByTestId("os-sessions-modal")).toBeNull());
    expect(trigger).toHaveFocus();
  });

  it("Should switch the list to the archive and keep unarchive reachable", async () => {
    const user = userEvent.setup();
    const shell = createShell();
    // Breadth and the archive are two independent controls in the same row.
    const active = listView();
    const first = renderModal(shell, true, active);

    const archivedToggle = screen.getByTestId("os-sessions-modal-archived");
    expect(archivedToggle).toHaveAccessibleName("Archived");
    expect(archivedToggle).toHaveAttribute("aria-pressed", "false");
    expect(screen.getByTestId("os-sessions-modal-scope")).toHaveAttribute("aria-pressed", "false");
    await user.click(archivedToggle);
    expect(active.setArchived).toHaveBeenCalledWith(true);

    first.unmount();
    const archivedSession = session({
      id: "session-archived",
      name: "Archived audit",
      state: "stopped",
      badge: "stopped",
      archived_at: "2026-08-04T12:00:00Z",
    });
    render(
      <OsShellContext.Provider value={shell}>
        <OsSessionsModal
          open
          onOpenChange={() => {}}
          sessions={[archivedSession]}
          disconnected={false}
          view={listView({ archived: true })}
          onNewSession={vi.fn()}
          sessionActions={SESSION_ACTIONS}
        />
      </OsShellContext.Provider>
    );

    expect(screen.getByTestId("os-sessions-modal-archived")).toHaveAttribute(
      "aria-pressed",
      "true"
    );
    expect(screen.getByTestId("os-sessions-modal-session-session-archived")).toHaveTextContent(
      "Archived"
    );
    fireEvent.click(screen.getByTestId("session-row-actions-session-archived"));
    expect(screen.getByTestId("session-row-unarchive-session-archived")).toBeInTheDocument();
    expect(screen.queryByTestId("session-row-archive-session-archived")).toBeNull();
  });

  it("Should start a session from the toolbar instead of a footer action", async () => {
    const user = userEvent.setup();
    const onNewSession = vi.fn();
    renderModal(createShell(), true, listView(), onNewSession);

    await user.click(screen.getByTestId("os-sessions-modal-new-session"));

    expect(onNewSession).toHaveBeenCalledOnce();
  });

  it("Should keep the catalog open when a row requests delete confirmation", async () => {
    const shell = createShell();
    const onDelete = vi.fn();
    const onOpenChange = vi.fn();

    render(
      <OsShellContext.Provider value={shell}>
        <OsSessionsModal
          open
          onOpenChange={onOpenChange}
          sessions={SESSIONS}
          disconnected={false}
          view={listView()}
          onNewSession={vi.fn()}
          sessionActions={{ ...SESSION_ACTIONS, onDelete }}
        />
      </OsShellContext.Provider>
    );

    fireEvent.click(screen.getAllByTestId("session-row-actions-session-1")[0]!);
    fireEvent.click(screen.getByTestId("session-row-delete-session-1"));

    expect(onDelete).toHaveBeenCalledWith(SESSIONS[0]);
    expect(onOpenChange).not.toHaveBeenCalled();
  });

  it("Should reject dismissal while a nested confirmation blocks it", async () => {
    const user = userEvent.setup();
    const shell = createShell();
    const onOpenChange = vi.fn();

    render(
      <OsShellContext.Provider value={shell}>
        <OsSessionsModal
          open
          onOpenChange={onOpenChange}
          dismissalBlocked
          sessions={SESSIONS}
          disconnected={false}
          view={listView()}
          onNewSession={vi.fn()}
          sessionActions={SESSION_ACTIONS}
        />
      </OsShellContext.Provider>
    );

    await user.keyboard("{Escape}");

    expect(onOpenChange).not.toHaveBeenCalled();
    expect(screen.getByTestId("os-sessions-modal")).toBeInTheDocument();
  });
});
