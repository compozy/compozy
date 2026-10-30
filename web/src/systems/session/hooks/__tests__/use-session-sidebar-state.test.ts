// Suite: session-window sessions rail state (open/closed, width, folder, thread branch, and group collapse)
// Invariant: rail state is owned per window scope — two session windows never
// share a toggle, a fold (thread, folder, or group-by group), or a width; a
// window without its own entry starts from the last-used (shell) state; a
// persisted entry survives a reload; a width stays
// within bounds and a reset follows the default again; closed windows' entries
// are pruned only by the workspace/profile snapshot that listed them, so a
// switch away and back keeps them; every older global blob (including one
// written before widths existed) upgrades into the shell scope without loss,
// and a pre-default blob's incidental `open: false` never masks the default.
// A thread branch shows one list: opening its subagents folds its children,
// and a stored blob without subagent flags reads as every subagent list closed.
// The List | Kanban view and the board's "Hide child sessions" filter are
// window-only (UT-067): they survive a reload but never seed another window.
// The board's session drawer width (#1052) is per window and survives a reload; the last
// width dragged seeds windows that never chose one, it clamps into bounds, and a blob
// written before drawers existed reads the default.
// Sort and Group by (#952) are one per-window choice shared by that window's
// List and Kanban: they survive the view toggle and a reload, two windows stay
// independent, and a window that never chose follows the daemon's
// `shell.sessions` value, which the web never writes.
// Boundary IN: useSessionSidebarState + its persisted store + window.localStorage.
// Boundary OUT: SessionSidebar layout, topbar toggle chrome, the session list query.

import { createElement, type ReactNode } from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const STORAGE_KEY = "compozy:session:sidebar:v1";
const WINDOW_ONE = "win_one";
const WINDOW_TWO = "win_two";
const PARTITION_A = "ws_a\u0000default";
const PARTITION_B = "ws_b\u0000default";

/** A reload: fresh store and fresh scope context from the same module graph. */
async function loadFreshSidebarState() {
  vi.resetModules();
  const [sidebar, view, order, { WindowScopeContext }, query] = await Promise.all([
    import("../use-session-sidebar-state"),
    import("../use-session-window-view-state"),
    import("../use-session-window-order"),
    import("@/hooks/use-window-scope"),
    import("@tanstack/react-query"),
  ]);
  return {
    ...sidebar,
    ...view,
    ...order,
    WindowScopeContext,
    QueryClient: query.QueryClient,
    QueryClientProvider: query.QueryClientProvider,
  };
}

type SidebarModule = Awaited<ReturnType<typeof loadFreshSidebarState>>;

/** Renders the hook as a component inside one window (or the shell when `null`). */
function renderInScopeMounted(module: SidebarModule, scopeId: string | null) {
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(module.WindowScopeContext, { value: scopeId }, children);
  return renderHook(() => module.useSessionSidebarState(), { wrapper });
}

function renderInScope(module: SidebarModule, scopeId: string | null) {
  return renderInScopeMounted(module, scopeId).result;
}

function persistedContext() {
  return JSON.parse(window.localStorage.getItem(STORAGE_KEY) ?? "{}").context;
}

beforeEach(() => {
  window.localStorage.clear();
});

afterEach(() => {
  window.localStorage.clear();
});

