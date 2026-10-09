import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest } from "@/test/fetch-test-utils";

import { primarySessionFixture } from "../../testing";
import type { SessionListViewModel } from "../../hooks/use-session-list-view";
import type { SessionPayload } from "../../types";
import { sessionInspectorFocusStore } from "../../hooks/use-session-inspector-focus";
import { sessionInspectorStore } from "../../hooks/use-session-inspector-state";
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
function renderList(
  overrides: {
    sessions?: SessionPayload[];
    revealedSession?: SessionPayload | null;
    collapsedThreadIds?: string[];
  } = {}
) {
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
    sessions: overrides.sessions ?? sessions,
    revealedSession: overrides.revealedSession,
    disconnected: false,
    collapsedThreadIds: overrides.collapsedThreadIds ?? [],
    view,
    onToggleThread: vi.fn(),
    onSelectSession: onSelect,
    onNewSession: vi.fn(),
    sessionActions: actions,
  };
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const tree = (node: React.ReactNode) => (
    <QueryClientProvider client={queryClient}>{node}</QueryClientProvider>
  );
  const rendered = render(tree(<SessionSidebar {...props} />));
  return {
    onSelect,
    actions,
    update: (nextSessions: SessionPayload[], nextView = view) =>
      rendered.rerender(
        tree(<SessionSidebar {...props} sessions={nextSessions} view={nextView} />)
      ),
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
    expect(row).toHaveAttribute("aria-pressed", "true");
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
    expect(first).toHaveAttribute("aria-pressed", "false");
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
      "aria-pressed",
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
    expect(screen.getByTestId("session-sidebar-session-running")).not.toHaveAttribute(
      "aria-pressed"
    );
  });
});

