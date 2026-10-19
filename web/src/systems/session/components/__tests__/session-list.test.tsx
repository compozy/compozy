import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  act,
  fireEvent,
  render,
  renderHook,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { formatAbsoluteTime } from "@compozy/ui";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest } from "@/test/fetch-test-utils";
import { allModelsListOptions, type ProviderModelPayload } from "@/systems/model-catalog";
import { providerListOptions, type ProviderSummary } from "@/systems/providers";

import { primaryAgentFixture } from "@/systems/agent/testing";
import { agentsListOptions } from "@/systems/agent";
import { sessionFoldersOptions } from "../../lib/session-catalog-options";

import { primarySessionFixture } from "../../testing";
import type { SessionListViewModel } from "../../hooks/use-session-list-view";
import type { SessionPayload } from "../../types";
import { sessionInspectorFocusStore } from "../../hooks/use-session-inspector-focus";
import { sessionInspectorStore } from "../../hooks/use-session-inspector-state";
import { useSubagentRoster } from "../../hooks/use-subagent-roster";
import { sessionKeys } from "../../lib/query-keys";
import { subagentViewFromPayload } from "../../lib/subagent-payload";
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
function catalogProvider(name: string, displayName: string): ProviderSummary {
  return {
    name,
    display_name: displayName,
    default: false,
    auth_status: {
      mode: "native_cli",
      env_policy: "inherit",
      home_policy: "operator",
      state: "authenticated",
      login: { configured: true, presence: "present" },
    },
  };
}
function catalogModel(provider: string, id: string, displayName: string): ProviderModelPayload {
  return {
    provider_id: provider,
    model_id: id,
    display_name: displayName,
    availability_state: "available_live",
    available: true,
    curated: true,
    deprecated: false,
    featured: false,
    hidden: false,
    stale: false,
    sources: [],
  };
}
/** The rail lives in a session window whose composer has already read the runtime catalog. */
function seedRuntimeCatalog(queryClient: QueryClient) {
  queryClient.setQueryData(providerListOptions().queryKey, {
    providers: [catalogProvider("codex", "Codex"), catalogProvider("claude", "Claude Code")],
  });
  queryClient.setQueryData(allModelsListOptions({ view: "all", includeStale: true }).queryKey, {
    models: [
      catalogModel("codex", "gpt-6.1-sol", "GPT-6.1 Sol"),
      catalogModel("claude", "claude-opus-5-5", "Claude Opus 5.5"),
    ],
  });
}
function renderList(
  overrides: {
    onNewSession?: (options?: { folderId?: string }) => void;
    sessions?: SessionPayload[];
    revealedSession?: SessionPayload | null;
    currentSessionId?: string;
    collapsedThreadIds?: string[];
    view?: SessionListViewModel;
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
    currentSessionId: overrides.currentSessionId,
    disconnected: false,
    collapsedThreadIds: overrides.collapsedThreadIds ?? [],
    view: overrides.view ?? view,
    onToggleThread: vi.fn(),
    onSelectSession: onSelect,
    onNewSession: overrides.onNewSession ?? vi.fn(),
    sessionActions: actions,
  };
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  seedRuntimeCatalog(queryClient);
  const tree = (node: React.ReactNode) => (
    <QueryClientProvider client={queryClient}>{node}</QueryClientProvider>
  );
  const rendered = render(tree(<SessionSidebar {...props} />));
  return {
    onSelect,
    actions,
    queryClient,
    update: (nextSessions: SessionPayload[], nextView = view) =>
      rendered.rerender(
        tree(<SessionSidebar {...props} sessions={nextSessions} view={nextView} />)
      ),
  };
}

