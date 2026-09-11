import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { primarySessionFixture } from "../../testing";
import type { SessionListViewModel } from "../../hooks/use-session-list-view";
import type { SessionPayload } from "../../types";
import { SessionSidebar } from "../session-sidebar";

// Invariant: the shared catalog routes selection gestures to bulk controls without navigation.
// Owner: SessionList through its sidebar host; no existing session-list component suite.
const sessions = [
  {
    ...primarySessionFixture,
    id: "running",
    name: "Running work",
    archived_at: null,
    state: "active" as const,
  },
  {
    ...primarySessionFixture,
    id: "stopped",
    name: "Stopped work",
    archived_at: null,
    state: "stopped" as const,
  },
  {
    ...primarySessionFixture,
    id: "other",
    name: "Other work",
    archived_at: null,
    state: "stopped" as const,
  },
];
const view: SessionListViewModel = {
  scope: "workspace",
  sort: "last_activity",
  archived: false,
  saving: false,
  setScope: vi.fn(),
  setSort: vi.fn(),
  setArchived: vi.fn(),
  workspaceGroups: [],
  collapsedWorkspaceIds: new Set(),
  toggleWorkspace: vi.fn(),
  aggregate: false,
  scopeLabel: "default",
  ownerOf: () => ({ id: "default", name: "default", archived: false }),
};
function renderList() {
  const onSelect = vi.fn();
  const actions = {
    pendingAction: null,
    pendingSessionId: null,
    onStop: vi.fn(),
    onRename: vi.fn(),
    onArchive: vi.fn(),
    onUnarchive: vi.fn(),
    onDelete: vi.fn(),
    onStopMany: vi.fn(),
    onArchiveMany: vi.fn(),
    onUnarchiveMany: vi.fn(),
    onDeleteMany: vi.fn(),
  };
  const props = {
    open: true,
    sessions,
    disconnected: false,
    collapsedThreadIds: [],
    view,
    onToggleThread: vi.fn(),
    onSelectSession: onSelect,
    onNewSession: vi.fn(),
    sessionActions: actions,
  };
  const rendered = render(<SessionSidebar {...props} />);
  return {
    onSelect,
    actions,
    update: (nextSessions: SessionPayload[], nextView = view) =>
      rendered.rerender(<SessionSidebar {...props} sessions={nextSessions} view={nextView} />),
  };
}