// Invariant: ADR-005 — subagent sessions never take sidebar rows; the parent row carries one
// chip from `subagent_summary`, the viewed subagent is revealed under its parent, plain spawned
// children keep nesting, and a subagent session has no standalone archive verb.
// Owner: SessionList through its sidebar host (same suite as the selection invariants).
describe("SessionList subagents", () => {
  const lineage = (parent: string, spawnRole?: string): SessionPayload["lineage"] => ({
    parent_session_id: parent,
    root_session_id: parent,
    kind: "spawn",
    ...(spawnRole ? { spawn_role: spawnRole } : {}),
    spawn_depth: 1,
    auto_stop_on_parent: true,
    notify_creator: true,
    spawn_budget: { max_children: 0, max_depth: 0, ttl_seconds: 0 },
    permission_policy: { tools: [], skills: [], mcp_servers: [], workspace_paths: [] },
  });
  const row = (id: string, patch: Partial<SessionPayload> = {}): SessionPayload => ({
    ...primarySessionFixture,
    id,
    name: `Session ${id}`,
    archived_at: null,
    state: "active",
    badge: "done",
    ...patch,
  });
  const summary = (live: number, total: number, failed = 0, attention = 0) => ({
    subagent_summary: { live, total, failed, attention, most_urgent: "running" },
  });
  const wireSubagent = (index: number, status: string) => ({
    subagent_id: `sub-${index}`,
    workspace_id: primarySessionFixture.workspace_id,
    parent_session_id: "parent",
    parent_turn_id: "turn-1",
    child_session_id: `child-${index}`,
    origin: "delegated",
    provider_tool_call_id: null,
    title: `Subagent ${index}`,
    role: "",
    status,
    work_state: "working",
    progress: "",
    result: null,
    result_preview: "",
    result_truncated: false,
    error: null,
    delivery: "none",
    depth: 1,
    wait_timed_out: false,
    runtime: { agent: "coder", provider: "claude", model: "", reasoning_effort: "", speed: "" },
    started_at: `2026-10-08T12:${String(index).padStart(2, "0")}:00Z`,
    settled_at: null,
    updated_at: `2026-10-08T12:${String(index).padStart(2, "0")}:00Z`,
  });

  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("UT-W18: reveals only the viewed subagent under its parent and keeps plain children nested", () => {
    const parent = row("parent", summary(1, 3));
    const plainChild = row("plain", { lineage: lineage("parent") });
    const viewed = row("viewed-sub", { lineage: lineage("parent", "subagent") });
    renderList({ sessions: [parent, plainChild], revealedSession: viewed });

    const thread = screen.getByTestId("session-sidebar-thread-parent");
    expect(within(thread).getByTestId("session-sidebar-session-plain")).toBeInTheDocument();
    expect(within(thread).getByTestId("session-sidebar-session-viewed-sub")).toBeInTheDocument();
    expect(screen.getAllByTestId(/^session-sidebar-session-/)).toHaveLength(3);
    // Only the plain child counts toward the thread toggle; the revealed subagent never folds.
    expect(screen.getByTestId("session-sidebar-thread-toggle-parent")).toHaveAccessibleName(
      "Toggle 1 child session"
    );
  });

  it("UT-W18: a parent whose only child is the viewed subagent gets no thread toggle", () => {
    const viewed = row("viewed-sub", { lineage: lineage("parent", "subagent") });
    renderList({ sessions: [row("parent", summary(1, 3))], revealedSession: viewed });

    expect(screen.getByTestId("session-sidebar-session-viewed-sub")).toBeVisible();
    expect(screen.queryByTestId("session-sidebar-thread-toggle-parent")).not.toBeInTheDocument();
  });

  it("UT-W18: keeps the viewed subagent visible when its parent's plain children are folded", () => {
    const parent = row("parent", summary(1, 3));
    const plainChild = row("plain", { lineage: lineage("parent") });
    const viewed = row("viewed-sub", { lineage: lineage("parent", "subagent") });
    renderList({
      sessions: [parent, plainChild],
      revealedSession: viewed,
      collapsedThreadIds: ["parent"],
    });

    expect(screen.getByTestId("session-sidebar-thread-toggle-parent")).toHaveAttribute(
      "aria-expanded",
      "false"
    );
    expect(screen.getByTestId("session-sidebar-session-plain").closest("[inert]")).not.toBeNull();
    expect(screen.getByTestId("session-sidebar-session-viewed-sub").closest("[inert]")).toBeNull();
  });

  it("UT-W18: never promotes a viewed subagent whose parent is off the page", () => {
    const orphan = row("orphan-sub", { lineage: lineage("elsewhere", "subagent") });
    renderList({ sessions: [row("other")], revealedSession: orphan });

    expect(screen.queryByTestId("session-sidebar-session-orphan-sub")).not.toBeInTheDocument();
  });

  it("UT-W17: renders the parent chip and loads a five-row preview on hover", async () => {
    const user = userEvent.setup();
    const parent = row("parent", { badge: "running", ...summary(3, 10) });
    vi.mocked(globalThis.fetch).mockResolvedValue(
      new Response(
        JSON.stringify({
          subagents: Array.from({ length: 10 }, (_, index) =>
            wireSubagent(index, index < 3 ? "running" : "completed")
          ),
          next_cursor: null,
        }),
        { headers: { "Content-Type": "application/json" } }
      )
    );
    renderList({ sessions: [parent] });

    const chip = screen.getByRole("button", { name: "3 of 10 subagents running" });
    expect(chip).toHaveTextContent("3/10");
    expect(chip).toHaveAttribute("data-state", "running");
    expect(globalThis.fetch).not.toHaveBeenCalled();
    await user.hover(chip);

    await waitFor(() => expect(screen.getByText("+5 more")).toBeInTheDocument());
    await expectFetchRequest({
      path: `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/parent/subagents?limit=200`,
    });
    expect(screen.getAllByText(/^Subagent \d$/)).toHaveLength(5);
  });

  it("UT-W17: shows the total alone for failures and hides once everything settled cleanly", () => {
    renderList({
      sessions: [row("failed-parent", summary(0, 6, 2)), row("clean-parent", summary(0, 4))],
    });

    const chip = screen.getByRole("button", { name: "6 subagents, 2 failed" });
    expect(chip).toHaveTextContent(/^6$/);
    expect(chip).toHaveAttribute("data-state", "failed");
    expect(screen.queryByRole("button", { name: /4 subagents/ })).not.toBeInTheDocument();
  });

  it("UT-W14: an idle parent with live subagents reads delegated, a running one does not", () => {
    renderList({
      sessions: [
        row("waiting-parent", { badge: "done", ...summary(2, 2) }),
        row("working-parent", { badge: "running", ...summary(2, 2) }),
      ],
    });

    const waiting = screen.getByTestId("session-sidebar-session-waiting-parent");
    expect(within(waiting).getByRole("img")).toHaveAttribute("data-badge", "delegated");
    expect(screen.getAllByRole("button", { name: "2 of 2 subagents running" })[0]).toHaveAttribute(
      "data-state",
      "delegated"
    );
    const working = screen.getByTestId("session-sidebar-session-working-parent");
    expect(within(working).getByRole("img")).toHaveAttribute("data-badge", "running");
  });

  it("opens the parent with its inspector landing on Subagents when the chip is clicked", async () => {
    const user = userEvent.setup();
    const parent = row("chip-parent", summary(1, 1));
    const { onSelect } = renderList({ sessions: [parent] });

    await user.click(screen.getByRole("button", { name: "1 of 1 subagent running" }));

    expect(onSelect).toHaveBeenCalledExactlyOnceWith(parent);
    expect(sessionInspectorStore.getSnapshot().context.bySession["chip-parent"]).toBe(true);
    expect(sessionInspectorFocusStore.getSnapshot().context.request).toEqual({
      sessionId: "chip-parent",
      section: "subagents",
    });
  });

  it("offers no standalone archive on a subagent session row", async () => {
    const user = userEvent.setup();
    const parent = row("parent", { state: "stopped", badge: "stopped" });
    const viewed = row("viewed-sub", {
      state: "stopped",
      badge: "stopped",
      lineage: lineage("parent", "subagent"),
    });
    renderList({ sessions: [parent], revealedSession: viewed });

    await user.click(screen.getByTestId("session-row-actions-viewed-sub"));
    expect(await screen.findByTestId("session-row-delete-viewed-sub")).toBeInTheDocument();
    expect(screen.queryByTestId("session-row-archive-viewed-sub")).not.toBeInTheDocument();
    await user.keyboard("{Escape}");
    await user.click(screen.getByTestId("session-row-actions-parent"));
    expect(await screen.findByTestId("session-row-archive-parent")).toBeInTheDocument();
  });
});