describe("SessionList selection", () => {
  // Invariant: hovering a row swaps its compact time for the actions, so the row
  // target itself still reveals the full timestamp to mouse users.
  it("keeps the full timestamp on the row target while hover hides the compact time", () => {
    renderList();
    const running = sessions.find(session => session.id === "running")!;
    expect(screen.getByTestId("session-sidebar-session-running")).toHaveAttribute(
      "title",
      formatAbsoluteTime(running.updated_at)
    );
  });

  it("Should offer all five sort choices and send the selected stable order", async () => {
    const user = userEvent.setup();
    const setSort = vi.fn();
    renderList({ view: { ...view, sort: "created", setSort } });
    await user.click(screen.getByRole("button", { name: "Sort sessions" }));
    const choices = await screen.findAllByRole("menuitemradio");
    expect(choices.map(choice => choice.textContent)).toEqual([
      "Created",
      "Name",
      "Last activity",
      "Attention first",
      "Manual",
    ]);
    expect(screen.getByRole("menuitemradio", { name: "Created" })).toHaveAttribute(
      "aria-checked",
      "true"
    );
    await user.click(screen.getByRole("menuitemradio", { name: "Name" }));
    expect(setSort).toHaveBeenCalledWith("name");
  });

  it.each([
    ["created", false],
    ["name", false],
    ["created", true],
    ["name", true],
  ] as const)(
    "Should preserve %s catalog order for nested running children (subagents: %s)",
    (sort, subagents) => {
      const parent = { ...sessions[0]!, id: "parent", name: "Parent work" };
      const lineage = {
        ...(subagents ? { spawn_role: "subagent" } : {}),
        parent_session_id: "parent",
        root_session_id: "parent",
        spawn_depth: 1,
        auto_stop_on_parent: false,
        notify_creator: false,
        spawn_budget: { max_children: 0, max_depth: 0, ttl_seconds: 0 },
        permission_policy: { tools: [], skills: [], mcp_servers: [], workspace_paths: [] },
      };
      const children = [
        { ...sessions[0]!, id: "child-b", name: "Beta work", lineage },
        { ...sessions[0]!, id: "child-a", name: "Alpha work", lineage },
      ];
      const ordered =
        sort === "name" ? [parent, ...[...children].reverse()] : [parent, ...children];
      const stableView = { ...view, sort, ...(subagents ? { search: "work" } : {}) };
      const { update } = renderList({ sessions: ordered, view: stableView });
      const rowIds = () =>
        screen.getAllByTestId(/^session-sidebar-session-/).map(row => row.dataset.testid);
      const before = rowIds();
      expect(before).toEqual(ordered.map(session => `session-sidebar-session-${session.id}`));
      update(
        ordered.map((session, index) => ({
          ...session,
          updated_at: `2026-10-16T15:0${index}:00Z`,
          badge: index === 1 ? "waiting-for-input" : "running",
        })),
        stableView
      );
      expect(rowIds()).toEqual(before);
    }
  );

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
  const lineage = (parent: string, spawnRole?: string): NonNullable<SessionPayload["lineage"]> => ({
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
    created_at: `2026-10-08T12:${String(index).padStart(2, "0")}:00Z`,
    updated_at: `2026-10-08T12:${String(index).padStart(2, "0")}:00Z`,
  });

  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("Should keep a viewed verification judge out of default rows and child counts", () => {
    const judge = row("judge", {
      type: "system",
      lineage: { ...lineage("parent"), kind: "provenance", auto_stop_on_parent: false },
    });
    renderList({ sessions: [row("parent")], revealedSession: judge, currentSessionId: judge.id });
    expect(screen.getByTestId("session-sidebar-session-parent")).toBeVisible();
    expect(screen.queryByTestId("session-sidebar-session-judge")).not.toBeInTheDocument();
    expect(screen.queryByTestId("session-sidebar-thread-toggle-parent")).not.toBeInTheDocument();
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

  const detailResponse = (sessionsById: Record<string, SessionPayload>) =>
    vi.mocked(globalThis.fetch).mockImplementation(async input => {
      const id = new URL((input as Request).url).pathname.split("/").at(-1) ?? "";
      const session = sessionsById[id];
      return session
        ? new Response(JSON.stringify({ session }), {
            headers: { "Content-Type": "application/json" },
          })
        : new Response(JSON.stringify({ error: "not found" }), { status: 404 });
    });

  // m19 / S9: the backend pages search matches only; a matched subagent whose parent did not
  // match is nested under that parent, loaded as a non-matching context row — never a root.
  it("UT-W18: nests a search-matched subagent under its loaded, non-matching parent", async () => {
    const offPageParent = row("parent-off", { name: "Ship checkout v2" });
    const matched = row("matched-sub", {
      name: "Audit payment webhooks",
      lineage: lineage("parent-off", "subagent"),
    });
    detailResponse({ "parent-off": offPageParent });
    renderList({ sessions: [matched], view: { ...view, search: "webhook", setSearch: vi.fn() } });

    // While the parent loads, the match is held back rather than shown as a root.
    expect(screen.queryByTestId("session-sidebar-thread-matched-sub")).not.toBeInTheDocument();
    const thread = await screen.findByTestId("session-sidebar-thread-parent-off");
    expect(within(thread).getByTestId("session-sidebar-session-parent-off")).toBeVisible();
    expect(within(thread).getByTestId("session-sidebar-session-matched-sub")).toBeVisible();
    await expectFetchRequest({
      path: `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/parent-off?include_health=true&all_profiles=true`,
    });
  });

  it("UT-W18: reveals a depth-2 subagent under its nearest on-page ancestor", async () => {
    const parent = row("parent", summary(1, 1));
    const middle = row("mid-sub", { lineage: lineage("parent", "subagent") });
    const viewed = row("grand-sub", { lineage: lineage("mid-sub", "subagent") });
    detailResponse({ "mid-sub": middle });
    renderList({ sessions: [parent], revealedSession: viewed });

    const thread = await screen.findByTestId("session-sidebar-thread-parent");
    await waitFor(() =>
      expect(within(thread).getByTestId("session-sidebar-session-grand-sub")).toBeVisible()
    );
    expect(within(thread).getByTestId("session-sidebar-session-mid-sub")).toBeVisible();
    expect(screen.queryByTestId("session-sidebar-thread-toggle-parent")).not.toBeInTheDocument();
  });

  // Review round 2: a deleted ancestor no longer hides its descendant for good.
  it("UT-W18: shows a viewed subagent whose parent was deleted, labelled as such", async () => {
    const viewed = row("orphan-sub", { lineage: lineage("deleted-parent", "subagent") });
    detailResponse({});
    renderList({ sessions: [row("other")], revealedSession: viewed });

    const orphan = await screen.findByTestId("session-sidebar-session-orphan-sub");
    expect(orphan).toHaveTextContent("Subagent of a deleted session");
    expect(screen.getByTestId("session-sidebar-session-other")).not.toHaveTextContent(
      "Subagent of a deleted session"
    );
  });

  it("keeps one cache subscription per chip across list re-renders", () => {
    const parent = row("parent", summary(1, 2));
    const { queryClient, update } = renderList({ sessions: [parent] });
    const subscribe = vi.spyOn(queryClient.getQueryCache(), "subscribe");

    update([{ ...parent, name: "Renamed parent" }]);
    update([{ ...parent, name: "Renamed again" }]);

    expect(subscribe).not.toHaveBeenCalled();
  });

  it("UT-W18: never promotes a viewed subagent whose parent is off the page", () => {
    const orphan = row("orphan-sub", { lineage: lineage("elsewhere", "subagent") });
    renderList({ sessions: [row("other")], revealedSession: orphan });

    expect(screen.queryByTestId("session-sidebar-session-orphan-sub")).not.toBeInTheDocument();
  });

  it("UT-W17: renders the parent chip and loads a five-row preview on hover", async () => {
    const user = userEvent.setup();
    const parent = row("parent", { badge: "running", ...summary(3, 10) });
    // A further page exists; the five-row preview still reads only the first (review n6).
    vi.mocked(globalThis.fetch).mockImplementation(
      async () =>
        new Response(
          JSON.stringify({
            subagents: Array.from({ length: 10 }, (_, index) =>
              wireSubagent(index, index < 3 ? "running" : "completed")
            ),
            next_cursor: "cursor-page-2",
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
      path: `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/parent/subagents?all_profiles=true&limit=200`,
    });
    expect(screen.getAllByText(/^Subagent \d$/)).toHaveLength(5);
    expect(globalThis.fetch).toHaveBeenCalledTimes(1);
  });

  // M9: an open parent's stream-fed roster feeds the preview; the list route stays cold.
  // Invariant: only this parent's workspace-qualified roster updates its live
  // preview; removing its last stream observer restores the authoritative list.
  // Owning layer: roster/query integration; canonical suite: this sidebar suite.
  it("Should keep a live preview scoped and fall back when its stream owner leaves", async () => {
    const user = userEvent.setup();
    const parent = row("parent", { badge: "running", ...summary(1, 1) });
    vi.mocked(globalThis.fetch).mockResolvedValue(
      new Response(
        JSON.stringify({
          subagents: [{ ...wireSubagent(1, "running"), title: "Cold list row" }],
        }),
        { headers: { "Content-Type": "application/json" } }
      )
    );
    const { queryClient } = renderList({ sessions: [parent] });
    const workspaceId = primarySessionFixture.workspace_id ?? "";
    const roster = (title: string) => ({
      rows: [subagentViewFromPayload({ ...wireSubagent(1, "running"), title } as never)],
      staleIds: new Set<string>(),
    });
    queryClient.setQueryData(sessionKeys.subagentRoster(workspaceId, "parent"), roster("Live row"));
    const owner = renderHook(() => useSubagentRoster(workspaceId, "parent"), {
      wrapper: ({ children }) => (
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      ),
    });
    await user.hover(screen.getByRole("button", { name: "1 of 1 subagent running" }));
    expect(await screen.findByText("Live row")).toBeInTheDocument();
    act(() => {
      queryClient.setQueryData(
        sessionKeys.subagentRoster("another-workspace", "parent"),
        roster("Foreign row")
      );
      queryClient.setQueryData(
        sessionKeys.subagentRoster(workspaceId, "parent"),
        roster("Updated live row")
      );
    });
    expect(await screen.findByText("Updated live row")).toBeInTheDocument();
    expect(screen.queryByText("Foreign row")).not.toBeInTheDocument();
    expect(globalThis.fetch).not.toHaveBeenCalled();

    owner.unmount();
    expect(await screen.findByText("Cold list row")).toBeInTheDocument();
    expect(globalThis.fetch).toHaveBeenCalledTimes(1);
  });

  it("UT-W17: previews from the parent's live roster while its thread holds the stream", async () => {
    const user = userEvent.setup();
    const parent = row("parent", { badge: "running", ...summary(1, 1) });
    const { queryClient } = renderList({ sessions: [parent] });
    const workspaceId = primarySessionFixture.workspace_id ?? "";
    queryClient.setQueryData(sessionKeys.subagentRoster(workspaceId, "parent"), {
      rows: [
        subagentViewFromPayload({ ...wireSubagent(1, "running"), title: "Streamed row" } as never),
      ],
      staleIds: new Set<string>(),
    });
    renderHook(() => useSubagentRoster(workspaceId, "parent"), {
      wrapper: ({ children }) => (
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      ),
    });

    await user.hover(screen.getByRole("button", { name: "1 of 1 subagent running" }));

    await waitFor(() => expect(screen.getByText("Streamed row")).toBeInTheDocument());
    expect(globalThis.fetch).not.toHaveBeenCalled();
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

// Invariant: every row, nested children included, names the runtime it is bound to in the
// runtime catalog's display names (wire ids only where the catalog has none), repaints when a
// re-read reports another runtime, shows no mark before the first bind, and adds no tab stop.
describe("SessionList provider marks", () => {
  const runtime = (provider: string, model?: string): SessionPayload["runtime"] => ({
    status: "ready",
    transition: "initial_bind",
    effective: { provider, ...(model ? { model } : {}) },
    selection_revision: 0,
  });
  const row = (id: string, patch: Partial<SessionPayload> = {}): SessionPayload => ({
    ...primarySessionFixture,
    id,
    name: `Session ${id}`,
    archived_at: null,
    state: "active",
    badge: "running",
    ...patch,
  });
  const rowButton = (id: string) => screen.getByTestId(`session-sidebar-session-${id}`);

  it("names root and nested rows and identifies an unselected runtime", () => {
    renderList({
      sessions: [
        row("root", { runtime: runtime("codex", "gpt-6.1-sol") }),
        row("child", {
          runtime: runtime("claude"),
          lineage: {
            parent_session_id: "root",
            root_session_id: "root",
            spawn_depth: 1,
            auto_stop_on_parent: false,
            notify_creator: true,
            spawn_budget: { max_children: 0, max_depth: 0, ttl_seconds: 0 },
            permission_policy: { tools: [], skills: [], mcp_servers: [], workspace_paths: [] },
          },
        }),
        row("unbound", { runtime: { status: "unbound", selection_revision: 0 } }),
      ],
    });

    expect(rowButton("root")).toHaveAccessibleName(/Session root\s*Codex · GPT-6\.1 Sol/);
    const rootMark = screen.getByTestId("session-provider-mark-root");
    expect(rootMark).toHaveAttribute("data-kind", "codex");
    expect(rootMark).toHaveAttribute("aria-hidden", "true");
    expect(
      within(screen.getByTestId("session-sidebar-thread-root")).getByTestId(
        "session-provider-mark-child"
      )
    ).toHaveAttribute("data-kind", "claude");
    expect(rowButton("child")).toHaveAccessibleName(/Session child\s*Claude Code/);
    expect(screen.getByTestId("session-provider-mark-unbound")).toHaveAttribute(
      "data-kind",
      "unknown"
    );
    expect(rowButton("unbound")).toHaveAccessibleName(
      /Session unbound\s*Runtime not selected frontend-engineer-agent/
    );
  });

  it.each(["released", "detached", "stopped"])(
    "shows the recorded runtime for %s sessions",
    state => {
      renderList({
        sessions: [
          row(state, {
            state: state === "stopped" ? "stopped" : "active",
            runtime: {
              status: "unbound",
              selection_revision: 1,
              generation: 1,
              last_known: { provider: "codex", model: "gpt-6.1-sol" },
              selected: { provider: "claude" },
            },
          }),
        ],
      });
      expect(screen.getByTestId(`session-provider-mark-${state}`)).toHaveAttribute(
        "data-kind",
        "codex"
      );
      expect(rowButton(state)).toHaveAccessibleName(/Codex · GPT-6\.1 Sol frontend-engineer-agent/);
    }
  );

  it("uses the selected runtime before defaults and resolves never-bound defaults in the owner scope", async () => {
    const unbound = row("fresh", {
      workspace_id: "ws",
      profile_name: "marketing",
      runtime: { status: "unbound", selection_revision: 0 },
    });
    const { queryClient } = renderList({
      sessions: [
        unbound,
        row("selected", {
          runtime: {
            status: "unbound",
            selection_revision: 1,
            last_known: { provider: "codex", model: "gpt-5.6-sol" },
            selected: { provider: "claude", model: "claude-opus-5-5" },
          },
        }),
      ],
    });
    act(() =>
      queryClient.setQueryData(agentsListOptions("ws", "marketing").queryKey, [
        {
          ...primaryAgentFixture,
          name: unbound.agent_name,
          effective_runtime: {
            provider: "codex",
            model: "gpt-6.1-sol",
            sources: { provider: "project_default", model: "provider_default" },
          },
        },
      ])
    );
    await waitFor(() => expect(rowButton("fresh")).toHaveAccessibleName(/Codex · GPT-6\.1 Sol/));
    expect(rowButton("selected")).toHaveAccessibleName(/Claude Code · Claude Opus 5.5/);
  });

  // Invariant: every child retains its runtime and agent, even when they match its root.
  it("shows a child's runtime even when it repeats its root", () => {
    const lineage = (parent: string): SessionPayload["lineage"] => ({
      parent_session_id: parent,
      root_session_id: parent,
      spawn_depth: 1,
      auto_stop_on_parent: false,
      notify_creator: true,
      spawn_budget: { max_children: 0, max_depth: 0, ttl_seconds: 0 },
      permission_policy: { tools: [], skills: [], mcp_servers: [], workspace_paths: [] },
    });
    renderList({
      sessions: [
        row("root", { runtime: runtime("codex", "gpt-6.1-sol") }),
        row("same", { runtime: runtime("codex", "gpt-6.1-sol"), lineage: lineage("root") }),
        row("other", { runtime: runtime("claude"), lineage: lineage("root") }),
        row("model", { runtime: runtime("codex", "gpt-6.1-mini"), lineage: lineage("root") }),
      ],
    });

    expect(screen.getByTestId("session-provider-mark-same")).toHaveAttribute("data-kind", "codex");
    expect(rowButton("same")).toHaveAccessibleName(
      /Session same\s*Codex · GPT-6\.1 Sol frontend-engineer-agent/
    );
    expect(screen.getByTestId("session-provider-mark-other")).toHaveAttribute(
      "data-kind",
      "claude"
    );
    expect(screen.getByTestId("session-provider-mark-model")).toHaveAttribute("data-kind", "codex");
  });

  // Invariant: the mark answers mouse hover with its runtime and, lifted above the
  // row's stretched target, still navigates to the row when clicked.
  it("opens the runtime tooltip on hover and navigates when the mark is clicked", async () => {
    const user = userEvent.setup();
    const { onSelect } = renderList({
      sessions: [row("root", { runtime: runtime("codex", "gpt-6.1-sol") })],
    });

    const mark = screen.getByTestId("session-provider-mark-root");
    expect(screen.queryByText("Codex · GPT-6.1 Sol")).not.toBeInTheDocument();
    await user.hover(mark);
    expect(await screen.findByText("Codex · GPT-6.1 Sol")).toBeInTheDocument();
    await user.click(mark);
    expect(onSelect).toHaveBeenCalledWith(expect.objectContaining({ id: "root" }));
  });

  // Invariant: the aggregate tags a thread's root with its owner and a child only
  // when the child belongs to another profile.
  it("tags a child with its owner only when it differs from its root's", () => {
    const lineage = (parent: string): SessionPayload["lineage"] => ({
      parent_session_id: parent,
      root_session_id: parent,
      spawn_depth: 1,
      auto_stop_on_parent: false,
      notify_creator: true,
      spawn_budget: { max_children: 0, max_depth: 0, ttl_seconds: 0 },
      permission_policy: { tools: [], skills: [], mcp_servers: [], workspace_paths: [] },
    });
    const owners = {
      default: { id: "default", name: "Personal", archived: false },
      design: { id: "design", name: "Design", archived: false },
    };
    renderList({
      sessions: [
        row("root"),
        row("mine", { lineage: lineage("root") }),
        row("theirs", { lineage: lineage("root"), profile_id: "design" }),
      ],
      view: {
        ...view,
        aggregate: true,
        ownerOf: session => (session.profile_id === "design" ? owners.design : owners.default),
      },
    });

    const tags = within(screen.getByTestId("session-sidebar-thread-root")).getAllByTestId(
      "profile-owner-tag"
    );
    expect(tags.map(tag => tag.getAttribute("aria-label"))).toEqual(["Personal", "Design"]);
  });

  it("keeps the wire ids and the neutral glyph for a runtime the catalog does not name", () => {
    const warn = vi.spyOn(console, "warn");
    renderList({ sessions: [row("custom", { runtime: runtime("acme-agent", "m-1") })] });

    expect(rowButton("custom")).toHaveAccessibleName(/acme-agent · m-1/);
    const mark = screen.getByTestId("session-provider-mark-custom");
    expect(mark).toHaveAttribute("data-kind", "acme-agent");
    expect(mark.querySelector("svg")).not.toBeNull();
    expect(warn).not.toHaveBeenCalled();
    warn.mockRestore();
  });

  it("repaints the mark when a re-read reports another runtime", () => {
    const before = row("switched", { runtime: runtime("codex", "gpt-6.1-sol") });
    const { update } = renderList({ sessions: [before] });
    expect(rowButton("switched")).toHaveAccessibleName(/Codex · GPT-6\.1 Sol/);

    update([{ ...before, runtime: runtime("claude", "claude-opus-5-5") }]);

    expect(rowButton("switched")).toHaveAccessibleName(/Claude Code · Claude Opus 5\.5/);
    expect(screen.getByTestId("session-provider-mark-switched")).toHaveAttribute(
      "data-kind",
      "claude"
    );
  });

  it("reveals the runtime on keyboard focus, closes on Escape, and adds no tab stop", async () => {
    const user = userEvent.setup();
    renderList({ sessions: [row("keyed", { runtime: runtime("codex", "gpt-6.1-sol") })] });

    await user.click(screen.getByRole("searchbox", { name: "Filter sessions" }));
    while (document.activeElement !== rowButton("keyed")) await user.tab();
    const tooltip = { selector: '[data-slot="tooltip-content"]' };
    expect(await screen.findByText("Codex · GPT-6.1 Sol", tooltip)).toBeInTheDocument();

    await user.keyboard("{Escape}");
    await waitFor(() =>
      expect(screen.queryByText("Codex · GPT-6.1 Sol", tooltip)).not.toBeInTheDocument()
    );

    // The next stop is the row's own selection checkbox: the glyph inside the row is not one.
    await user.tab();
    expect(document.activeElement).toBe(
      screen.getByRole("checkbox", { name: "Select Session keyed" })
    );
  });
});

describe("SessionList folders and manual order", () => {
  // Invariant: in a workspace list, folders group root sessions, the (+) menu and
  // context menus reach every layout verb, and drags resolve to the same verbs.
  const folderA = {
    id: "fa",
    name: "Alpha",
    workspace_id: "ws",
    profile_id: "default",
    position: 0,
    session_count: 1,
    created_at: "2026-10-17T00:00:00Z",
    updated_at: "2026-10-17T00:00:00Z",
  };
  const folderB = { ...folderA, id: "fb", name: "Beta", position: 1, session_count: 0 };
  const layoutRows = [
    { ...sessions[0]!, id: "r1", name: "Root one" },
    { ...sessions[1]!, id: "f1", name: "Filed one", folder_id: "fa" },
    { ...sessions[2]!, id: "r2", name: "Root two" },
  ];
  function fakeLayout(overrides: Partial<NonNullable<SessionListViewModel["layout"]>> = {}) {
    return {
      folders: [folderA, folderB],
      createFolder: vi.fn(async () => folderA),
      renameFolder: vi.fn(async () => undefined),
      deleteFolder: vi.fn(async () => undefined),
      moveFolder: vi.fn(),
      reorderFolders: vi.fn(),
      moveSession: vi.fn(),
      reorderSessions: vi.fn(),
      ...overrides,
    };
  }
  function renderLayout(sort: SessionListViewModel["sort"] = "manual") {
    const layout = fakeLayout();
    const onNewSession = vi.fn();
    renderList({ sessions: layoutRows, view: { ...view, sort, layout }, onNewSession });
    return { layout, onNewSession };
  }

  it("Should keep folders and moves inside their owning profile in the aggregate", async () => {
    const groups = [
      {
        owner: { id: "default", name: "default", archived: false },
        workspaceId: "ws",
        folders: [folderA],
        canCreateSession: true,
      },
      {
        owner: { id: "other", name: "other", archived: false },
        workspaceId: "ws",
        folders: [{ ...folderB, profile_id: "other" }],
        canCreateSession: false,
      },
    ];
    const { queryClient } = renderList({
      sessions: [
        { ...layoutRows[1]!, profile_id: "default", profile_name: "default" },
        {
          ...layoutRows[0]!,
          id: "foreign",
          folder_id: "fb",
          profile_id: "other",
          profile_name: "other",
        },
      ],
      view: {
        ...view,
        aggregate: true,
        profileFolderGroups: groups,
        ownerOf: session => groups.find(group => group.owner.id === session.profile_id)!.owner,
      },
    });
    act(() => {
      for (const group of groups)
        queryClient.setQueryData(
          sessionFoldersOptions({ workspaceId: "ws", profile: group.owner.name }).queryKey,
          group.folders
        );
    });
    const alpha = await screen.findByTestId("session-sidebar-folder-fa");
    const beta = await screen.findByTestId("session-sidebar-folder-fb");
    expect(within(alpha).getByTestId("session-sidebar-session-f1")).toBeInTheDocument();
    expect(within(beta).getByTestId("session-sidebar-session-foreign")).toBeInTheDocument();
    expect(screen.getByTestId("session-sidebar-folder-fb-new-session")).toBeDisabled();
    const source = screen.getByTestId("session-sidebar-session-f1");
    fireEvent.dragStart(source, { dataTransfer: { setData: vi.fn() } });
    fireEvent.drop(screen.getByTestId("session-sidebar-folder-fb-header"));
    expect(within(alpha).getByTestId("session-sidebar-session-f1")).toBeInTheDocument();
    fireEvent.dragEnd(source);
  });

  it("Should turn (+) into a New session / New folder menu and create a named folder", async () => {
    const user = userEvent.setup();
    const { layout, onNewSession } = renderLayout();
    await user.click(screen.getByTestId("session-sidebar-new"));
    const items = await screen.findAllByRole("menuitem");
    expect(items.map(item => item.textContent)).toEqual(["New session", "New folder"]);
    await user.click(screen.getByRole("menuitem", { name: "New session" }));
    expect(onNewSession).toHaveBeenCalledWith();

    await user.click(screen.getByTestId("session-sidebar-new"));
    await user.click(await screen.findByRole("menuitem", { name: "New folder" }));
    await user.type(await screen.findByTestId("session-folder-name"), "  Research  ");
    await user.click(screen.getByTestId("session-folder-name-confirm"));
    expect(layout.createFolder).toHaveBeenCalledWith("Research");
  });

  it("Should group foldered sessions, count them, collapse, and start a session inside", async () => {
    const user = userEvent.setup();
    const { onNewSession } = renderLayout("created");
    const alpha = screen.getByTestId("session-sidebar-folder-fa");
    expect(within(alpha).getByTestId("session-sidebar-session-f1")).toBeInTheDocument();
    expect(screen.getByTestId("session-sidebar-folder-fa-count")).toHaveTextContent("1");
    expect(screen.getByTestId("session-sidebar-folder-fb-count")).toHaveTextContent("0");
    const root = screen.getByTestId("session-sidebar-root");
    expect(
      within(root)
        .getAllByTestId(/^session-sidebar-session-/)
        .map(row => row.dataset.testid)
    ).toEqual(["session-sidebar-session-r1", "session-sidebar-session-r2"]);

    await user.click(screen.getByTestId("session-sidebar-folder-fb-new-session"));
    expect(onNewSession).toHaveBeenCalledWith({ folderId: "fb" });

    await user.click(screen.getByTestId("session-sidebar-folder-fa-toggle"));
    expect(screen.queryByTestId("session-sidebar-session-f1")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-sidebar-folder-fa-toggle")).toHaveAttribute(
      "aria-expanded",
      "false"
    );
    await user.click(screen.getByTestId("session-sidebar-folder-fa-toggle"));
    expect(screen.getByTestId("session-sidebar-session-f1")).toBeInTheDocument();
  });

  it("Should offer the row actions plus folder moves and Move up/down on right-click", async () => {
    const user = userEvent.setup();
    const { layout } = renderLayout();
    fireEvent.contextMenu(screen.getByTestId("session-sidebar-session-f1"));
    const menu = await screen.findByRole("menu");
    expect(within(menu).getByRole("menuitem", { name: "Rename session" })).toBeInTheDocument();
    expect(within(menu).getByRole("menuitem", { name: "Delete session" })).toBeInTheDocument();
    expect(within(menu).getByRole("menuitem", { name: "Move to folder" })).toBeInTheDocument();
    await user.click(within(menu).getByRole("menuitem", { name: "Remove from folder" }));
    expect(layout.moveSession).toHaveBeenCalledWith("f1", null);

    fireEvent.contextMenu(screen.getByTestId("session-sidebar-session-r1"));
    const rootMenu = await screen.findByRole("menu");
    expect(within(rootMenu).getByRole("menuitem", { name: "Move up" })).toHaveAttribute(
      "aria-disabled",
      "true"
    );
    await user.click(within(rootMenu).getByRole("menuitem", { name: "Move down" }));
    expect(layout.reorderSessions).toHaveBeenCalledWith(null, ["r2", "r1"]);
  });

  it("Should say the folders failed to load and retry from the list", async () => {
    const user = userEvent.setup();
    const retry = vi.fn();
    renderList({
      sessions: layoutRows,
      view: { ...view, sort: "manual", layoutError: { retry } },
    });
    expect(screen.getByText("Couldn’t load folders.")).toBeInTheDocument();
    expect(screen.queryByTestId("session-sidebar-root")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(retry).toHaveBeenCalledOnce();
  });

  // A reorder pins the listed rows ahead of every omitted member, so it needs the
  // whole container on screen.
  it.each([
    ["on a later catalog page", { previous: true }, {}],
    ["while a worktree narrows the list", {}, { worktreeId: "wt-1" }],
  ])("Should keep manual reorder off %s", async (_case, page, narrowed) => {
    const layout = fakeLayout();
    const catalog: NonNullable<SessionListViewModel["catalog"]> = {
      sessions: layoutRows,
      facets: undefined,
      total: undefined,
      loading: false,
      failed: false,
      next: false,
      previous: false,
      paging: false,
      nextPage: vi.fn(),
      previousPage: vi.fn(),
      retry: vi.fn(),
      ...page,
    };
    renderList({
      sessions: layoutRows,
      view: { ...view, sort: "manual", layout, catalog, ...narrowed },
    });
    fireEvent.contextMenu(screen.getByTestId("session-sidebar-session-r1"));
    const menu = await screen.findByRole("menu");
    expect(within(menu).getByRole("menuitem", { name: "Move to folder" })).toBeInTheDocument();
    expect(within(menu).queryByRole("menuitem", { name: "Move up" })).not.toBeInTheDocument();
    expect(within(menu).queryByRole("menuitem", { name: "Move down" })).not.toBeInTheDocument();

    const row = (id: string) => screen.getByTestId(`session-sidebar-session-${id}`).parentElement!;
    fireEvent.dragStart(row("r1"));
    fireEvent.dragOver(row("r2"));
    fireEvent.drop(row("r2"));
    expect(layout.reorderSessions).not.toHaveBeenCalled();
  });

  it("Should open the row menu from the keyboard and move a session into a folder", async () => {
    // A keyboard-opened menu is driven by keys; the pointer check would only see its guard layer.
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    const { layout } = renderLayout("created");
    screen.getByTestId("session-sidebar-session-r2").focus();
    await user.keyboard("{Shift>}{F10}{/Shift}");
    const menu = await screen.findByRole("menu");
    expect(within(menu).queryByRole("menuitem", { name: "Move up" })).not.toBeInTheDocument();
    within(menu).getByRole("menuitem", { name: "Move to folder" }).focus();
    await user.keyboard("{ArrowRight}");
    const beta = await screen.findByRole("menuitem", { name: "Beta" });
    await waitFor(() =>
      expect(beta.closest('[role="menu"]')).toContainElement(document.activeElement as HTMLElement)
    );
    beta.focus();
    await user.keyboard("{Enter}");
    await waitFor(() => expect(layout.moveSession).toHaveBeenCalledWith("r2", "fb"));
  });

  it("Should rename and delete a folder from its header menu", async () => {
    const user = userEvent.setup();
    const { layout } = renderLayout();
    fireEvent.contextMenu(screen.getByTestId("session-sidebar-folder-fa-toggle"));
    const menu = await screen.findByRole("menu");
    expect(
      within(menu)
        .getAllByRole("menuitem")
        .map(item => item.textContent)
    ).toEqual([
      "New session in folder",
      "Rename folder…",
      "Move folder up",
      "Move folder down",
      "Delete folder…",
    ]);
    await user.click(within(menu).getByRole("menuitem", { name: "Delete folder…" }));
    expect(await screen.findByText("Delete folder?")).toBeInTheDocument();
    expect(
      screen.getByText("Sessions in this folder move back to the list. No sessions are deleted.")
    ).toBeInTheDocument();
    await user.click(screen.getByTestId("session-folder-delete-confirm"));
    expect(layout.deleteFolder).toHaveBeenCalledWith("fa");

    fireEvent.contextMenu(screen.getByTestId("session-sidebar-folder-fb-toggle"));
    await user.click(await screen.findByRole("menuitem", { name: "Rename folder…" }));
    const input = await screen.findByTestId("session-folder-name");
    await user.clear(input);
    await user.type(input, "Gamma");
    await user.click(screen.getByTestId("session-folder-name-confirm"));
    expect(layout.renameFolder).toHaveBeenCalledWith("fb", "Gamma");
  });

  function rowContainer(id: string) {
    return screen.getByTestId(`session-sidebar-session-${id}`).parentElement!;
  }

  // jsdom reports zero-size boxes, so every pointer lands on the target's lower half ("after").
  it("Should reorder root rows by drag in Manual sort with a drop indicator", () => {
    const { layout } = renderLayout();
    fireEvent.dragStart(rowContainer("r1"));
    expect(rowContainer("r1")).toHaveAttribute("data-dragging", "true");
    fireEvent.dragOver(rowContainer("r2"));
    expect(screen.getByTestId("session-sidebar-drop-after-r2")).toBeInTheDocument();
    fireEvent.drop(rowContainer("r2"));
    expect(layout.reorderSessions).toHaveBeenCalledWith(null, ["r2", "r1"]);
    expect(screen.queryByTestId("session-sidebar-drop-after-r2")).not.toBeInTheDocument();
  });

  it("Should move by drag into a folder and back out under any sort, but not reorder", () => {
    const { layout } = renderLayout("created");
    fireEvent.dragStart(rowContainer("r1"));
    fireEvent.dragOver(rowContainer("r2"));
    expect(screen.queryByTestId("session-sidebar-drop-after-r2")).not.toBeInTheDocument();
    const header = screen.getByTestId("session-sidebar-folder-fb-header");
    fireEvent.dragOver(header);
    expect(header).toHaveAttribute("data-drop", "into");
    fireEvent.drop(header);
    expect(layout.moveSession).toHaveBeenCalledWith("r1", "fb");

    fireEvent.dragStart(rowContainer("f1"));
    fireEvent.dragOver(screen.getByTestId("session-sidebar-root"));
    fireEvent.drop(screen.getByTestId("session-sidebar-root"));
    expect(layout.moveSession).toHaveBeenCalledWith("f1", null);
    expect(layout.reorderSessions).not.toHaveBeenCalled();
  });

  it("Should reorder folders by dragging one header onto another", () => {
    const { layout } = renderLayout("created");
    fireEvent.dragStart(screen.getByTestId("session-sidebar-folder-fa-header"));
    fireEvent.dragOver(screen.getByTestId("session-sidebar-folder-fb-header"));
    expect(screen.getByTestId("session-sidebar-folder-fb-header")).toHaveAttribute(
      "data-drop",
      "after"
    );
    fireEvent.drop(screen.getByTestId("session-sidebar-folder-fb-header"));
    expect(layout.reorderFolders).toHaveBeenCalledWith(["fb", "fa"]);
  });
});