describe("SessionList selection", () => {
  it("selects with the checkbox, shows eligible verbs, and keeps filtered selection", async () => {
    const user = userEvent.setup();
    const { onSelect, actions } = renderList();
    const row = screen.getByTestId("session-sidebar-session-running");
    const checkbox = screen.getByRole("checkbox", { name: "Select Running work" });
    expect(row.contains(checkbox)).toBe(false);
    await user.click(checkbox);
    expect(onSelect).not.toHaveBeenCalled();
    expect(row).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("toolbar", { name: "Selected sessions" })).toBeInTheDocument();
    expect(screen.queryByTestId("session-row-actions-running")).not.toBeInTheDocument();
    await user.click(screen.getByTestId("session-sidebar-session-stopped"));
    expect(screen.getByTestId("session-sidebar-selection-count")).toHaveTextContent("2 selected");
    await user.type(screen.getByRole("searchbox", { name: "Filter sessions" }), "Stopped");
    expect(screen.getByTestId("session-sidebar-selection-count")).toHaveTextContent(
      "2 selected· 1 hidden"
    );
    await user.click(screen.getByTestId("session-sidebar-selection-more"));
    expect(await screen.findByTestId("session-sidebar-selection-stop")).toHaveTextContent(
      "1 active"
    );
    expect(screen.getByTestId("session-sidebar-selection-archive")).toHaveTextContent("1 stopped");
    expect(screen.getByTestId("session-sidebar-selection-unarchive")).toHaveAttribute(
      "aria-disabled",
      "true"
    );
    await user.click(screen.getByTestId("session-sidebar-selection-stop"));
    expect(actions.onStopMany).toHaveBeenCalledWith([sessions[0], sessions[1]]);
  });

  it("supports modifier selection, visible ranges, select all, keyboard toggles, and Esc", async () => {
    const user = userEvent.setup();
    const { onSelect, actions } = renderList();
    const first = screen.getByTestId("session-sidebar-session-running");
    fireEvent.click(first, { ctrlKey: true });
    fireEvent.click(screen.getByTestId("session-sidebar-session-other"), { shiftKey: true });
    expect(screen.getByTestId("session-sidebar-selection-count")).toHaveTextContent("3 selected");
    expect(screen.getByTestId("session-sidebar-selection-all")).toHaveAttribute(
      "aria-checked",
      "true"
    );
    first.focus();
    await user.keyboard(" ");
    expect(first).toHaveAttribute("aria-selected", "false");
    fireEvent.keyDown(first, { key: "a", metaKey: true });
    expect(
      within(screen.getByRole("toolbar")).getByTestId("session-sidebar-selection-count")
    ).toHaveTextContent("3 selected");
    fireEvent.keyDown(first, { key: "Backspace", metaKey: true });
    expect(actions.onDeleteMany).toHaveBeenCalled();
    fireEvent.keyDown(first, { key: "Escape" });
    expect(screen.queryByRole("toolbar", { name: "Selected sessions" })).not.toBeInTheDocument();
    expect(screen.getByTestId("session-row-actions-running")).toBeInTheDocument();
    await user.click(first);
    expect(onSelect).toHaveBeenCalledExactlyOnceWith(sessions[0]);
  });
  it.each([{ ctrlKey: true }, { metaKey: true }])(
    "keeps editable filter shortcuts separate from bulk actions (%o)",
    modifier => {
      const { actions } = renderList();
      const filter = screen.getByRole("searchbox", { name: "Filter sessions" });
      expect(fireEvent.keyDown(filter, { key: "a", ...modifier })).toBe(true);
      fireEvent.keyUp(filter, { key: "a", ...modifier });
      expect(screen.queryByRole("toolbar", { name: "Selected sessions" })).not.toBeInTheDocument();
      fireEvent.click(screen.getByTestId("session-sidebar-session-running"), modifier);
      expect(fireEvent.keyDown(filter, { key: "a", ...modifier })).toBe(true);
      fireEvent.keyUp(filter, { key: "a", ...modifier });
      expect(fireEvent.keyDown(filter, { key: "Backspace", ...modifier })).toBe(true);
      fireEvent.keyUp(filter, { key: "Backspace", ...modifier });
      expect(actions.onDeleteMany).not.toHaveBeenCalled();
      expect(screen.getByTestId("session-sidebar-selection-count")).toHaveTextContent("1 selected");
    }
  );

  it("prunes departed rows but retains failures and derives updated eligibility from payloads", async () => {
    const user = userEvent.setup();
    const { update } = renderList();
    fireEvent.click(screen.getByTestId("session-sidebar-session-running"), { metaKey: true });
    fireEvent.click(screen.getByTestId("session-sidebar-session-stopped"));
    const stopped = { ...sessions[0]!, state: "stopped" as const };
    update([stopped, sessions[1]!, sessions[2]!]);
    await user.click(screen.getByTestId("session-sidebar-selection-more"));
    expect(await screen.findByTestId("session-sidebar-selection-stop")).toHaveAttribute(
      "aria-disabled",
      "true"
    );
    expect(screen.getByTestId("session-sidebar-selection-archive")).toHaveTextContent("2 stopped");
    await user.keyboard("{Escape}");
    update([sessions[1]!, sessions[2]!]);
    expect(screen.getByTestId("session-sidebar-selection-count")).toHaveTextContent("1 selected");
    expect(screen.getByTestId("session-sidebar-session-stopped")).toHaveAttribute(
      "aria-selected",
      "true"
    );
    update([sessions[2]!]);
    expect(screen.queryByRole("toolbar", { name: "Selected sessions" })).not.toBeInTheDocument();
  });

  it("clears selection on scope change and offers no selection in workspace groups", () => {
    const { update } = renderList();
    fireEvent.click(screen.getByTestId("session-sidebar-session-running"), { metaKey: true });
    update(sessions, {
      ...view,
      scope: "all-workspaces",
      workspaceGroups: [
        {
          workspaceId: "foreign",
          workspaceName: "Foreign",
          sessions,
          total: 3,
          loading: false,
          failed: false,
          retry: vi.fn(),
        },
      ],
    });
    const row = screen.getByTestId("session-sidebar-session-running");
    fireEvent.keyDown(row, { key: "a", metaKey: true });
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    expect(screen.queryByRole("toolbar", { name: "Selected sessions" })).not.toBeInTheDocument();
    update(sessions);
    expect(screen.getByTestId("session-sidebar-session-running")).toHaveAttribute(
      "aria-selected",
      "false"
    );
  });
});
