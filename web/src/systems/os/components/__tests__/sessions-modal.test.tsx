// Suite: OS sessions modal
// Invariant: the modal filters catalog truth, retains group state, and dismisses via Dialog.
// Boundary IN: OsSessionsModal, OS controller, and routing coordinator.
// Boundary OUT: session catalog transport and full browser window journeys.
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { SessionLifecycleActionHandlers, SessionPayload } from "@/systems/session";

import { OsShellContext, type OsShellHandle } from "../../contexts/os-shell-context";
import { WindowManagerRuntime } from "../../runtime/window-manager-runtime";
import { RoutingCoordinator, type OsRouterPort } from "../../lib/routing-coordinator";
import { OsSessionsModal } from "../sessions-modal";

function session(overrides: Partial<SessionPayload> = {}): SessionPayload {
  return {
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
  onStop: () => {},
  onUnarchive: () => {},
};

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
  archivedSessions: SessionPayload[] = [],
  archivedTotal?: number
) {
  return render(
    <OsShellContext.Provider value={shell}>
      <OsSessionsModal
        open={open}
        onOpenChange={() => {}}
        sessions={SESSIONS}
        archivedSessions={archivedSessions}
        archivedTotal={archivedTotal}
        disconnected={false}
        sessionActions={SESSION_ACTIONS}
      />
    </OsShellContext.Provider>
  );
}

describe("OsSessionsModal", () => {
  afterEach(() => {
    for (const manager of managers.splice(0)) manager.destroy();
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

  it("Should retain an agent collapse after the modal remounts (UT-068)", async () => {
    const user = userEvent.setup();
    const shell = createShell();
    const first = renderModal(shell);

    await user.click(screen.getByRole("button", { name: "Show all sessions" }));
    const group = screen.getByRole("button", { name: /codex/i, expanded: true });
    await user.click(group);
    expect(group).toHaveAttribute("aria-expanded", "false");
    expect(shell.manager.getState().railCollapsedAgentIds).toEqual(["codex"]);

    first.unmount();
    renderModal(shell);
    await user.click(screen.getByRole("button", { name: "Show all sessions" }));
    expect(screen.getByRole("button", { name: /codex/i, expanded: false })).toHaveAttribute(
      "aria-expanded",
      "false"
    );
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
            archivedSessions={[]}
            archivedTotal={0}
            disconnected={false}
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

  it("Should reveal archived sessions only on request and offer unarchive", async () => {
    const user = userEvent.setup();
    const shell = createShell();
    renderModal(
      shell,
      true,
      [
        session({
          id: "session-archived",
          name: "Archived audit",
          state: "stopped",
          badge: "stopped",
          archived_at: "2026-08-04T12:00:00Z",
        }),
      ],
      4
    );

    const archivedDisclosure = screen.getByRole("button", { name: "Archived sessions (4)" });
    expect(archivedDisclosure).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByTestId("os-sessions-modal-session-session-archived")).toBeNull();
    expect(screen.getByRole("button", { name: "Show all sessions" })).toBeInTheDocument();

    await user.click(archivedDisclosure);
    expect(screen.getByTestId("os-sessions-modal-session-session-archived")).toHaveTextContent(
      "Archived"
    );
    fireEvent.click(screen.getByTestId("session-row-actions-session-archived"));
    expect(screen.getByTestId("session-row-unarchive-session-archived")).toBeInTheDocument();
    expect(screen.queryByTestId("session-row-archive-session-archived")).toBeNull();
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
          archivedSessions={[]}
          archivedTotal={0}
          disconnected={false}
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
          archivedSessions={[]}
          archivedTotal={0}
          disconnected={false}
          sessionActions={SESSION_ACTIONS}
        />
      </OsShellContext.Provider>
    );

    await user.keyboard("{Escape}");

    expect(onOpenChange).not.toHaveBeenCalled();
    expect(screen.getByTestId("os-sessions-modal")).toBeInTheDocument();
  });
});
