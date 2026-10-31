import {
  QueryClient,
  QueryClientProvider,
  useInfiniteQuery,
  useQuery,
} from "@tanstack/react-query";
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
import { useState } from "react";
import { toast } from "sonner";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest } from "@/test/fetch-test-utils";
import { allModelsListOptions, type ProviderModelPayload } from "@/systems/model-catalog";
import {
  clearActiveWorkspaceSelection,
  rehydrateActiveWorkspaceStore,
  setActiveWorkspaceId,
  workspacesListOptions,
  workspaceBadgeIdentity,
  type DataScope,
} from "@/systems/workspace";
import { rehydrateProfileViewStore, resetProfileViews, setProfileView } from "@/systems/profiles";
import { providerListOptions, type ProviderSummary } from "@/systems/providers";

import { primaryAgentFixture } from "@/systems/agent/testing";
import { agentsListOptions } from "@/systems/agent";

import { primarySessionFixture } from "../../testing";
import type { SessionListViewModel } from "../../hooks/use-session-list-view";
import {
  profileSessionGroups,
  projectSessionGroup,
  projectSessionGroups,
} from "../../lib/session-list-groups";
import { buildProjectGroupTree, projectGroupTreeGroups } from "../../lib/workspace-folders";
import { workspaceFolderFixture } from "../../mocks/workspace-layout-handlers";
import {
  sessionListCatalogFixture,
  sessionListViewFixture,
} from "../../mocks/session-list-view-fixtures";
import {
  SessionClearContext,
  type SessionClearRequest,
} from "../../contexts/session-clear-context-value";
import type { SessionThreadSegment } from "../../lib/session-thread-branch";
import type { SessionPayload, WorkspacePlacementPayload } from "../../types";
import { useSessionSubagentBranch } from "../../hooks/use-session-subagent-branch";
import { useSubagentRoster } from "../../hooks/use-subagent-roster";
import { sessionKeys } from "../../lib/query-keys";
import { sessionScopedDetailOptions, sessionTranscriptOptions } from "../../lib/query-options";
import { preloadSessionEntry } from "../../lib/session-entry-preload";
import { subagentViewFromPayload } from "../../lib/subagent-payload";
import { formatLaterReturn } from "../../lib/session-later";
import { SessionSidebar } from "../session-sidebar";

// Dynamic JavaScript acquisition is an I/O boundary; these tests exercise the real query cache.
vi.mock("@/systems/os/lib/app-registry", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/os/lib/app-registry")>()),
  preloadOsAppCode: vi.fn().mockResolvedValue(undefined),
}));

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
const view: SessionListViewModel = sessionListViewFixture({
  sort: "last_activity",
  setSort: vi.fn(),
  setShelf: vi.fn(),
  ownerOf: () => ({ id: "default", name: "default", archived: false }),
});
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
    subagentThreadIds?: string[];
    view?: SessionListViewModel;
    actions?: { onAdopt?: () => void; onMakeTopLevel?: () => void };
    /** The session window's clear request for the session it has open. */
    clear?: SessionClearRequest;
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
    onMoveToLater: vi.fn(),
    onMoveToLaterMany: vi.fn(),
    onBackFromLater: vi.fn(),
    onBackFromLaterMany: vi.fn(),
    onDeleteMany: vi.fn(),
    ...overrides.actions,
  };
  const onBranch = vi.fn<(sessionId: string, segment: SessionThreadSegment | null) => void>();
  const props = {
    open: true,
    sessions: overrides.sessions ?? sessions,
    revealedSession: overrides.revealedSession,
    currentSessionId: overrides.currentSessionId,
    disconnected: false,
    view: overrides.view ?? view,
    onSelectSession: onSelect,
    onNewSession: overrides.onNewSession ?? vi.fn(),
    sessionActions: actions,
  };
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  seedRuntimeCatalog(queryClient);
  // The rail's branch flags, held the way the sidebar store holds them: opening the
  // subagent list folds the children, and a fold closes both.
  function BranchHost(
    hostProps: Omit<
      React.ComponentProps<typeof SessionSidebar>,
      "collapsedThreadIds" | "subagentThreadIds" | "onThreadBranchChange"
    >
  ) {
    const [collapsed, setCollapsed] = useState(overrides.collapsedThreadIds ?? []);
    const [subagents, setSubagents] = useState(overrides.subagentThreadIds ?? []);
    return (
      <SessionSidebar
        {...hostProps}
        collapsedThreadIds={collapsed}
        subagentThreadIds={subagents}
        onThreadBranchChange={(sessionId, segment) => {
          onBranch(sessionId, segment);
          setCollapsed(ids => [
            ...ids.filter(id => id !== sessionId),
            ...(segment === "children" ? [] : [sessionId]),
          ]);
          setSubagents(ids => [
            ...ids.filter(id => id !== sessionId),
            ...(segment === "subagents" ? [sessionId] : []),
          ]);
        }}
      />
    );
  }
  const tree = (node: React.ReactNode) => (
    <QueryClientProvider client={queryClient}>
      <SessionClearContext value={overrides.clear ?? null}>{node}</SessionClearContext>
    </QueryClientProvider>
  );
  const rendered = render(tree(<BranchHost {...props} />));
  return {
    onSelect,
    onBranch,
    actions,
    queryClient,
    update: (nextSessions: SessionPayload[], nextView = view) =>
      rendered.rerender(tree(<BranchHost {...props} sessions={nextSessions} view={nextView} />)),
  };
}

// Invariant: catalog intent warms one scoped session head before selection, without crossing a
// workspace/profile boundary or rereading retained history. Owner: SessionList entry + Query cache.
describe("SessionList entry preloading", () => {
  const session = sessions[0]!;
  const workspace = {
    id: session.workspace_id!,
    name: "Session project",
    root_dir: "/tmp/session-project",
    add_dirs: [],
    created_at: session.created_at,
    updated_at: session.updated_at,
  };
  const transcript = {
    entries: [],
    epoch: 1,
    generation: 1,
    max_sequence: 210,
    has_older: true,
    next_before_sequence: 10,
    limit: 200,
  };
  const readPaths: string[] = [];

  beforeEach(async () => {
    await Promise.all([rehydrateActiveWorkspaceStore(), rehydrateProfileViewStore()]);
    setActiveWorkspaceId(workspace.id);
    setProfileView({ kind: "profile", profile: session.profile_name });
    readPaths.length = 0;
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>(async request => {
        const url = new URL(request instanceof Request ? request.url : request);
        readPaths.push(`${url.pathname}${url.search}`);
        return Response.json(url.pathname.endsWith("/transcript") ? transcript : { session });
      })
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    clearActiveWorkspaceSelection();
    resetProfileViews();
  });

  function renderedEntry(target = session) {
    const rendered = renderList({ sessions: [target] });
    rendered.queryClient.setQueryData(workspacesListOptions().queryKey, [workspace]);
    return rendered;
  }

  it("Should fetch detail and one transcript head before clicking, then reuse both on mount", async () => {
    const { queryClient, onSelect } = renderedEntry();
    const row = screen.getByTestId(`session-sidebar-session-${session.id}`);
    fireEvent.pointerEnter(row);
    expect(readPaths).toEqual([]);
    await waitFor(() => expect(readPaths).toHaveLength(2));
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));
    expect(readPaths).toEqual([
      `/api/sessions/${session.id}?profile=${session.profile_name}&include_health=true`,
      `/api/workspaces/${workspace.id}/sessions/${session.id}/transcript`,
    ]);
    expect(onSelect).not.toHaveBeenCalled();

    fireEvent.click(row);
    expect(onSelect).toHaveBeenCalledWith(session);
    const observer = renderHook(
      () => {
        useQuery(sessionScopedDetailOptions(session.id, { profile: session.profile_name }));
        useInfiniteQuery(sessionTranscriptOptions(workspace.id, session.id));
      },
      {
        wrapper: ({ children }) => (
          <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
        ),
      }
    );
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));
    expect(readPaths).toHaveLength(2);
    observer.unmount();
    queryClient.clear();
  });

  it.each([{ workspace_id: "foreign-project" }, { profile_name: "foreign-profile" }])(
    "Should skip speculative session data outside the selected scope: %j",
    async overrides => {
      const target = { ...session, ...overrides };
      const { queryClient } = renderedEntry(target);
      await preloadSessionEntry(queryClient, target);
      expect(readPaths).toEqual([]);
      queryClient.clear();
    }
  );

  it("Should retry a failed detail read on later intent and then warm the transcript", async () => {
    vi.mocked(globalThis.fetch).mockImplementationOnce(async request => {
      const url = new URL(request instanceof Request ? request.url : request);
      readPaths.push(`${url.pathname}${url.search}`);
      return Response.json({ error: "Temporarily unavailable" }, { status: 503 });
    });
    const { queryClient } = renderedEntry();
    const row = screen.getByTestId(`session-sidebar-session-${session.id}`);
    const detail = sessionScopedDetailOptions(session.id, { profile: session.profile_name });
    fireEvent.pointerEnter(row);
    await waitFor(() => expect(queryClient.getQueryState(detail.queryKey)?.status).toBe("error"));
    expect(readPaths).toHaveLength(1);
    expect(
      queryClient.getQueryData(sessionTranscriptOptions(workspace.id, session.id).queryKey)
    ).toBeUndefined();

    fireEvent.pointerLeave(row);
    fireEvent.pointerEnter(row);
    await waitFor(() => expect(readPaths).toHaveLength(3));
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));
    expect(readPaths).toEqual([
      `/api/sessions/${session.id}?profile=${session.profile_name}&include_health=true`,
      `/api/sessions/${session.id}?profile=${session.profile_name}&include_health=true`,
      `/api/workspaces/${workspace.id}/sessions/${session.id}/transcript`,
    ]);
    queryClient.clear();
  });

  it("Should skip a queued session read when the profile changes before intent settles", async () => {
    vi.useFakeTimers();
    try {
      const { queryClient } = renderedEntry();
      fireEvent.pointerEnter(screen.getByTestId(`session-sidebar-session-${session.id}`));
      act(() => setProfileView({ kind: "profile", profile: "other" }));
      await act(() => vi.advanceTimersByTimeAsync(100));
      expect(readPaths).toEqual([]);
      expect(queryClient.isFetching()).toBe(0);
      queryClient.clear();
    } finally {
      vi.useRealTimers();
    }
  });

  it("Should not read the transcript after a profile switch while detail is in flight", async () => {
    let resolveDetail!: (response: Response) => void;
    vi.mocked(globalThis.fetch).mockImplementationOnce(
      () => new Promise<Response>(resolve => (resolveDetail = resolve))
    );
    const { queryClient } = renderedEntry();
    const preload = preloadSessionEntry(queryClient, session);
    await waitFor(() => expect(globalThis.fetch).toHaveBeenCalledTimes(1));
    act(() => setProfileView({ kind: "profile", profile: "other" }));
    resolveDetail(Response.json({ session }));
    await preload;
    expect(globalThis.fetch).toHaveBeenCalledTimes(1);
    expect(
      queryClient.getQueryData(sessionTranscriptOptions(workspace.id, session.id).queryKey)
    ).toBeUndefined();
    queryClient.clear();
  });

  it("Should preserve a stale multi-page transcript without refetching history on intent", async () => {
    const { queryClient } = renderedEntry();
    const options = sessionTranscriptOptions(workspace.id, session.id);
    const retained = {
      pages: [
        { ...transcript, cursor: transcript.max_sequence },
        { ...transcript, has_older: false, max_sequence: 10, cursor: 10 },
      ],
      pageParams: [undefined, { beforeSequence: 10, epoch: 1, generation: 1 }],
    };
    queryClient.setQueryData(options.queryKey, retained, { updatedAt: Date.now() - 60_000 });
    await preloadSessionEntry(queryClient, session);
    expect(readPaths).toHaveLength(1);
    expect(readPaths[0]).not.toContain("/transcript");
    expect(queryClient.getQueryData(options.queryKey)).toBe(retained);
    queryClient.clear();
  });
});