describe("useSessionSidebarState", () => {
  it("Should open the rail when no preference is stored", async () => {
    const sidebar = await loadFreshSidebarState();
    const result = renderInScope(sidebar, WINDOW_ONE);
    expect(result.current.open).toBe(true);
    expect(result.current.collapsedThreadIds).toEqual([]);
    expect(result.current.collapsedFolderIds).toEqual([]);
  });

  it("Should show one branch list at a time and remember it across a reload", async () => {
    const first = await loadFreshSidebarState();
    const result = renderInScope(first, WINDOW_ONE);
    expect(result.current.subagentThreadIds).toEqual([]);

    act(() => {
      result.current.setThreadBranch("sess_root", "subagents");
    });
    expect(result.current).toMatchObject({
      collapsedThreadIds: ["sess_root"],
      subagentThreadIds: ["sess_root"],
    });

    act(() => {
      result.current.setThreadBranch("sess_root", "children");
    });
    expect(result.current).toMatchObject({ collapsedThreadIds: [], subagentThreadIds: [] });
    // A closed subagent list is the default, so nothing of it is stored.
    expect(persistedContext().byScope[WINDOW_ONE].subagentThreads).toEqual({});

    act(() => {
      result.current.setThreadBranch("sess_other", "subagents");
      result.current.setThreadBranch("sess_root", null);
    });
    const reloaded = await loadFreshSidebarState();
    expect(renderInScope(reloaded, WINDOW_ONE).current).toMatchObject({
      collapsedThreadIds: ["sess_root", "sess_other"],
      subagentThreadIds: ["sess_other"],
    });
  });

  it("Should keep a closed rail closed across a reload once the operator closed it", async () => {
    const first = await loadFreshSidebarState();
    const result = renderInScope(first, WINDOW_ONE);
    act(() => {
      result.current.toggle();
    });
    expect(result.current.open).toBe(false);
    expect(persistedContext()).toMatchObject({ byScope: { [WINDOW_ONE]: { open: false } } });

    const reloaded = await loadFreshSidebarState();
    expect(renderInScope(reloaded, WINDOW_ONE).current.open).toBe(false);
  });

  it("Should keep each window's rail, folders, threads, and groups independent", async () => {
    const sidebar = await loadFreshSidebarState();
    const one = renderInScope(sidebar, WINDOW_ONE);
    const two = renderInScope(sidebar, WINDOW_TWO);

    act(() => {
      one.current.toggle();
      one.current.toggleFolder("sfold-a");
      one.current.setThreadBranch("sess_root", null);
      one.current.toggleGroup("project:ws_a");
    });

    expect(one.current).toMatchObject({
      open: false,
      collapsedFolderIds: ["sfold-a"],
      collapsedThreadIds: ["sess_root"],
      collapsedGroupIds: ["project:ws_a"],
    });
    expect(two.current).toMatchObject({
      open: true,
      collapsedFolderIds: [],
      collapsedThreadIds: [],
      collapsedGroupIds: [],
    });

    act(() => {
      two.current.toggleFolder("sfold-b");
    });
    expect(one.current.collapsedFolderIds).toEqual(["sfold-a"]);
    expect(two.current.collapsedFolderIds).toEqual(["sfold-b"]);

    const reloaded = await loadFreshSidebarState();
    expect(renderInScope(reloaded, WINDOW_ONE).current).toMatchObject({
      open: false,
      collapsedFolderIds: ["sfold-a"],
      collapsedGroupIds: ["project:ws_a"],
    });
    expect(renderInScope(reloaded, WINDOW_TWO).current).toMatchObject({
      open: true,
      collapsedFolderIds: ["sfold-b"],
    });

    // Reopening a group drops its entry instead of storing `false`: only folds persist.
    const afterReload = renderInScope(reloaded, WINDOW_ONE);
    act(() => {
      afterReload.current.toggleGroup("project:ws_a");
    });
    expect(afterReload.current.collapsedGroupIds).toEqual([]);
    expect(persistedContext().byScope[WINDOW_ONE].collapsedGroups).toEqual({});
  });

  it("Should open a new window with the last-used rail state, then let it diverge", async () => {
    const sidebar = await loadFreshSidebarState();
    const one = renderInScope(sidebar, WINDOW_ONE);
    act(() => {
      one.current.toggle();
      one.current.toggleFolder("sfold-a");
    });

    const two = renderInScope(sidebar, WINDOW_TWO);
    expect(two.current).toMatchObject({ open: false, collapsedFolderIds: ["sfold-a"] });

    act(() => {
      one.current.toggle();
    });
    expect(one.current.open).toBe(true);
    expect(two.current.open).toBe(false);
  });

  it("UT-067: Should restore a window's Kanban view and child filter after a reload, never seeding a new window", async () => {
    const before = await loadFreshSidebarState();
    const renderView = (module: SidebarModule, scopeId: string) =>
      renderHook(() => module.useSessionWindowViewState(), {
        wrapper: ({ children }: { children: ReactNode }) =>
          createElement(module.WindowScopeContext, { value: scopeId }, children),
      }).result;
    const one = renderView(before, WINDOW_ONE);
    expect(one.current).toMatchObject({ view: "list", hideChildren: false });
    act(() => {
      one.current.setView("kanban");
      one.current.setHideChildren(true);
    });
    expect(renderView(before, WINDOW_TWO).current).toMatchObject({
      view: "list",
      hideChildren: false,
    });

    const after = await loadFreshSidebarState();
    await waitFor(() =>
      expect(renderView(after, WINDOW_ONE).current).toMatchObject({
        view: "kanban",
        hideChildren: true,
      })
    );
    expect(renderView(after, WINDOW_TWO).current.view).toBe("list");
  });

  it("Should restore a window's drawer width, seed new windows with the last one, and clamp it", async () => {
    const before = await loadFreshSidebarState();
    const renderView = (module: SidebarModule, scopeId: string) =>
      renderHook(() => module.useSessionWindowViewState(), {
        wrapper: ({ children }: { children: ReactNode }) =>
          createElement(module.WindowScopeContext, { value: scopeId }, children),
      }).result;
    const one = renderView(before, WINDOW_ONE);
    expect(one.current.drawerWidth).toBe(640);
    act(() => one.current.setDrawerWidth(812.4));
    expect(one.current.drawerWidth).toBe(812);
    expect(renderView(before, WINDOW_TWO).current.drawerWidth).toBe(812);

    act(() => one.current.setDrawerWidth(5000));
    expect(one.current.drawerWidth).toBe(1200);

    const after = await loadFreshSidebarState();
    await waitFor(() => expect(renderView(after, WINDOW_ONE).current.drawerWidth).toBe(1200));
  });

  it("Should read the default drawer width from a window entry stored before drawers existed", async () => {
    window.localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        context: {
          byScope: { [WINDOW_ONE]: { open: true, view: "kanban" } },
          partitionByScope: {},
        },
        version: 3,
      })
    );
    const upgraded = await loadFreshSidebarState();
    const result = renderHook(() => upgraded.useSessionWindowViewState(), {
      wrapper: ({ children }: { children: ReactNode }) =>
        createElement(upgraded.WindowScopeContext, { value: WINDOW_ONE }, children),
    }).result;
    await waitFor(() => expect(result.current.view).toBe("kanban"));
    expect(result.current.drawerWidth).toBe(640);
  });

  describe("per-window Sort and Group by", () => {
    let daemonSessions: { sort?: string; group_by?: string } = {};
    let fetcher: ReturnType<typeof vi.spyOn<typeof globalThis, "fetch">>;

    beforeEach(() => {
      daemonSessions = {};
      fetcher = vi.spyOn(globalThis, "fetch").mockImplementation(async input => {
        const request = input as Request;
        if (new URL(request.url).pathname !== "/api/settings/shell") {
          return new Response(null, { status: 404 });
        }
        return new Response(JSON.stringify({ config: { sessions: { ...daemonSessions } } }), {
          headers: { "content-type": "application/json" },
        });
      });
    });

    afterEach(() => {
      fetcher.mockRestore();
    });

    /** One window's view toggle and order, as its List and its Kanban read them. */
    function renderWindow(module: SidebarModule, scopeId: string) {
      const client = new module.QueryClient({ defaultOptions: { queries: { retry: false } } });
      return renderHook(
        () => ({ view: module.useSessionWindowViewState(), order: module.useSessionWindowOrder() }),
        {
          wrapper: ({ children }: { children: ReactNode }) =>
            createElement(
              module.QueryClientProvider,
              { client },
              createElement(module.WindowScopeContext, { value: scopeId }, children)
            ),
        }
      ).result;
    }

    it("Should keep a window's choice across the List | Kanban toggle and a reload", async () => {
      const before = await loadFreshSidebarState();
      const one = renderWindow(before, WINDOW_ONE);
      act(() => {
        one.current.order.setSort("name");
        one.current.order.setGroupBy("project");
      });
      act(() => one.current.view.setView("kanban"));
      expect(one.current.order).toMatchObject({ sort: "name", groupBy: "project" });
      act(() => one.current.view.setView("list"));
      expect(one.current.order).toMatchObject({ sort: "name", groupBy: "project" });

      const after = await loadFreshSidebarState();
      expect(renderWindow(after, WINDOW_ONE).current.order).toMatchObject({
        sort: "name",
        groupBy: "project",
      });
    });

    it("Should seed a window that never chose from the daemon and keep windows independent", async () => {
      daemonSessions = { sort: "attention", group_by: "profile" };
      const sidebar = await loadFreshSidebarState();
      const one = renderWindow(sidebar, WINDOW_ONE);
      const two = renderWindow(sidebar, WINDOW_TWO);
      await waitFor(() =>
        expect(one.current.order).toMatchObject({ sort: "attention", groupBy: "profile" })
      );

      act(() => one.current.order.setSort("name"));
      expect(one.current.order.sort).toBe("name");
      await waitFor(() => expect(two.current.order.sort).toBe("attention"));
      // A window opened after another chose still starts from the daemon, not the last choice.
      const three = renderWindow(sidebar, "win_three");
      await waitFor(() =>
        expect(three.current.order).toMatchObject({ sort: "attention", groupBy: "profile" })
      );
      expect(
        fetcher.mock.calls.filter(([input]) => (input as Request).method === "PATCH")
      ).toHaveLength(0);
    });
  });

  it("Should prune closed windows and keep live windows and the shell", async () => {
    const sidebar = await loadFreshSidebarState();
    const one = renderInScopeMounted(sidebar, WINDOW_ONE);
    const two = renderInScope(sidebar, WINDOW_TWO);
    act(() => {
      one.result.current.toggleFolder("sfold-a");
      two.current.toggleFolder("sfold-b");
    });

    act(() => {
      sidebar.pruneSessionSidebarScopes(PARTITION_A, [WINDOW_ONE, WINDOW_TWO]);
    });
    // Closing a window unmounts its rail before the shell sees the new window set.
    one.unmount();
    act(() => {
      sidebar.pruneSessionSidebarScopes(PARTITION_A, [WINDOW_TWO]);
    });

    const byScope = sidebar.sessionSidebarStore.getSnapshot().context.byScope;
    expect(Object.keys(byScope).sort()).toEqual(["shell", WINDOW_TWO].sort());
    expect(byScope[WINDOW_TWO]?.collapsedFolders).toEqual({ "sfold-b": true });
  });

  it("Should drop a window closed after a reload that preceded any other gesture", async () => {
    const sidebar = await loadFreshSidebarState();
    const one = renderInScopeMounted(sidebar, WINDOW_ONE);
    renderInScope(sidebar, WINDOW_TWO);
    act(() => {
      one.result.current.toggleFolder("sfold-a");
    });
    // The snapshot claims both windows; no gesture follows before the reload.
    act(() => {
      sidebar.pruneSessionSidebarScopes(PARTITION_A, [WINDOW_ONE, WINDOW_TWO]);
    });

    const reloaded = await loadFreshSidebarState();
    act(() => {
      reloaded.pruneSessionSidebarScopes(PARTITION_A, [WINDOW_TWO]);
    });
    expect(Object.keys(reloaded.sessionSidebarStore.getSnapshot().context.byScope)).not.toContain(
      WINDOW_ONE
    );

    // The cleanup itself survives the next reload too.
    const again = await loadFreshSidebarState();
    expect(Object.keys(again.sessionSidebarStore.getSnapshot().context.byScope)).not.toContain(
      WINDOW_ONE
    );
    expect(persistedContext().partitionByScope).toEqual({ [WINDOW_TWO]: PARTITION_A });
  });

  it("Should keep another workspace's windows when switching away and back", async () => {
    const sidebar = await loadFreshSidebarState();
    const one = renderInScopeMounted(sidebar, WINDOW_ONE);
    act(() => {
      one.result.current.toggle();
      one.result.current.toggleFolder("sfold-a");
    });
    act(() => {
      sidebar.pruneSessionSidebarScopes(PARTITION_A, [WINDOW_ONE]);
    });

    // Switching to workspace B unmounts A's windows and lists only B's.
    one.unmount();
    const two = renderInScope(sidebar, WINDOW_TWO);
    act(() => {
      sidebar.pruneSessionSidebarScopes(PARTITION_B, [WINDOW_TWO]);
      two.current.toggle();
    });

    // Back in A after a reload: window one keeps its own rail, not the last-used one.
    const reloaded = await loadFreshSidebarState();
    act(() => {
      reloaded.pruneSessionSidebarScopes(PARTITION_A, [WINDOW_ONE]);
    });
    expect(renderInScope(reloaded, WINDOW_ONE).current).toMatchObject({
      open: false,
      collapsedFolderIds: ["sfold-a"],
    });
  });

  it("Should move the old global rail into the shell scope without loss", async () => {
    window.localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        context: {
          open: false,
          collapsedThreads: { sess_root: true, sess_open: false },
          collapsedFolders: { "sfold-a": true },
        },
        version: 2,
      })
    );

    const upgraded = await loadFreshSidebarState();
    for (const scopeId of [null, WINDOW_ONE]) {
      expect(renderInScope(upgraded, scopeId).current).toMatchObject({
        open: false,
        collapsedThreadIds: ["sess_root"],
        collapsedFolderIds: ["sfold-a"],
      });
    }
  });

  it("Should open the rail and keep collapsed threads when upgrading a pre-default blob", async () => {
    // Written before the rail opened by default: collapsing a thread persisted
    // the whole context, including the old closed default.
    window.localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        context: { open: false, collapsedThreads: { sess_root: true } },
        version: 0,
      })
    );

    const upgraded = await loadFreshSidebarState();
    const result = renderInScope(upgraded, WINDOW_ONE);
    expect(result.current.open).toBe(true);
    expect(result.current.collapsedThreadIds).toEqual(["sess_root"]);

    act(() => {
      result.current.toggle();
    });
    const reloaded = await loadFreshSidebarState();
    const afterReload = renderInScope(reloaded, WINDOW_ONE);
    expect(afterReload.current.open).toBe(false);
    expect(afterReload.current.collapsedThreadIds).toEqual(["sess_root"]);
  });

  it("Should restore a dragged width per window across a reload and follow the default after a reset", async () => {
    const first = await loadFreshSidebarState();
    const result = renderInScope(first, WINDOW_ONE);
    const other = renderInScope(first, WINDOW_TWO);
    expect(result.current.width).toBe(264);
    act(() => {
      result.current.setWidth(361.6);
    });
    expect(result.current.width).toBe(362);
    expect(other.current.width).toBe(264);

    const reloaded = await loadFreshSidebarState();
    const afterReload = renderInScope(reloaded, WINDOW_ONE);
    expect(afterReload.current.width).toBe(362);
    act(() => {
      afterReload.current.resetWidth();
    });
    expect(afterReload.current.width).toBe(264);
    expect(persistedContext()).toMatchObject({ byScope: { [WINDOW_ONE]: { width: null } } });
  });

  it("Should follow a width reset made in another tab", async () => {
    const loaded = await loadFreshSidebarState();
    const result = renderInScope(loaded, WINDOW_ONE);
    act(() => {
      result.current.setWidth(400);
    });
    const stored = JSON.parse(window.localStorage.getItem(STORAGE_KEY) ?? "{}");
    stored.context.byScope[WINDOW_ONE].width = null;
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(stored));
    await act(async () => {
      window.dispatchEvent(new StorageEvent("storage", { key: STORAGE_KEY }));
      await Promise.resolve();
    });
    await waitFor(() => expect(result.current.width).toBe(264));
  });

  it.each([
    { name: "below the floor", stored: 40, expected: 200 },
    { name: "past the ceiling", stored: 4000, expected: 480 },
    { name: "not a number", stored: "wide", expected: 264 },
    { name: "reset to the default", stored: null, expected: 264 },
  ])("Should keep a stored width $name within bounds", async ({ stored, expected }) => {
    window.localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        context: { byScope: { [WINDOW_ONE]: { open: true, width: stored } }, partitionByScope: {} },
        version: 3,
      })
    );
    const loaded = await loadFreshSidebarState();
    const result = renderInScope(loaded, WINDOW_ONE);
    expect(result.current.width).toBe(expected);
  });

  it("Should clamp a width set past the bounds", async () => {
    const loaded = await loadFreshSidebarState();
    const result = renderInScope(loaded, WINDOW_ONE);
    act(() => {
      result.current.setWidth(10_000);
    });
    expect(result.current.width).toBe(480);
    act(() => {
      result.current.setWidth(Number.NaN);
    });
    expect(result.current.width).toBe(480);
  });

  it("Should keep the open state and folds of a global blob written before widths existed", async () => {
    window.localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        context: { open: false, collapsedThreads: {}, collapsedFolders: { "sfold-a": true } },
        version: 2,
      })
    );
    const upgraded = await loadFreshSidebarState();
    const result = renderInScope(upgraded, WINDOW_ONE);
    expect(result.current.open).toBe(false);
    expect(result.current.collapsedFolderIds).toEqual(["sfold-a"]);
    expect(result.current.width).toBe(264);
  });

  it("Should start folders expanded and remember a collapsed folder across a reload", async () => {
    const first = await loadFreshSidebarState();
    const result = renderInScope(first, WINDOW_ONE);
    expect(result.current.collapsedFolderIds).toEqual([]);
    act(() => {
      result.current.toggleFolder("sfold-a");
    });
    expect(result.current.collapsedFolderIds).toEqual(["sfold-a"]);

    const reloaded = await loadFreshSidebarState();
    const afterReload = renderInScope(reloaded, WINDOW_ONE);
    expect(afterReload.current.collapsedFolderIds).toEqual(["sfold-a"]);
    act(() => {
      afterReload.current.toggleFolder("sfold-a");
    });
    expect(afterReload.current.collapsedFolderIds).toEqual([]);
  });
});