// USW W-08: a session running with the agent's `unrestricted` grant carries the warning-tone
// Unrestricted badge on line 2, named for assistive tech with the grant's plain description.
describe("SessionList unrestricted grant", () => {
  it("W-08 shows the Unrestricted badge only on sessions with the grant", () => {
    renderList({
      sessions: [
        { ...sessions[0]!, effective_workspace_access: "unrestricted" },
        { ...sessions[1]!, effective_workspace_access: "" },
      ],
    });

    expect(screen.getByTestId("session-row-unrestricted-running")).toHaveAccessibleName(
      "Unrestricted: This agent can read and change every project and profile."
    );
    expect(screen.queryByTestId("session-row-unrestricted-stopped")).not.toBeInTheDocument();
  });
});

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
    await user.click(screen.getByRole("button", { name: "Sort by: Created" }));
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
    // A running row in the selection archives too, after the daemon stops it.
    expect(screen.getByTestId("session-sidebar-selection-archive")).toHaveTextContent(
      "Stop and archive2 sessions"
    );
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

  it("clears selection when the list's identity changes", () => {
    const { update } = renderList();
    fireEvent.click(screen.getByTestId("session-sidebar-session-running"), { metaKey: true });
    expect(screen.getByTestId("session-sidebar-selection-count")).toHaveTextContent("1 selected");
    update(sessions, { ...view, selectionKey: "another-breadth" });
    expect(screen.queryByRole("toolbar", { name: "Selected sessions" })).not.toBeInTheDocument();
    expect(screen.getByTestId("session-sidebar-session-running")).not.toHaveAttribute(
      "aria-pressed"
    );
  });

  // #1029: a context click inside a multi-selection acts on the whole selection with the
  // bar's verbs; one outside it retargets the selection to that row (Finder) and keeps the
  // per-session menu, which a single selection also keeps.
  it("scopes a selected row's context menu to the whole multi-selection", async () => {
    const user = userEvent.setup();
    const { actions } = renderList();
    const running = screen.getByTestId("session-sidebar-session-running");
    fireEvent.click(running, { metaKey: true });
    fireEvent.contextMenu(running);
    expect(await screen.findByTestId("session-row-menu-copy-id-running")).toBeVisible();
    await user.keyboard("{Escape}");
    fireEvent.click(screen.getByTestId("session-sidebar-session-stopped"));
    expect(screen.getByTestId("session-sidebar-selection-count")).toHaveTextContent("2 selected");
    fireEvent.contextMenu(running);
    expect(await screen.findByTestId("session-sidebar-selection-menu-count")).toHaveTextContent(
      "2 sessions"
    );
    expect(screen.queryByTestId("session-row-menu-copy-id-running")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-sidebar-selection-menu-stop")).toHaveTextContent("1 active");
    expect(screen.getByTestId("session-sidebar-selection-menu-archive")).toHaveTextContent(
      "Stop and archive2 sessions"
    );
    expect(screen.getByTestId("session-sidebar-selection-menu-delete-item")).toHaveTextContent(
      "Delete 2 sessions"
    );
    // The list's ⌘⌫ cannot reach a portaled context menu, so its hint stays on the bar only.
    expect(screen.getByTestId("session-sidebar-selection-menu-delete-item")).not.toHaveTextContent(
      "⌘⌫"
    );
    await user.click(screen.getByTestId("session-sidebar-selection-menu-archive"));
    expect(actions.onArchiveMany).toHaveBeenCalledWith([sessions[0], sessions[1]]);
    expect(actions.onArchive).not.toHaveBeenCalled();

    // Shift+F10 opens the same selection menu from the keyboard.
    running.focus();
    await user.keyboard("{Shift>}{F10}{/Shift}");
    await user.click(await screen.findByTestId("session-sidebar-selection-menu-delete-item"));
    expect(actions.onDeleteMany).toHaveBeenCalledWith(
      [sessions[0], sessions[1]],
      expect.any(Function)
    );

    fireEvent.contextMenu(screen.getByTestId("session-sidebar-session-other"));
    expect(await screen.findByTestId("session-row-menu-copy-id-other")).toBeVisible();
    expect(screen.getByTestId("session-sidebar-selection-count")).toHaveTextContent("1 selected");
    expect(screen.getByTestId("session-sidebar-session-other")).toHaveAttribute(
      "aria-pressed",
      "true"
    );
    expect(running).toHaveAttribute("aria-pressed", "false");
  });

  it("offers stop and archive on a running session row in one action", async () => {
    const user = userEvent.setup();
    const { actions } = renderList();

    await user.click(screen.getByTestId("session-row-actions-running"));
    const archive = await screen.findByTestId("session-row-archive-running");
    expect(archive).toHaveTextContent("Stop and archive session");
    await user.click(archive);
    expect(actions.onArchive).toHaveBeenCalledExactlyOnceWith(sessions[0]);
  });

  // #952: Clear conversation left the window head; the window offers it on its own session's row
  // only (clearing resets that window's runtime thread), unavailable while nothing can be cleared.
  it("offers Clear conversation only on the row of the window's own session", async () => {
    const user = userEvent.setup();
    const request = vi.fn();
    renderList({ clear: { sessionId: "running", canClear: true, request } });

    await user.click(screen.getByTestId("session-row-actions-other"));
    expect(await screen.findByTestId("session-row-delete-other")).toBeInTheDocument();
    expect(screen.queryByTestId("session-row-clear-other")).not.toBeInTheDocument();
    await user.keyboard("{Escape}");

    await user.click(screen.getByTestId("session-row-actions-running"));
    await user.click(await screen.findByTestId("session-row-clear-running"));
    expect(request).toHaveBeenCalledOnce();
  });

  it("shows Clear conversation unavailable while the conversation cannot be cleared", async () => {
    const user = userEvent.setup();
    renderList({ clear: { sessionId: "running", canClear: false, request: vi.fn() } });
    await user.click(screen.getByTestId("session-row-actions-running"));
    expect(await screen.findByTestId("session-row-clear-running")).toHaveAttribute(
      "aria-disabled",
      "true"
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
    // Only the plain child counts toward the branch; the revealed subagent never folds.
    expect(screen.getByRole("button", { name: "1 child session" })).toHaveAttribute(
      "aria-expanded",
      "true"
    );
  });

  it("UT-W18: a parent whose only child is the viewed subagent gets no child-session toggle", () => {
    const viewed = row("viewed-sub", { lineage: lineage("parent", "subagent") });
    renderList({ sessions: [row("parent", summary(1, 3))], revealedSession: viewed });

    expect(screen.getByTestId("session-sidebar-session-viewed-sub")).toBeVisible();
    expect(screen.queryByRole("button", { name: /child session/ })).not.toBeInTheDocument();
    expect(screen.getByTestId("session-sidebar-thread-toggle-parent")).toHaveAttribute(
      "data-segment",
      "subagents"
    );
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
    // A folded list does not mount; the viewed subagent stays outside the fold.
    expect(screen.queryByTestId("session-sidebar-session-plain")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-sidebar-session-viewed-sub")).toBeVisible();
  });

  it("Should browse delegated work on a revealed subagent outside its parent fold", async () => {
    const user = userEvent.setup();
    const parent = row("parent", summary(1, 3));
    const viewed = row("viewed-sub", {
      ...summary(1, 1),
      lineage: lineage("parent", "subagent"),
    });
    vi.mocked(globalThis.fetch).mockResolvedValue(subagentPage(0, 1));
    const { onBranch } = renderList({
      sessions: [parent],
      revealedSession: viewed,
      collapsedThreadIds: ["parent"],
    });
    await user.click(screen.getByRole("button", { name: "1 of 1 subagent running" }));
    expect(onBranch).toHaveBeenLastCalledWith("viewed-sub", "subagents");
    expect(await screen.findByTestId("session-sidebar-subagents-viewed-sub")).toBeVisible();
    expect(screen.getByTestId("session-sidebar-session-viewed-sub")).toBeVisible();
  });

  it("Should not report first-page loading when the branch query is disabled", () => {
    const queryClient = new QueryClient();
    const { result } = renderHook(() => useSessionSubagentBranch("", "unscoped", 0), {
      wrapper: ({ children }) => (
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      ),
    });
    expect(result.current.loading).toBe(false);
    expect(globalThis.fetch).not.toHaveBeenCalled();
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
    expect(screen.queryByRole("button", { name: /child session/ })).not.toBeInTheDocument();
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

  it("reads no subagent cache for a folded branch across list re-renders", () => {
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

  const subagentPage = (from: number, to: number, nextCursor?: string) =>
    new Response(
      JSON.stringify({
        subagents: Array.from({ length: to - from }, (_, offset) =>
          wireSubagent(from + offset, from + offset < 3 ? "running" : "completed")
        ),
        ...(nextCursor ? { next_cursor: nextCursor } : {}),
      }),
      { headers: { "Content-Type": "application/json" } }
    );

  // Invariant: an opened branch keeps the rows' place while its first page is read, sized by the
  // parent's summary, and hands it to the real rows. Owner: the rail's subagent branch.
  it("Should hold an opened branch's place with placeholder rows until its list answers", async () => {
    let answer: (page: Response) => void = () => undefined;
    vi.mocked(globalThis.fetch).mockImplementation(
      () =>
        new Promise<Response>(resolve => {
          answer = resolve;
        })
    );
    renderList({ sessions: [row("parent", summary(0, 3))], subagentThreadIds: ["parent"] });

    const branch = await screen.findByTestId("session-sidebar-subagents-parent");
    const loading = within(branch).getByRole("status");
    expect(loading).toHaveTextContent("Loading subagents");
    expect(loading.querySelectorAll(':scope > [aria-hidden="true"]')).toHaveLength(3);

    answer(subagentPage(0, 3));

    expect(await within(branch).findAllByRole("button", { name: /^Open Subagent/ })).toHaveLength(
      3
    );
    expect(within(branch).queryByRole("status")).not.toBeInTheDocument();
  });

  // Invariant: branch rows carry the session StateGlyph and word — running and completed never
  // read alike — and the loaded rows order by urgency. Owner: the rail's subagent branch.
  it("Should mark branch rows with distinct glyphs and words, most urgent first", async () => {
    const settledAt = "2026-10-08T12:30:00Z";
    vi.mocked(globalThis.fetch).mockResolvedValue(
      new Response(
        JSON.stringify({
          subagents: [
            { ...wireSubagent(4, "completed"), settled_at: settledAt },
            wireSubagent(3, "running"),
            { ...wireSubagent(2, "failed"), settled_at: settledAt },
            wireSubagent(1, "waiting"),
          ],
        }),
        { headers: { "Content-Type": "application/json" } }
      )
    );
    renderList({ sessions: [row("parent", summary(2, 4, 1, 1))], subagentThreadIds: ["parent"] });

    const branch = await screen.findByTestId("session-sidebar-subagents-parent");
    await waitFor(() =>
      expect(within(branch).getAllByRole("button", { name: /^Open Subagent/ })).toHaveLength(4)
    );
    const rows = [...branch.querySelectorAll('[data-slot="session-subagent-row"]')];
    expect(rows.map(node => node.getAttribute("data-state"))).toEqual([
      "attention",
      "failed",
      "running",
      "done",
    ]);
    expect(
      rows.map(node => node.querySelector('[data-slot="state-glyph"]')?.getAttribute("data-state"))
    ).toEqual(["attention", "failed", "running", "done"]);
    const open = (title: string) => within(branch).getByRole("button", { name: `Open ${title}` });
    expect(open("Subagent 3")).toHaveAttribute("aria-description", "Working");
    expect(open("Subagent 4")).toHaveAttribute("aria-description", "Done");
    expect(within(open("Subagent 3")).getByText("Working")).toBeInTheDocument();
    expect(within(open("Subagent 4")).getByText("Done")).toBeInTheDocument();
  });

  // Invariant: a sidebar without a drawer must not advertise an action with no target.
  // Owner/canonical suite: session sidebar navigation, this file.
  it("Should keep a pending child without a drawer host non-interactive", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(
      new Response(
        JSON.stringify({
          subagents: [{ ...wireSubagent(1, "queued"), child_session_id: null }],
        }),
        { headers: { "Content-Type": "application/json" } }
      )
    );
    renderList({ sessions: [row("parent", summary(1, 1))], subagentThreadIds: ["parent"] });
    const branch = await screen.findByTestId("session-sidebar-subagents-parent");
    expect(await within(branch).findByText("Subagent 1")).toBeVisible();
    expect(
      within(branch).queryByRole("button", { name: "Open Subagent 1" })
    ).not.toBeInTheDocument();
  });

  // Issue #919: the subagent list reads nothing while folded, then short pages on demand.
  it("UT-W17: opens a bounded subagent branch that reads one page and pages with Show more", async () => {
    const user = userEvent.setup();
    const parent = row("parent", { badge: "running", ...summary(3, 21) });
    vi.mocked(globalThis.fetch).mockImplementation(async input =>
      new URL((input as Request).url).searchParams.get("cursor") === "page-2"
        ? subagentPage(8, 16, "page-3")
        : subagentPage(0, 8, "page-2")
    );
    const { onBranch } = renderList({ sessions: [parent] });

    const toggle = screen.getByRole("button", { name: "3 of 21 subagents running" });
    expect(toggle).toHaveTextContent("3/21");
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(globalThis.fetch).not.toHaveBeenCalled();

    await user.click(toggle);
    expect(onBranch).toHaveBeenLastCalledWith("parent", "subagents");
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    const branch = await screen.findByTestId("session-sidebar-subagents-parent");
    await waitFor(() =>
      expect(within(branch).getAllByRole("button", { name: /^Open Subagent/ })).toHaveLength(8)
    );
    await expectFetchRequest({
      path: `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/parent/subagents?all_profiles=true&limit=8&order=urgency`,
    });

    await user.click(within(branch).getByRole("button", { name: "Show 8 more" }));
    await waitFor(() =>
      expect(within(branch).getAllByRole("button", { name: /^Open Subagent/ })).toHaveLength(16)
    );
    await expectFetchRequest({
      callIndex: 1,
      path: `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/parent/subagents?all_profiles=true&limit=8&order=urgency&cursor=page-2`,
    });
    expect(within(branch).getByRole("button", { name: "Show 5 more" })).toBeInTheDocument();
    expect(globalThis.fetch).toHaveBeenCalledTimes(2);

    // Folding unmounts the list; reopening reads the cache, not the route.
    await user.click(toggle);
    expect(onBranch).toHaveBeenLastCalledWith("parent", null);
    await waitFor(() =>
      expect(screen.queryByTestId("session-sidebar-subagents-parent")).not.toBeInTheDocument()
    );
  });

  // M9: an open parent's stream-fed roster feeds the branch; the list route stays cold.
  // Invariant: only this parent's workspace-qualified roster updates its live
  // branch; removing its last stream observer restores the authoritative list.
  // Owning layer: roster/query integration; canonical suite: this sidebar suite.
  it("Should keep a live branch scoped and fall back when its stream owner leaves", async () => {
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
    await user.click(screen.getByRole("button", { name: "1 of 1 subagent running" }));
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

  it("UT-W17: shows the total alone for failures and a quiet count once everything settled", () => {
    renderList({
      sessions: [row("failed-parent", summary(0, 6, 2)), row("clean-parent", summary(0, 4))],
    });

    const failed = screen.getByRole("button", { name: "6 subagents, 2 failed" });
    expect(failed).toHaveTextContent(/^6$/);
    expect(failed).toHaveAttribute("data-state", "failed");
    // Settled subagents stay browsable: the indicator keeps the count, without a glyph.
    const clean = screen.getByRole("button", { name: "4 subagents" });
    expect(clean).toHaveTextContent(/^4$/);
    expect(clean).not.toHaveAttribute("data-state");
  });

  it("UT-W14: a parent the daemon reports waiting on sessions reads delegated, a running one does not", () => {
    renderList({
      sessions: [
        row("waiting-parent", { badge: "waiting", waiting_on: "sessions", ...summary(2, 2) }),
        row("working-parent", { badge: "running", ...summary(2, 2) }),
      ],
    });

    const waiting = screen.getByTestId("session-sidebar-session-waiting-parent");
    const mark = within(waiting).getByRole("img", { name: "Session badge: waiting" });
    expect(mark).toHaveAttribute("data-waiting-on", "sessions");
    expect(mark.querySelector("[data-state]")).toHaveAttribute("data-state", "delegated");
    expect(waiting.closest("li") ?? waiting.parentElement!).toHaveTextContent(
      "Waiting on sessions"
    );
    expect(screen.getAllByRole("button", { name: "2 of 2 subagents running" })[0]).toHaveAttribute(
      "data-state",
      "delegated"
    );
    const working = screen.getByTestId("session-sidebar-session-working-parent");
    expect(within(working).getByRole("img")).toHaveAttribute("data-badge", "running");
  });

  it("UT-075: reads the daemon's waiting reason, never a client guess from monitors or subagents", () => {
    renderList({
      sessions: [
        row("watching", {
          badge: "waiting",
          waiting_on: "monitor",
          monitor_summary: { active: 1 },
        }),
        row("watching-running", { badge: "running", monitor_summary: { active: 1 } }),
        row("finished", { badge: "done", monitor_summary: { active: 1 }, ...summary(1, 1) }),
      ],
    });

    const watching = screen.getByTestId("session-sidebar-session-watching");
    const mark = within(watching).getByRole("img");
    expect(mark).toHaveAttribute("data-badge", "waiting");
    expect(mark).toHaveAttribute("data-waiting-on", "monitor");
    expect(mark.querySelector("[data-state]")).toHaveAttribute("data-state", "monitoring");
    const running = screen.getByTestId("session-sidebar-session-watching-running");
    expect(within(running).getByRole("img")).toHaveAttribute("data-badge", "running");
    const finished = screen.getByTestId("session-sidebar-session-finished");
    expect(within(finished).getByRole("img")).toHaveAttribute("data-badge", "done");
  });

  it("opens a subagent's own session from its branch row", async () => {
    const user = userEvent.setup();
    const parent = row("branch-parent", summary(1, 1));
    vi.mocked(globalThis.fetch).mockResolvedValue(subagentPage(0, 1));
    const { onSelect } = renderList({ sessions: [parent], subagentThreadIds: ["branch-parent"] });

    await user.click(await screen.findByRole("button", { name: "Open Subagent 0" }));

    expect(onSelect).toHaveBeenCalledExactlyOnceWith({
      id: "child-0",
      agent_name: "coder",
      workspace_id: parent.workspace_id,
    });
  });

  // Issue #919: with both kinds, the indicators are segments of one control; the selected
  // segment folds the branch, the chevron reopens the first list, one list shows at a time.
  it("Should switch between child sessions and subagents through the segmented control", async () => {
    const user = userEvent.setup();
    const parent = row("parent", summary(1, 2));
    const child = row("plain", { lineage: lineage("parent") });
    vi.mocked(globalThis.fetch).mockResolvedValue(subagentPage(0, 2));
    const { onBranch } = renderList({ sessions: [parent, child] });

    const group = screen.getByRole("group", { name: "Child sessions and subagents" });
    const children = within(group).getByRole("button", { name: "1 child session" });
    const subagents = within(group).getByRole("button", { name: "1 of 2 subagents running" });
    const chevron = screen.getByRole("button", { name: "Toggle child sessions and subagents" });
    expect(children).toHaveAttribute("aria-expanded", "true");
    expect(subagents).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByTestId("session-sidebar-session-plain")).toBeVisible();
    expect(globalThis.fetch).not.toHaveBeenCalled();

    await user.click(subagents);
    expect(onBranch).toHaveBeenLastCalledWith("parent", "subagents");
    expect(subagents).toHaveAttribute("aria-expanded", "true");
    expect(children).toHaveAttribute("aria-expanded", "false");
    expect(await screen.findByRole("button", { name: "Open Subagent 1" })).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.queryByTestId("session-sidebar-session-plain")).not.toBeInTheDocument()
    );

    // Pressing the selected segment folds the branch, like the chevron.
    await user.click(subagents);
    expect(onBranch).toHaveBeenLastCalledWith("parent", null);
    expect(chevron).toHaveAttribute("aria-expanded", "false");

    await user.click(chevron);
    expect(onBranch).toHaveBeenLastCalledWith("parent", "children");
    expect(children).toHaveAttribute("aria-expanded", "true");
    expect(await screen.findByTestId("session-sidebar-session-plain")).toBeVisible();
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
    // A subagent session follows its parent into and out of Later, too.
    expect(screen.queryByTestId("session-row-later-viewed-sub")).not.toBeInTheDocument();
    await user.keyboard("{Escape}");
    await user.click(screen.getByTestId("session-row-actions-parent"));
    expect(await screen.findByTestId("session-row-archive-parent")).toBeInTheDocument();
    expect(screen.getByTestId("session-row-later-parent")).toBeInTheDocument();
  });
});

// Invariant (#1076): Later mirrors the archive — one toolbar toggle beside Archive, mutually
// exclusive with it, counting the scope's parked sessions; a parked row shows why it waits,
// when it comes back and its waiting messages, and offers Back from Later instead of Move to
// Later. Owner: SessionList through its sidebar host (same suite as the archive verbs).
describe("SessionList Later", () => {
  const parked: SessionPayload = {
    ...primarySessionFixture,
    id: "parked",
    name: "Parked work",
    archived_at: null,
    state: "stopped",
    later: {
      parked_at: "2026-10-30T09:00:00Z",
      reason: "Waiting on the vendor",
      return_at: "2026-11-04T09:00:00Z",
      pending_messages: 2,
    },
  };

  it("Should switch the list between Active, Later and the archive from the toolbar", async () => {
    const user = userEvent.setup();
    const setShelf = vi.fn();
    const { update } = renderList({ view: { ...view, laterCount: 3, setShelf } });

    const later = screen.getByTestId("session-sidebar-later");
    expect(later).toHaveAccessibleName("Later, 3");
    expect(screen.getByTestId("session-sidebar-later-count")).toHaveTextContent("3");
    expect(later).toHaveAttribute("aria-pressed", "false");
    await user.click(later);
    expect(setShelf).toHaveBeenLastCalledWith("later");

    update(sessions, { ...view, shelf: "later", laterCount: 3, setShelf });
    expect(screen.getByTestId("session-sidebar-later")).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByTestId("session-sidebar-archived")).toHaveAttribute("aria-pressed", "false");
    await user.click(screen.getByTestId("session-sidebar-archived"));
    expect(setShelf).toHaveBeenLastCalledWith("archive");

    // From the archive, Later switches straight to Later rather than back to Active.
    update(sessions, { ...view, shelf: "archive", laterCount: 3, setShelf });
    await user.click(screen.getByTestId("session-sidebar-later"));
    expect(setShelf).toHaveBeenLastCalledWith("later");

    update(sessions, { ...view, shelf: "later", laterCount: 3, setShelf });
    await user.click(screen.getByTestId("session-sidebar-later"));
    expect(setShelf).toHaveBeenLastCalledWith("active");
  });

  it("Should show no count while nothing is in Later", () => {
    renderList({ view: { ...view, laterCount: 0 } });
    expect(screen.getByTestId("session-sidebar-later")).toHaveAccessibleName("Later");
    expect(screen.queryByTestId("session-sidebar-later-count")).not.toBeInTheDocument();
  });

  it("Should show a parked row's reason, return date and waiting messages", () => {
    renderList({ sessions: [parked], view: { ...view, shelf: "later" } });

    expect(screen.getByTestId("session-later-reason-parked")).toHaveTextContent(
      "Waiting on the vendor"
    );
    expect(screen.getByTestId("session-later-return-parked")).toHaveTextContent(
      `Back ${formatLaterReturn("2026-11-04T09:00:00Z")}`
    );
    expect(screen.getByTestId("session-later-parked-parked")).toHaveTextContent(/Parked/);
    const pending = screen.getByTestId("session-later-pending-parked");
    expect(pending).toHaveTextContent("2");
    expect(pending).toHaveAttribute("title", "2 messages waiting");
    expect(screen.getByTestId("session-sidebar-session-parked")).toHaveTextContent(
      "In Later, Waiting on the vendor"
    );
  });

  it("Should name the Later view when it is empty", () => {
    renderList({ sessions: [], view: { ...view, shelf: "later", scopeLabel: "marketing" } });
    expect(screen.getByText("No sessions in Later in marketing yet.")).toBeInTheDocument();
  });

  it("Should move a selection to Later and bring its parked sessions back", async () => {
    const user = userEvent.setup();
    const { actions } = renderList({ sessions: [sessions[1]!, parked] });
    const stopped = screen.getByTestId("session-sidebar-session-stopped");
    fireEvent.click(stopped, { metaKey: true });
    fireEvent.click(screen.getByTestId("session-sidebar-session-parked"), { metaKey: true });
    fireEvent.contextMenu(stopped);

    expect(await screen.findByTestId("session-sidebar-selection-menu-later")).toHaveTextContent(
      "Move to Later…1 session"
    );
    expect(screen.getByTestId("session-sidebar-selection-menu-back-from-later")).toHaveTextContent(
      "Back from Later1 in Later"
    );
    await user.click(screen.getByTestId("session-sidebar-selection-menu-later"));
    expect(actions.onMoveToLaterMany).toHaveBeenCalledWith([sessions[1], parked]);

    fireEvent.contextMenu(stopped);
    await user.click(await screen.findByTestId("session-sidebar-selection-menu-back-from-later"));
    expect(actions.onBackFromLaterMany).toHaveBeenCalledWith([sessions[1], parked]);
  });

  it("Should offer Move to Later on an active row and Back from Later on a parked one", async () => {
    const user = userEvent.setup();
    const { actions } = renderList({ sessions: [sessions[1]!, parked] });

    await user.click(screen.getByTestId("session-row-actions-stopped"));
    const later = await screen.findByTestId("session-row-later-stopped");
    expect(later).toHaveTextContent("Move to Later…");
    await user.click(later);
    expect(actions.onMoveToLater).toHaveBeenCalledExactlyOnceWith(sessions[1]);

    await user.click(screen.getByTestId("session-row-actions-parked"));
    expect(screen.queryByTestId("session-row-later-parked")).not.toBeInTheDocument();
    await user.click(await screen.findByTestId("session-row-back-from-later-parked"));
    expect(actions.onBackFromLater).toHaveBeenCalledExactlyOnceWith(parked);
  });
});

// Invariant: both row menus copy the session's CompozyOS ID and its shown title, and the
// selection menu copies every selected ID, and say so.
describe("SessionList row copy actions", () => {
  it("Should copy the session ID from the … menu and the title from the context menu", async () => {
    const user = userEvent.setup();
    const writeText = vi.spyOn(navigator.clipboard, "writeText").mockResolvedValue(undefined);
    const success = vi.spyOn(toast, "success");
    renderList();

    await user.click(screen.getByTestId("session-row-actions-running"));
    await user.click(await screen.findByTestId("session-row-copy-id-running"));
    expect(writeText).toHaveBeenLastCalledWith("running");
    await waitFor(() => expect(success).toHaveBeenLastCalledWith("Session ID copied"));

    fireEvent.contextMenu(screen.getByTestId("session-sidebar-session-stopped"));
    const menu = await screen.findByRole("menu");
    expect(within(menu).getByRole("menuitem", { name: "Copy session ID" })).toBeInTheDocument();
    await user.click(within(menu).getByRole("menuitem", { name: "Copy title" }));
    expect(writeText).toHaveBeenLastCalledWith("Stopped work");
    await waitFor(() => expect(success).toHaveBeenLastCalledWith("Title copied"));
  });

  it("Should copy every selected session ID, one per line, from the selection menu", async () => {
    const user = userEvent.setup();
    const writeText = vi.spyOn(navigator.clipboard, "writeText").mockResolvedValue(undefined);
    const success = vi.spyOn(toast, "success");
    renderList();
    fireEvent.click(screen.getByTestId("session-sidebar-session-running"), { metaKey: true });
    fireEvent.click(screen.getByTestId("session-sidebar-session-other"));
    await user.click(screen.getByTestId("session-sidebar-selection-more"));
    await user.click(await screen.findByTestId("session-sidebar-selection-copy-ids"));
    expect(writeText).toHaveBeenLastCalledWith("running\nother");
    await waitFor(() => expect(success).toHaveBeenLastCalledWith("Session IDs copied"));
  });

  it("Should copy every selected session ID from a selected row's context menu", async () => {
    const user = userEvent.setup();
    const writeText = vi.spyOn(navigator.clipboard, "writeText").mockResolvedValue(undefined);
    renderList();
    fireEvent.click(screen.getByTestId("session-sidebar-session-running"), { metaKey: true });
    fireEvent.click(screen.getByTestId("session-sidebar-session-other"));
    fireEvent.contextMenu(screen.getByTestId("session-sidebar-session-running"));
    const menu = await screen.findByRole("menu");
    await user.click(within(menu).getByRole("menuitem", { name: "Copy session IDs" }));
    expect(writeText).toHaveBeenLastCalledWith("running\nother");
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
      moveSessions: vi.fn(async () => undefined),
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

  // BR-11: folders and manual order live in one (workspace, profile) pair; a list
  // spanning pairs has no layout, so filed sessions render as plain rows.
  it("Should render filed sessions as plain rows where no single pair owns the list", () => {
    const rows = [
      { ...layoutRows[0]!, profile_id: "default" },
      { ...layoutRows[1]!, profile_id: "other", profile_name: "other" },
    ];
    renderList({ sessions: rows, view: { ...view, aggregate: true, scopeLabel: null } });
    expect(screen.queryByTestId("session-sidebar-folder-fa")).not.toBeInTheDocument();
    expect(screen.queryByTestId("session-sidebar-root")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-sidebar-session-f1")).toBeVisible();
    fireEvent.contextMenu(screen.getByTestId("session-sidebar-session-f1"));
    expect(screen.queryByRole("menuitem", { name: "Move to folder" })).not.toBeInTheDocument();
  });

  // Invariant: stale aggregate rows cannot navigate while a page change is in flight.
  it.each([true, false])("Should gate aggregate selection while paging is %s", async paging => {
    const rows = [{ ...layoutRows[0]!, profile_id: "default", profile_name: "default" }];
    const { onSelect } = renderList({
      sessions: rows,
      view: {
        ...view,
        aggregate: true,
        catalog: sessionListCatalogFixture({ sessions: rows, paging }),
      },
    });
    const row = screen.getByTestId("session-sidebar-session-r1");
    await userEvent.setup().click(row);
    expect(onSelect).toHaveBeenCalledTimes(paging ? 0 : 1);
    if (paging) expect(row.closest("[aria-busy]")).toHaveAttribute("aria-busy", "true");
    else expect(row.closest("[aria-busy]")).toBeNull();
  });

  it("Should select mixed-profile rows together in the aggregate", async () => {
    const rows = [
      { ...layoutRows[0]!, profile_id: "default" },
      { ...layoutRows[1]!, profile_id: "new-profile" },
    ];
    renderList({ sessions: rows, view: { ...view, aggregate: true } });
    const user = userEvent.setup();
    await user.click(screen.getByRole("checkbox", { name: "Select Root one" }));
    await user.click(screen.getByTestId("session-sidebar-session-f1"));
    expect(screen.getByTestId("session-sidebar-selection-count")).toHaveTextContent("2 selected");
  });

  // Invariant: where no single pair owns the list, New folder stays visible but
  // disabled with the reason, instead of silently missing.
  it("Should show New folder disabled with its reason in a flat aggregate", async () => {
    const user = userEvent.setup();
    const reason = "Folders live inside one project and profile. Group by project to add one.";
    renderList({
      sessions: [{ ...layoutRows[0]!, profile_id: "default" }],
      view: { ...view, aggregate: true, scopeLabel: null, newFolderUnavailable: reason },
    });
    await user.click(screen.getByTestId("session-sidebar-new"));
    const item = await screen.findByTestId("session-sidebar-new-folder");
    expect(item).toHaveAttribute("aria-disabled", "true");
    expect(screen.getByTestId("session-sidebar-new-folder-unavailable")).toHaveTextContent(reason);
  });

  it("Should explain an aggregate empty search", async () => {
    const rows = [{ ...layoutRows[0]!, profile_id: "default" }];
    renderList({ sessions: rows, view: { ...view, aggregate: true, scopeLabel: null } });
    await userEvent
      .setup()
      .type(screen.getByRole("searchbox", { name: "Filter sessions" }), "unmatched query");
    expect(screen.getByText(/No matching sessions/)).toBeVisible();
    expect(screen.queryByTestId("session-sidebar-session-r1")).not.toBeInTheDocument();
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
      placeholder: false,
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

  // Invariant: dropping a top-level row onto another row (its middle while rows
  // reorder, anywhere otherwise) adopts it as that row's child; a session that
  // already has a parent never offers the drop.
  it("Should adopt a top-level session by dropping it onto another row", () => {
    const onAdopt = vi.fn();
    const rows = [
      ...layoutRows.map(session => ({ ...session, lineage: undefined })),
      {
        ...layoutRows[0]!,
        id: "r3",
        name: "Has parent",
        lineage: { ...layoutRows[0]!.lineage!, parent_session_id: "elsewhere", kind: "provenance" },
      },
    ] as SessionPayload[];
    renderList({
      sessions: rows,
      view: { ...view, sort: "created", layout: fakeLayout() },
      actions: { onAdopt },
    });
    fireEvent.dragStart(rowContainer("r1"));
    fireEvent.dragOver(rowContainer("r2"));
    expect(rowContainer("r2")).toHaveAttribute("data-drop", "into");
    fireEvent.drop(rowContainer("r2"));
    expect(onAdopt).toHaveBeenCalledWith(
      expect.objectContaining({ id: "r1" }),
      expect.objectContaining({ id: "r2" })
    );
    expect(rowContainer("r2")).not.toHaveAttribute("data-drop");

    fireEvent.dragStart(rowContainer("r3"));
    fireEvent.dragOver(rowContainer("r2"));
    expect(rowContainer("r2")).not.toHaveAttribute("data-drop");
    fireEvent.drop(rowContainer("r2"));
    expect(onAdopt).toHaveBeenCalledTimes(1);
  });

  // Owning layer: session list interaction. Nesting is independent of folder layout;
  // nested user rows remain destinations, and keyboard and drag use the same adopt action.
  it.each([false, true])("Should nest onto a child with folder layout %s", withLayout => {
    const onAdopt = vi.fn();
    const parent = { ...layoutRows[0]!, lineage: undefined };
    const source = { ...layoutRows[1]!, lineage: undefined };
    const child = {
      ...layoutRows[2]!,
      id: "nest-child",
      name: "Nested destination",
      lineage: {
        ...layoutRows[0]!.lineage!,
        parent_session_id: parent.id,
        root_session_id: parent.id,
        spawn_depth: 1,
        kind: "provenance",
      },
    } as SessionPayload;
    renderList({
      sessions: [parent, source, child],
      view: { ...view, sort: "manual", layout: withLayout ? fakeLayout() : undefined },
      actions: { onAdopt },
    });
    fireEvent.dragStart(rowContainer(source.id));
    fireEvent.dragOver(rowContainer(child.id));
    expect(rowContainer(child.id)).toHaveAttribute("data-drop", "into");
    fireEvent.drop(rowContainer(child.id));
    expect(onAdopt).toHaveBeenCalledWith(source, child);
    // An ancestor cannot move beneath its own child.
    fireEvent.dragStart(rowContainer(parent.id));
    fireEvent.dragOver(rowContainer(child.id));
    expect(rowContainer(child.id)).not.toHaveAttribute("data-drop");
  });

  it("Should nest through the keyboard menu using only eligible parent choices", async () => {
    const user = userEvent.setup();
    const onAdopt = vi.fn();
    const source = { ...layoutRows[0]!, lineage: undefined };
    const target = { ...layoutRows[1]!, name: "Destination", lineage: undefined };
    const foreign = { ...target, id: "foreign", name: "Foreign", profile_id: "another-profile" };
    renderList({ sessions: [source, target, foreign], actions: { onAdopt } });
    screen.getByTestId(`session-row-actions-${source.id}`).focus();
    await user.keyboard("{Enter}");
    await user.keyboard("Nest{ArrowRight}");
    expect(await screen.findByRole("menuitem", { name: "Destination" })).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: "Foreign" })).not.toBeInTheDocument();
    await user.keyboard("{Enter}");
    expect(onAdopt).toHaveBeenCalledWith(source, target);
  });

  it("Should offer Make top-level only on a nested provenance child", async () => {
    const user = userEvent.setup();
    const onMakeTopLevel = vi.fn();
    const parent = { ...layoutRows[0]!, lineage: undefined };
    const child = {
      ...layoutRows[2]!,
      id: "c-prov",
      name: "Nested child",
      lineage: {
        ...layoutRows[0]!.lineage!,
        parent_session_id: parent.id,
        root_session_id: parent.id,
        spawn_depth: 1,
        kind: "provenance",
      },
    } as SessionPayload;
    renderList({ sessions: [parent, child], actions: { onMakeTopLevel } });

    await user.click(screen.getByTestId("session-row-actions-r1"));
    expect(screen.queryByTestId("session-row-make-top-level-r1")).not.toBeInTheDocument();
    await user.keyboard("{Escape}");
    await user.click(screen.getByTestId("session-row-actions-c-prov"));
    await user.click(await screen.findByTestId("session-row-make-top-level-c-prov"));
    expect(onMakeTopLevel).toHaveBeenCalledWith(expect.objectContaining({ id: "c-prov" }));
  });

  // Invariant: a selection moves into a folder or the root through the bar's
  // Move to folder menu or by dragging a selected row, as one layout write of
  // its thread roots in on-screen order; success ends the selection, a refusal
  // keeps it, and a selection that cannot move says why.
  const rowNames: Record<string, string> = {
    r1: "Root one",
    r2: "Root two",
    f1: "Filed one",
    c1: "Child one",
  };
  /** Enters selection with the first row's checkbox, then toggles the rest by click. */
  async function selectRows(...ids: string[]) {
    // Submenus open on top of their parent menu, whose guard layer the pointer check would hit.
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    const [first, ...rest] = ids;
    await user.click(screen.getByRole("checkbox", { name: `Select ${rowNames[first!]}` }));
    for (const id of rest) await user.click(screen.getByTestId(`session-sidebar-session-${id}`));
    return user;
  }
  /**
   * Opens the selection bar's More menu and, from the keyboard, its Move to
   * folder submenu; resolves once focus sits inside the submenu.
   */
  async function openMoveMenu(user: ReturnType<typeof userEvent.setup>) {
    await user.click(screen.getByTestId("session-sidebar-selection-more"));
    const trigger = await screen.findByTestId("session-sidebar-selection-move");
    await waitFor(() => expect(trigger.closest('[role="menu"]')).toBeInTheDocument());
    trigger.focus();
    await user.keyboard("{ArrowRight}");
    await waitFor(() => {
      const submenu = document.activeElement?.closest('[role="menu"]');
      expect(submenu).toBeTruthy();
      expect(submenu).not.toContainElement(trigger);
    });
  }
  /** Activates one submenu target from the keyboard, as the row menu suite does. */
  async function chooseMoveTarget(user: ReturnType<typeof userEvent.setup>, name: string) {
    const item = await screen.findByRole("menuitem", { name });
    item.focus();
    await user.keyboard("{Enter}");
  }

  it("Should move the selection to a folder from the selection menu and end the selection", async () => {
    const { layout } = renderLayout("created");
    const user = await selectRows("r1", "f1");
    await openMoveMenu(user);
    const items = await screen.findAllByTestId(/^session-sidebar-selection-move-/);
    expect(items.map(item => item.textContent)).toEqual([
      "Alpha",
      "Beta",
      "No folder",
      "New folder…",
    ]);
    await chooseMoveTarget(user, "Beta");

    await waitFor(() => expect(layout.moveSessions).toHaveBeenCalledWith(["f1", "r1"], "fb"));
    await waitFor(() =>
      expect(screen.queryByTestId("session-sidebar-selection-bar")).not.toBeInTheDocument()
    );
  });

  it("Should skip targets that change nothing and keep the selection when the move is refused", async () => {
    let refuse: (error: Error) => void = () => undefined;
    const layout = fakeLayout({
      moveSessions: vi.fn(
        () =>
          new Promise<void>((_resolve, reject) => {
            refuse = reject;
          })
      ),
    });
    renderList({ sessions: layoutRows, view: { ...view, layout } });
    const user = await selectRows("f1");
    await openMoveMenu(user);
    expect(await screen.findByRole("menuitem", { name: "Alpha" })).toHaveAttribute(
      "aria-disabled",
      "true"
    );
    await chooseMoveTarget(user, "No folder");

    expect(layout.moveSessions).toHaveBeenCalledWith(["f1"], null);
    await waitFor(() =>
      expect(screen.getByTestId("session-sidebar-selection-more")).toBeDisabled()
    );
    await act(async () => refuse(new Error("layout changed concurrently")));
    expect(screen.getByTestId("session-sidebar-selection-count")).toHaveTextContent("1 selected");
    expect(screen.getByTestId("session-sidebar-selection-more")).toBeEnabled();
  });

  it("Should create a folder around the selection and move it in", async () => {
    const { layout } = renderLayout("created");
    const user = await selectRows("r1", "r2");
    await openMoveMenu(user);
    await chooseMoveTarget(user, "New folder…");
    expect(await screen.findByText("The 2 sessions you selected move into it.")).toBeVisible();
    await user.type(screen.getByTestId("session-folder-name"), "Launch");
    await user.click(screen.getByRole("button", { name: "Create and move" }));

    expect(layout.createFolder).toHaveBeenCalledWith("Launch");
    await waitFor(() => expect(layout.moveSessions).toHaveBeenCalledWith(["r1", "r2"], "fa"));
    await waitFor(() =>
      expect(screen.queryByTestId("session-folder-name-dialog")).not.toBeInTheDocument()
    );
    expect(screen.queryByTestId("session-sidebar-selection-bar")).not.toBeInTheDocument();
  });

  it("Should say a selection of child rows moves with its parent", async () => {
    const child = {
      ...layoutRows[2]!,
      id: "c1",
      name: "Child one",
      lineage: {
        parent_session_id: "r1",
        root_session_id: "r1",
        spawn_depth: 1,
        auto_stop_on_parent: false,
        notify_creator: false,
        spawn_budget: { max_children: 0, max_depth: 0, ttl_seconds: 0 },
        permission_policy: { tools: [], skills: [], mcp_servers: [], workspace_paths: [] },
      },
    };
    const layout = fakeLayout();
    renderList({ sessions: [...layoutRows, child], view: { ...view, layout } });
    const user = await selectRows("c1");
    await openMoveMenu(user);

    // The reason takes focus, so keyboard and screen reader users hear it.
    expect(
      await screen.findByRole("menuitem", { name: "Child sessions move with their parent." })
    ).toHaveFocus();
    expect(screen.queryByRole("menuitem", { name: "Beta" })).not.toBeInTheDocument();
  });

  it("Should drag the selection onto a folder, and drag an unselected row alone", async () => {
    const { layout } = renderLayout("created");
    await selectRows("r1");
    fireEvent.dragStart(rowContainer("r2"));
    fireEvent.drop(screen.getByTestId("session-sidebar-folder-fb-header"));
    expect(layout.moveSession).toHaveBeenCalledWith("r2", "fb");
    expect(layout.moveSessions).not.toHaveBeenCalled();

    fireEvent.dragStart(rowContainer("r1"));
    expect(rowContainer("r1")).toHaveAttribute("data-dragging", "true");
    // Rows never take a dragged selection, so it lands on the folder around them.
    fireEvent.dragOver(rowContainer("f1"));
    expect(screen.getByTestId("session-sidebar-folder-fa-header")).toHaveAttribute(
      "data-drop",
      "into"
    );
    fireEvent.drop(rowContainer("f1"));
    await waitFor(() => expect(layout.moveSessions).toHaveBeenCalledWith(["r1"], "fa"));
    expect(layout.reorderSessions).not.toHaveBeenCalled();
  });

  it.each([
    ["another bulk operation starts", "busy"],
    ["a carried session leaves the list", "pruned"],
  ])("Should refuse a selection drop once %s mid-drag", async (_case, change) => {
    const layout = fakeLayout();
    const layoutView = { ...view, sort: "created" as const, layout };
    const { actions, update } = renderList({ sessions: layoutRows, view: layoutView });
    await selectRows("r1", "r2");
    fireEvent.dragStart(rowContainer("r1"));
    if (change === "busy") {
      (actions as { pendingAction: string | null }).pendingAction = "stop";
      update(layoutRows, layoutView);
    } else {
      update(
        layoutRows.filter(row => row.id !== "r2"),
        layoutView
      );
    }

    fireEvent.dragOver(screen.getByTestId("session-sidebar-folder-fb"));
    expect(screen.getByTestId("session-sidebar-folder-fb-header")).not.toHaveAttribute("data-drop");
    fireEvent.drop(screen.getByTestId("session-sidebar-folder-fb"));
    expect(layout.moveSessions).not.toHaveBeenCalled();
  });

  it("Should take a dragged selection back to the root, writing only its filed sessions", async () => {
    const { layout } = renderLayout("created");
    await selectRows("r2", "f1");
    fireEvent.dragStart(rowContainer("f1"));
    expect(rowContainer("r2")).toHaveAttribute("data-dragging", "true");
    fireEvent.dragOver(screen.getByTestId("session-sidebar-folder-fa"));
    expect(screen.getByTestId("session-sidebar-folder-fa-header")).toHaveAttribute(
      "data-drop",
      "into"
    );
    fireEvent.drop(screen.getByTestId("session-sidebar-root"));
    await waitFor(() => expect(layout.moveSessions).toHaveBeenCalledWith(["f1"], null));
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

// Invariants (ADR-004): under All projects one flat query lists every row with its project
// (W-08); Group by renders facets-driven groups that read and page their own rows, read-only,
// and a collapsed header still signals needs-you (W-09); one failed group retries in place while
// a failed flat list states one list-level Retry (W-24); manual order is served only to a group
// whose own query is one pair (W-12). Session catalog I/O is the mocked boundary.
describe("SessionList breadth and Group by", () => {
  const NAMES = new Map([
    ["ws-alpha", "Alpha"],
    ["ws-beta", "Beta"],
  ]);
  const WORKSPACES = [
    { id: "ws-alpha", name: "Alpha" },
    { id: "ws-beta", name: "Beta" },
  ];
  const lineage = (parent: string) => ({
    parent_session_id: parent,
    root_session_id: parent,
    spawn_depth: 1,
    auto_stop_on_parent: false,
    notify_creator: false,
    spawn_budget: { max_children: 0, max_depth: 0, ttl_seconds: 0 },
    permission_policy: { tools: [], skills: [], mcp_servers: [], workspace_paths: [] },
  });
  const row = (id: string, workspaceId: string, patch: Partial<SessionPayload> = {}) =>
    ({
      ...primarySessionFixture,
      id,
      name: `Session ${id}`,
      archived_at: null,
      state: "active",
      badge: "running",
      workspace_id: workspaceId,
      profile_id: "default",
      profile_name: "default",
      ...patch,
    }) as SessionPayload;
  const facet = (all: number, needsYou = 0) => ({
    terminal_approvals: 0,
    all,
    working: 0,
    needs_you: needsYou,
    finished: 0,
    idle: all - needsYou,
    board: { needs_you: 0, working: 0, waiting: 0, in_review: 0, done: 0 },
  });
  const ROWS = [
    row("a1", "ws-alpha"),
    row("a2", "ws-alpha", { profile_id: "p-work", profile_name: "work" }),
    row("b1", "ws-beta"),
    row("u1", ""),
  ];
  const PROFILES = [
    { id: "default", name: "default", color: "", icon: "", emoji: "", archived_at: null },
    { id: "p-work", name: "work", color: "", icon: "", emoji: "", archived_at: null },
  ];
  const FOLDER = {
    id: "fold-new",
    name: "",
    workspace_id: "ws-alpha",
    profile_id: "p-work",
    position: 0,
    session_count: 0,
    created_at: "2026-10-22T00:00:00Z",
    updated_at: "2026-10-22T00:00:00Z",
  };
  /** Facets the daemon would count for one read: its scope and lens narrow the rows. */
  function facetsFor(url: URL) {
    const selector = url.searchParams.get("workspace_id");
    const profile = url.searchParams.get("profile");
    const rows = ROWS.filter(
      session =>
        (!selector || session.workspace_id === (selector === "@user" ? "" : selector)) &&
        (!profile || session.profile_name === profile)
    );
    const count = (match: (session: SessionPayload) => boolean) => facet(rows.filter(match).length);
    return {
      facets: count(() => true),
      by_workspace: [...new Set(rows.map(session => session.workspace_id))].map(id => ({
        workspace_id: id,
        facets: count(session => session.workspace_id === id),
      })),
      by_profile: [...new Set(rows.map(session => session.profile_id))].map(id => ({
        profile_id: id,
        facets: count(session => session.profile_id === id),
      })),
    };
  }
  const badgeOf = (session: SessionPayload) => workspaceBadgeIdentity(session.workspace_id, NAMES);
  const projectGroups = () =>
    projectSessionGroups(
      {
        by_workspace: [
          { workspace_id: "ws-alpha", facets: facet(1) },
          { workspace_id: "ws-beta", facets: facet(4, 2) },
          { workspace_id: "", facets: facet(1) },
        ],
      },
      { workspaces: WORKSPACES, lens: { profile: "default" } }
    );
  const grouped = (overrides: Partial<SessionListViewModel> = {}) =>
    sessionListViewFixture({
      ...view,
      storedGroupBy: "project",
      groupBy: "project",
      groupByOptions: ["none", "project"],
      groupFilters: { limit: 100 },
      badgeOf,
      groups: projectGroups(),
      ...overrides,
    });

  const failing = new Set<string>();
  /** Session folders the daemon lists, by the workspace id in the request path. */
  const listedFolders = new Map<string, unknown[]>();
  let fetcher: ReturnType<typeof vi.spyOn<typeof globalThis, "fetch">>;
  beforeEach(() => {
    failing.clear();
    listedFolders.clear();
    fetcher = vi.spyOn(globalThis, "fetch").mockImplementation(async input => {
      const url = new URL((input as Request).url);
      const json = (body: unknown, status = 200) =>
        new Response(JSON.stringify(body), {
          status,
          headers: { "content-type": "application/json" },
        });
      if (url.pathname.endsWith("/session-folders")) {
        if ((input as Request).method === "POST") {
          const body = (await (input as Request).json()) as { name: string };
          return json({ folder: { ...FOLDER, name: body.name } }, 201);
        }
        const workspaceId = url.pathname.split("/").at(-2) ?? "";
        return json({ folders: listedFolders.get(workspaceId) ?? [] });
      }
      if (url.pathname === "/api/profiles") return json(PROFILES);
      if (url.pathname === "/api/workspaces") {
        return json({
          workspaces: WORKSPACES.map(workspace => ({
            ...workspace,
            root_dir: `/projects/${workspace.id}`,
            archived_at: null,
          })),
        });
      }
      if (url.pathname === "/api/sessions/facets") return json(facetsFor(url));
      if (url.pathname !== "/api/sessions") return json({ error: "not found" }, 404);
      const selector = url.searchParams.get("workspace_id");
      if (selector && failing.has(selector)) return json({ error: "unreachable" }, 503);
      const profile = url.searchParams.get("profile");
      const listed = ROWS.filter(
        session =>
          (!selector || session.workspace_id === (selector === "@user" ? "" : selector)) &&
          (!profile || session.profile_name === profile)
      );
      return json({ sessions: listed, page: { has_more: false, limit: 100 } });
    });
  });
  afterEach(() => fetcher.mockRestore());

  function catalogReads() {
    return fetcher.mock.calls
      .map(([input]) => new URL((input as Request).url))
      .filter(url => url.pathname === "/api/sessions");
  }

  it("W-08: Should list All projects flat, naming each row's project before its agent", () => {
    const child = row("a1-child", "ws-alpha", { lineage: lineage("a1") });
    const { update } = renderList({
      sessions: [ROWS[0]!, child, ROWS[3]!],
      view: { ...view, badgeOf },
    });

    const alpha = screen.getByTestId("session-row-project-a1");
    expect(alpha).toHaveTextContent("Alpha");
    expect(screen.getByTestId("session-row-project-u1")).toHaveTextContent("User");
    expect(
      alpha.compareDocumentPosition(screen.getByTestId("session-provider-mark-a1")) &
        Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy();
    // A child in its root's project repeats nothing.
    expect(screen.queryByTestId("session-row-project-a1-child")).not.toBeInTheDocument();
    expect(catalogReads()).toEqual([]);

    // A focused project names no project on its rows.
    update([ROWS[0]!, child, ROWS[3]!], view);
    expect(screen.queryByTestId("session-row-project-a1")).not.toBeInTheDocument();
  });

  // #952: Sort and Group by are two explicit controls; Group by exists only where a grouping applies.
  it("Should offer Group by as its own control only where a grouping applies", async () => {
    const user = userEvent.setup();
    const setGroupBy = vi.fn();
    const flat = renderList({ view: { ...view, groupByOptions: ["none"], setGroupBy } });
    expect(screen.getByRole("button", { name: /^Sort by/ })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^Group by/ })).not.toBeInTheDocument();
    flat.update(sessions, { ...view, groupByOptions: ["none", "project"], setGroupBy });
    await user.click(screen.getByRole("button", { name: "Group by: None" }));
    expect(await screen.findByTestId("session-sidebar-group-by-none")).toHaveAttribute(
      "aria-checked",
      "true"
    );
    expect(screen.queryByTestId("session-sidebar-group-by-profile")).not.toBeInTheDocument();
    // The Group by menu holds only groupings; sorts live behind their own icon.
    expect(screen.queryByRole("menuitemradio", { name: "Created" })).not.toBeInTheDocument();
    await user.click(screen.getByTestId("session-sidebar-group-by-project"));
    expect(setGroupBy).toHaveBeenCalledWith("project");
  });

  // #1085: "Show empty projects" belongs to project grouping and reports the operator's choice.
  it.each([
    { checked: false, next: true },
    { checked: true, next: false },
  ])(
    "Should offer Show empty projects only while grouped by project (checked: $checked)",
    async ({ checked, next }) => {
      // The first menu's close leaves its pointer lock for a frame; the reopen does not wait on it.
      const user = userEvent.setup({ pointerEventsCheck: 0 });
      const setShowEmptyProjects = vi.fn();
      const list = renderList({
        sessions: [],
        view: grouped({ showEmptyProjects: checked, setShowEmptyProjects }),
      });
      await user.click(screen.getByRole("button", { name: "Group by: Project" }));
      const toggle = await screen.findByRole("menuitemcheckbox", { name: "Show empty projects" });
      expect(toggle).toHaveAttribute("aria-checked", String(checked));
      await user.click(toggle);
      expect(setShowEmptyProjects).toHaveBeenCalledWith(next);
      await user.keyboard("{Escape}");

      list.update(
        [],
        grouped({ groupBy: "none", storedGroupBy: "none", groups: [], setShowEmptyProjects })
      );
      await user.click(screen.getByRole("button", { name: "Group by: None" }));
      expect(await screen.findByRole("menuitemradio", { name: "Project" })).toBeInTheDocument();
      expect(screen.queryByRole("menuitemcheckbox", { name: "Show empty projects" })).toBeNull();
    }
  );

  // #1085: an empty project skips its page but keeps its session folders, so a folder made
  // from its + shows at once.
  it("Should keep an empty project's session folders while skipping its page", async () => {
    listedFolders.set("ws-gamma", [
      {
        ...FOLDER,
        id: "fold-ideas",
        name: "Ideas",
        workspace_id: "ws-gamma",
        profile_id: "default",
      },
    ]);
    const empty = projectSessionGroup(
      "ws-gamma",
      { all: 0, working: 0, needs_you: 0 },
      { workspaces: [{ id: "ws-gamma", name: "Gamma" }], lens: { profile: "default" } }
    );
    renderList({ sessions: [], view: grouped({ groups: [empty], showEmptyProjects: true }) });

    expect(await screen.findByText("Ideas")).toBeVisible();
    expect(catalogReads().map(url => url.searchParams.get("workspace_id"))).not.toContain(
      "ws-gamma"
    );
  });

  // #1085: a project whose sessions all left the view drops its cached rows: its page read
  // stops refreshing at a zero count, so those rows could be archived or deleted.
  it("Should drop a project's cached rows once its count reaches zero", async () => {
    const alpha = (total: number) =>
      projectSessionGroup(
        "ws-alpha",
        { all: total, working: 0, needs_you: 0 },
        { workspaces: WORKSPACES, lens: { profile: "default" } }
      );
    const list = renderList({ sessions: [], view: grouped({ groups: [alpha(1)] }) });
    expect(await screen.findByTestId("session-sidebar-session-a1")).toBeVisible();

    list.update([], grouped({ groups: [alpha(0)], showEmptyProjects: true }));
    expect(await screen.findByText("No sessions in Alpha yet.")).toBeVisible();
    expect(screen.queryByTestId("session-sidebar-session-a1")).not.toBeInTheDocument();
  });

  it("Should name the project, never the profile, in an empty project group", async () => {
    const empty = projectSessionGroup(
      "ws-gamma",
      { all: 0, working: 0, needs_you: 0 },
      { workspaces: [{ id: "ws-gamma", name: "Gamma" }], lens: { profile: "default" } }
    );
    renderList({ sessions: [], view: grouped({ groups: [empty], showEmptyProjects: true }) });

    const note = await screen.findByText("No sessions in Gamma yet.");
    expect(note).toBeVisible();
    // A quiet one-line note inside the group, not the list's full empty-state block.
    expect(note.closest("[data-slot='empty']")).toBeNull();
    // An empty project reads no page of its own.
    expect(catalogReads().map(url => url.searchParams.get("workspace_id"))).not.toContain(
      "ws-gamma"
    );
    expect(screen.getByTestId("session-sidebar-group-project:ws-gamma")).toHaveTextContent("0");
    expect(screen.queryByText(/in default/)).not.toBeInTheDocument();
  });

  it("W-09: Should group by project from facets, each open group reading its own rows", async () => {
    const user = userEvent.setup();
    const toggleGroup = vi.fn();
    renderList({
      sessions: [],
      view: grouped({ collapsedGroupKeys: new Set(["project:ws-beta"]), toggleGroup }),
    });

    expect(await screen.findByTestId("session-sidebar-session-a1")).toBeVisible();
    expect(await screen.findByTestId("session-sidebar-session-u1")).toBeVisible();
    const headers = [
      ...document.querySelectorAll<HTMLElement>(
        "[data-testid^='session-sidebar-group-'][aria-expanded]:not([aria-haspopup])"
      ),
    ];
    expect(headers.map(header => header.dataset.testid)).toEqual([
      "session-sidebar-group-project:ws-alpha",
      "session-sidebar-group-project:ws-beta",
      "session-sidebar-group-user",
    ]);
    const beta = screen.getByTestId("session-sidebar-group-project:ws-beta");
    expect(beta).toHaveAttribute("aria-expanded", "false");
    expect(beta).toHaveTextContent("4");
    expect(within(beta).getByRole("img", { name: "2 need you" })).toBeInTheDocument();
    // A collapsed group reads nothing; rows inside a project group repeat no project.
    expect(
      catalogReads()
        .map(url => url.searchParams.get("workspace_id"))
        .sort()
    ).toEqual(["@user", "ws-alpha"]);
    expect(screen.queryByTestId("session-row-project-a1")).not.toBeInTheDocument();

    await user.click(beta);
    expect(toggleGroup).toHaveBeenCalledWith("project:ws-beta");
  });

  it("W-24: Should retry one failed group in place while the other groups render", async () => {
    failing.add("ws-alpha");
    renderList({ sessions: [], view: grouped() });

    const retry = await screen.findByTestId("session-sidebar-group-project:ws-alpha-retry");
    expect(await screen.findByTestId("session-sidebar-session-b1")).toBeVisible();
    expect(screen.getByTestId("session-sidebar-session-u1")).toBeVisible();
    expect(screen.getAllByRole("button", { name: /Retry/ })).toHaveLength(1);

    failing.delete("ws-alpha");
    await userEvent.setup().click(retry);
    expect(await screen.findByTestId("session-sidebar-session-a1")).toBeVisible();
    expect(screen.queryByTestId("session-sidebar-group-project:ws-alpha-retry")).toBeNull();
  });

  it("W-24: Should state one list-level error with one Retry when the flat list fails", async () => {
    const retry = vi.fn();
    renderList({
      sessions: [],
      view: { ...view, badgeOf, catalog: sessionListCatalogFixture({ failed: true, retry }) },
    });
    expect(screen.getByText("Couldn’t load sessions.")).toBeVisible();
    const buttons = screen.getAllByRole("button", { name: /Retry/ });
    expect(buttons).toHaveLength(1);
    await userEvent.setup().click(buttons[0]!);
    expect(retry).toHaveBeenCalledOnce();
  });

  it.each([
    ["one project", { kind: "project", workspaceId: "ws-alpha" }, ["ws-alpha"]],
    ["All projects", { kind: "all" }, ["@user", "ws-alpha", "ws-beta"]],
  ] as const)(
    "W-12: Should read a profile group in %s in manual order, one project pair at a time",
    async (_case, scope: DataScope, workspaces) => {
      const owner = { id: "default", name: "default", archived: false };
      const groups = profileSessionGroups(
        { by_profile: [{ profile_id: "default", facets: facet(1) }] },
        { profiles: [owner], scope }
      );
      renderList({
        sessions: [],
        view: grouped({
          sort: "manual",
          aggregate: true,
          scopeLabel: null,
          storedGroupBy: "profile",
          groupBy: "profile",
          groups,
        }),
      });
      // A profile group spanning projects splits into one group per project pair.
      await waitFor(() => expect(catalogReads()).toHaveLength(workspaces.length));
      const reads = catalogReads();
      expect(reads.map(url => url.searchParams.get("workspace_id")).sort()).toEqual(workspaces);
      expect(reads.every(url => url.searchParams.get("sort") === "manual")).toBe(true);
      expect(reads.every(url => url.searchParams.get("profile") === "default")).toBe(true);
      expect(screen.queryByTestId("session-sidebar-sort-note")).not.toBeInTheDocument();
    }
  );

  // Invariant: under All projects and All profiles, a project group splits into one
  // subgroup per profile — each one (workspace, profile) pair that owns folders and order.
  it("Should split a project group under All profiles into profile pairs", async () => {
    const groups = projectSessionGroups(
      { by_workspace: [{ workspace_id: "ws-alpha", facets: facet(2) }] },
      { workspaces: WORKSPACES, lens: { all_profiles: true } }
    );
    expect(groups[0]!.pair).toBeNull();
    renderList({
      sessions: [],
      view: grouped({ sort: "manual", aggregate: true, scopeLabel: null, groups }),
    });

    expect(
      await screen.findByTestId("session-sidebar-group-project:ws-alpha/profile:default")
    ).toBeVisible();
    expect(
      screen.getByTestId("session-sidebar-group-project:ws-alpha/profile:p-work")
    ).toBeVisible();
    expect(await screen.findByTestId("session-sidebar-session-a1")).toBeVisible();
    expect(await screen.findByTestId("session-sidebar-session-a2")).toBeVisible();
    const reads = catalogReads();
    expect(reads.map(url => url.searchParams.get("profile")).sort()).toEqual(["default", "work"]);
    expect(reads.every(url => url.searchParams.get("sort") === "manual")).toBe(true);
  });

  // Invariant: a group header's + opens New session in the group's project (and
  // profile, when the group settles one), as a sibling of the collapse toggle.
  it("Should start a session from a group header + in that group's project and profile", async () => {
    const user = userEvent.setup();
    const onNewSession = vi.fn();
    const toggleGroup = vi.fn();
    renderList({ sessions: [], onNewSession, view: grouped({ toggleGroup }) });

    const header = await screen.findByTestId("session-sidebar-group-project:ws-alpha");
    expect(within(header).queryByRole("button", { name: /New session/ })).toBeNull();
    await user.click(screen.getByTestId("session-sidebar-group-project:ws-alpha-new"));
    await user.click(
      await screen.findByTestId("session-sidebar-group-project:ws-alpha-new-session")
    );
    expect(onNewSession).toHaveBeenCalledWith({ workspaceId: "ws-alpha", profile: "default" });
    expect(toggleGroup).not.toHaveBeenCalled();
  });

  it("Should ask which profile a group + New folder belongs to under All profiles", async () => {
    const user = userEvent.setup();
    const groups = projectSessionGroups(
      { by_workspace: [{ workspace_id: "ws-alpha", facets: facet(2) }] },
      { workspaces: WORKSPACES, lens: { all_profiles: true } }
    );
    renderList({ sessions: [], view: grouped({ aggregate: true, scopeLabel: null, groups }) });

    await user.click(await screen.findByTestId("session-sidebar-group-project:ws-alpha-new"));
    await user.click(
      await screen.findByTestId("session-sidebar-group-project:ws-alpha-new-folder")
    );
    const picker = await screen.findByTestId(
      "session-sidebar-group-project:ws-alpha-folder-profile"
    );
    await user.click(picker);
    await user.click(
      await screen.findByTestId("session-sidebar-group-project:ws-alpha-folder-profile-item-work")
    );
    await user.type(await screen.findByTestId("session-folder-name"), "Research");
    await user.click(screen.getByTestId("session-folder-name-confirm"));

    await waitFor(() => {
      const create = fetcher.mock.calls
        .map(([input]) => input as Request)
        .find(request => request.method === "POST");
      expect(create).toBeDefined();
      const url = new URL(create!.url);
      expect(url.pathname).toBe("/api/workspaces/ws-alpha/session-folders");
      expect(url.searchParams.get("profile")).toBe("work");
    });
  });

  // #1029: a project group header is a project surface: right-click and Shift+F10 open the
  // project's own actions and confirm; the User group stands for no project and has none.
  it("Should open the project's actions from its group header", async () => {
    const user = userEvent.setup();
    renderList({ sessions: [], view: grouped() });

    const toggle = await screen.findByTestId("session-sidebar-group-project:ws-alpha");
    // The header's trigger is inert until the project catalog answers.
    await waitFor(() => {
      fireEvent.contextMenu(toggle);
      expect(screen.getByRole("menuitem", { name: "Archive project" })).toBeVisible();
    });
    await user.keyboard("{Escape}");

    toggle.focus();
    await user.keyboard("{Shift>}{F10}{/Shift}");
    await user.click(await screen.findByRole("menuitem", { name: "Remove project" }));
    expect(await screen.findByRole("dialog", { name: "Remove Alpha?" })).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Cancel" }));

    // No project behind the User group: the click falls through and no project menu opens.
    expect(fireEvent.contextMenu(screen.getByTestId("session-sidebar-group-user"))).toBe(true);
    expect(screen.queryByRole("menuitem", { name: "Archive project" })).not.toBeInTheDocument();
  });

  // A profile group inside a project stands for a profile, not the project: no project menu.
  it("Should keep project actions off a profile group header", async () => {
    const groups = profileSessionGroups(
      { by_profile: [{ profile_id: "p-work", facets: facet(1) }] },
      {
        profiles: [{ id: "p-work", name: "work", archived: false }],
        scope: { kind: "project", workspaceId: "ws-alpha" },
      }
    );
    const { queryClient } = renderList({
      sessions: [],
      view: grouped({ storedGroupBy: "profile", groupBy: "profile", groups }),
    });

    const header = await screen.findByTestId("session-sidebar-group-profile:p-work");
    // A populated catalog makes the negative assertion exercise the group-kind guard.
    await waitFor(() =>
      expect(queryClient.getQueryData(workspacesListOptions().queryKey)).toMatchObject(WORKSPACES)
    );
    expect(fireEvent.contextMenu(header)).toBe(true);
    expect(screen.queryByRole("menuitem", { name: "Archive project" })).not.toBeInTheDocument();
  });

  // Invariant: a grouped list keeps one selection across its groups; lifecycle verbs
  // route by owner, while a folder move needs the selection inside one pair.
  it("Should select across groups, run bulk verbs, and refuse a cross-pair folder move", async () => {
    const user = userEvent.setup();
    const { actions } = renderList({ sessions: [], view: grouped() });

    await user.click(await screen.findByRole("checkbox", { name: "Select Session a1" }));
    await user.click(await screen.findByRole("checkbox", { name: "Select Session b1" }));
    expect(screen.getByTestId("session-sidebar-selection-count")).toHaveTextContent("2 selected");
    await user.click(screen.getByTestId("session-sidebar-selection-more"));
    await user.click(await screen.findByTestId("session-sidebar-selection-stop"));
    expect(actions.onStopMany).toHaveBeenCalledWith([
      expect.objectContaining({ id: "a1" }),
      expect.objectContaining({ id: "b1" }),
    ]);
    await user.click(screen.getByTestId("session-sidebar-selection-more"));
    await user.click(await screen.findByTestId("session-sidebar-selection-move"));
    expect(await screen.findByTestId("session-sidebar-selection-move-blocked")).toHaveTextContent(
      "Select sessions from one project and profile to move them."
    );
  });

  // Invariant (#1044): under All projects by project, one profile's project folders sit
  // above the root, hold projects only, and a folder never lands on or in a project.
  const PROJECT_FOLDER = workspaceFolderFixture("wf-1", "Clients");
  function projectFoldered(
    placements: WorkspacePlacementPayload[] = [],
    options: { hideEmptyFolders?: boolean } = {}
  ) {
    const layout = {
      folders: [PROJECT_FOLDER],
      placements,
      createFolder: vi.fn().mockResolvedValue(PROJECT_FOLDER),
      renameFolder: vi.fn().mockResolvedValue(undefined),
      deleteFolder: vi.fn().mockResolvedValue(undefined),
      moveFolder: vi.fn(),
      reorderFolders: vi.fn(),
      moveWorkspace: vi.fn(),
      reorderWorkspaces: vi.fn(),
    };
    const tree = buildProjectGroupTree(projectGroups(), layout, options);
    return {
      layout,
      view: grouped({
        groups: projectGroupTreeGroups(tree),
        projectFolders: { layout, tree },
        newFolderUnavailable: null,
      }),
    };
  }
  function groupHeaders() {
    return [
      ...document.querySelectorAll<HTMLElement>(
        "[data-testid^='session-sidebar-group-'][aria-expanded]:not([aria-haspopup])"
      ),
    ].map(header => header.dataset.testid);
  }

  it("Should hang a foldered project on the folder's rail above the root projects", async () => {
    const { view: foldered } = projectFoldered([
      { workspace_id: "ws-beta", folder_id: "wf-1", position: 0 },
      { workspace_id: "@user", position: 0 },
    ]);
    renderList({ sessions: [], view: foldered });

    const folder = await screen.findByTestId("session-sidebar-project-folder-wf-1");
    expect(within(folder).getByTestId("session-sidebar-group-project:ws-beta")).toBeVisible();
    expect(screen.getByTestId("session-sidebar-project-folder-wf-1-count")).toHaveTextContent("1");
    expect(
      within(folder)
        .getByTestId("session-sidebar-group-project:ws-beta-header")
        .closest("[data-slot='project-folder-member']")
    ).not.toBeNull();
    // Placed root projects lead in their order; never-placed ones follow in switcher order.
    expect(groupHeaders()).toEqual([
      "session-sidebar-group-project:ws-beta",
      "session-sidebar-group-user",
      "session-sidebar-group-project:ws-alpha",
    ]);
    // A project folder starts no session of its own.
    fireEvent.contextMenu(screen.getByTestId("session-sidebar-project-folder-wf-1-header"));
    expect(
      await screen.findByTestId("session-sidebar-project-folder-wf-1-menu-rename")
    ).toBeVisible();
    expect(
      screen.queryByTestId("session-sidebar-project-folder-wf-1-menu-new-session")
    ).not.toBeInTheDocument();
  });

  it("Should create a project folder from the list's New folder", async () => {
    const user = userEvent.setup();
    const { layout, view: foldered } = projectFoldered();
    renderList({ sessions: [], view: foldered });
    await user.click(screen.getByTestId("session-sidebar-new"));
    await user.click(await screen.findByRole("menuitem", { name: "New folder" }));
    await user.type(await screen.findByTestId("session-folder-name"), "Clients");
    await user.click(screen.getByTestId("session-folder-name-confirm"));
    expect(layout.createFolder).toHaveBeenCalledWith("Clients");
  });

  // jsdom reports zero-size boxes, so every pointer lands on the target's lower half ("after").
  it("Should drag projects into, within, and out of a folder, and never a folder into a project", async () => {
    const { layout, view: foldered } = projectFoldered([
      { workspace_id: "ws-beta", folder_id: "wf-1", position: 0 },
    ]);
    renderList({ sessions: [], view: foldered });
    const header = (key: string) => screen.getByTestId(`session-sidebar-group-${key}-header`);
    const folderHeader = await screen.findByTestId("session-sidebar-project-folder-wf-1-header");

    fireEvent.dragStart(header("project:ws-alpha"));
    fireEvent.dragOver(folderHeader);
    expect(folderHeader).toHaveAttribute("data-drop", "into");
    fireEvent.drop(folderHeader);
    expect(layout.moveWorkspace).toHaveBeenCalledWith("ws-alpha", "wf-1");

    fireEvent.dragStart(header("project:ws-alpha"));
    fireEvent.dragOver(header("user"));
    expect(header("user")).toHaveAttribute("data-drop", "after");
    fireEvent.drop(header("user"));
    expect(layout.reorderWorkspaces).toHaveBeenCalledWith(null, ["@user", "ws-alpha"]);

    fireEvent.dragStart(header("project:ws-beta"));
    const root = screen.getByTestId("session-sidebar-project-root");
    fireEvent.dragOver(root);
    expect(root).toHaveAttribute("data-drop", "into");
    fireEvent.drop(root);
    expect(layout.moveWorkspace).toHaveBeenCalledWith("ws-beta", null);

    // A folder offers no drop on a project, and lands nowhere it is let go of there.
    fireEvent.dragStart(folderHeader);
    fireEvent.dragOver(header("project:ws-alpha"));
    expect(header("project:ws-alpha")).not.toHaveAttribute("data-drop");
    fireEvent.drop(header("project:ws-alpha"));
    fireEvent.dragOver(root);
    expect(root).not.toHaveAttribute("data-drop");
    expect(layout.reorderFolders).not.toHaveBeenCalled();
    expect(layout.reorderWorkspaces).toHaveBeenCalledTimes(1);
  });

  it("Should move a project between folders from its header menu", async () => {
    const user = userEvent.setup();
    const { layout, view: foldered } = projectFoldered([
      { workspace_id: "ws-beta", folder_id: "wf-1", position: 0 },
    ]);
    renderList({ sessions: [], view: foldered });
    fireEvent.contextMenu(
      await screen.findByTestId("session-sidebar-group-project:ws-alpha-header")
    );
    await user.hover(
      await screen.findByTestId("session-sidebar-group-project:ws-alpha-arrange-move-to")
    );
    // The submenu opens on hover; its item is pressed directly once it mounts.
    fireEvent.click(
      await screen.findByTestId("session-sidebar-group-project:ws-alpha-arrange-move-to-wf-1")
    );
    expect(layout.moveWorkspace).toHaveBeenCalledWith("ws-alpha", "wf-1");

    fireEvent.contextMenu(screen.getByTestId("session-sidebar-group-project:ws-beta-header"));
    await user.click(
      await screen.findByTestId("session-sidebar-group-project:ws-beta-arrange-move-out")
    );
    expect(layout.moveWorkspace).toHaveBeenCalledWith("ws-beta", null);
  });

  // #1085: a folder hidden because its projects are empty still takes a project from the menu.
  it("Should offer a hidden empty folder as a move target", async () => {
    const user = userEvent.setup();
    const { layout, view: foldered } = projectFoldered(
      [{ workspace_id: "ws-gamma", folder_id: "wf-1", position: 0 }],
      { hideEmptyFolders: true }
    );
    renderList({ sessions: [], view: foldered });
    expect(screen.queryByTestId("session-sidebar-project-folder-wf-1")).not.toBeInTheDocument();
    fireEvent.contextMenu(
      await screen.findByTestId("session-sidebar-group-project:ws-alpha-header")
    );
    await user.hover(
      await screen.findByTestId("session-sidebar-group-project:ws-alpha-arrange-move-to")
    );
    fireEvent.click(
      await screen.findByTestId("session-sidebar-group-project:ws-alpha-arrange-move-to-wf-1")
    );
    expect(layout.moveWorkspace).toHaveBeenCalledWith("ws-alpha", "wf-1");
  });

  // Invariant (#1044): the footer states the scope's exact facet totals, only the
  // non-zero ones, and stays out of the way when there is nothing to count.
  it("Should total the scope's sessions by state in a quiet footer", () => {
    const { update } = renderList({
      sessions: [],
      view: grouped({ totals: { needsYou: 1, working: 3, idle: 0 } }),
    });
    const footer = screen.getByTestId("session-sidebar-totals");
    expect(footer).toHaveTextContent("1 needs you3 working");
    expect(screen.queryByTestId("session-sidebar-totals-idle")).not.toBeInTheDocument();
    expect(within(footer).queryByRole("button")).not.toBeInTheDocument();

    update([], grouped({ totals: { needsYou: 0, working: 0, idle: 0 } }));
    expect(screen.queryByTestId("session-sidebar-totals")).not.toBeInTheDocument();
  });
});

// Invariant: a rail that has not answered yet says so with row-shaped placeholders under one busy
// status. It never poses as an empty list, and a refetch over rows it already has never swaps them
// for placeholders. Owner: SessionList through its sidebar host (the canonical suite above).
describe("SessionList loading", () => {
  const loadingView = (overrides: Partial<SessionListViewModel> = {}) => ({
    ...view,
    catalog: sessionListCatalogFixture({ loading: true }),
    ...overrides,
  });

  it("Should hold the rail's place with placeholder rows until the first page lands", () => {
    const { update } = renderList({ sessions: [], view: loadingView() });

    const region = screen.getByTestId("session-list-skeleton");
    expect(region).toHaveAttribute("role", "status");
    expect(region).toHaveAttribute("aria-busy", "true");
    expect(region).toHaveTextContent("Loading sessions");
    expect(
      region.querySelectorAll('[data-slot="session-list-row-skeleton"]').length
    ).toBeGreaterThan(3);
    expect(screen.queryByText(/^No sessions/)).not.toBeInTheDocument();
    // The rail's own controls are real and already usable while its rows load.
    expect(screen.getByRole("searchbox", { name: "Filter sessions" })).toBeEnabled();
    expect(screen.getByTestId("session-sidebar-new-session")).toBeEnabled();

    update(sessions, { ...view, catalog: sessionListCatalogFixture({ sessions }) });

    expect(screen.queryByTestId("session-list-skeleton")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-sidebar-session-running")).toBeVisible();
  });

  it("Should say a settled empty list is empty once nothing is loading", () => {
    renderList({ sessions: [], view: { ...view, catalog: sessionListCatalogFixture() } });

    expect(screen.queryByTestId("session-list-skeleton")).not.toBeInTheDocument();
    expect(screen.getByText(/^No sessions/)).toBeVisible();
  });

  it("Should keep the rows it has while a loaded page refetches or pages", () => {
    renderList({
      sessions,
      view: { ...view, catalog: sessionListCatalogFixture({ sessions, paging: true }) },
    });

    expect(screen.queryByTestId("session-list-skeleton")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-sidebar-session-running")).toBeVisible();
  });

  it("Should hold grouped lists' place with header and row placeholders while their facets load", () => {
    renderList({
      sessions: [],
      view: sessionListViewFixture({
        ...view,
        storedGroupBy: "project",
        groupBy: "project",
        groupByOptions: ["none", "project"],
        groupsLoading: true,
      }),
    });

    const region = screen.getByTestId("session-list-groups-skeleton");
    expect(region).toHaveAttribute("role", "status");
    expect(region).toHaveTextContent("Loading sessions");
    expect(
      region.querySelectorAll('[data-slot="session-list-row-skeleton"]').length
    ).toBeGreaterThan(0);
    expect(screen.queryByText(/^No sessions/)).not.toBeInTheDocument();
  });
});
