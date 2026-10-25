// Suite: desktop chrome boot
// Invariant: useDesktopChrome owns OsShellHandle and mounts without OsShellContext;
// the settings read that names this client waits for its registration and reads no
// workspace overlay on a reserved partition (`global`, `user`); per-window
// sessions-rail state and worktree picks are pruned only against a snapshot fresh
// since mount, never the cached window list a returning destination starts from.
// Boundary IN: chrome hook projection reads and provider ownership.
// Boundary OUT: window-manager stream, client registration, and rendered DesktopShell.
import { QueryClient, QueryClientProvider, type UseQueryOptions } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const queryOptionsSeen = vi.hoisted(() => [] as Array<{ queryKey: unknown; enabled?: unknown }>);
const registration = vi.hoisted(() => ({ status: "idle" as string }));
const snapshotQuery = vi.hoisted(() => ({ live: false }));
const pruneSessionSidebarScopes = vi.hoisted(() => vi.fn());
const pruneWorktreeScopes = vi.hoisted(() => vi.fn());

vi.mock("@tanstack/react-router", () => ({
  useRouter: () => ({
    options: { stringifySearch: () => "" },
    history: { replace: vi.fn(), push: vi.fn() },
  }),
}));

vi.mock("@tanstack/react-query", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-query")>();
  return {
    ...actual,
    useQuery: (options: UseQueryOptions) => {
      queryOptionsSeen.push(options);
      if (snapshotQuery.live && options.queryKey.includes("snapshot")) {
        return actual.useQuery(options);
      }
      return { data: undefined };
    },
  };
});

vi.mock("@/systems/workspace", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/workspace")>();
  return {
    ...actual,
    useBreadthMode: () => "focused",
    pruneWorktreeScopes,
  };
});

vi.mock("@/systems/session", () => ({
  useSession: () => ({ data: undefined }),
  pruneSessionSidebarScopes,
}));

vi.mock("../use-window-manager-client", () => ({
  useWindowManagerClient: () => ({
    clientId: "client:test",
    registrationEpoch: 0,
    client: null,
    status: registration.status,
    error: null,
    reregister: vi.fn(),
  }),
}));

vi.mock("../use-window-manager-stream", () => ({
  useWindowManagerStream: vi.fn(),
}));

vi.mock("../use-global-shortcut-reconciliation", () => ({
  useGlobalShortcutReconciliation: () => ({ registrations: [] }),
}));

import type { WindowManagerSnapshot } from "../../lib/window-manager-types";
import { useDesktopChrome } from "../use-desktop-chrome";
import { windowManagerKeys } from "../../lib/window-manager-query";
import { useProfileReadScope } from "@/systems/profiles";
import { useWindowScopePruning } from "../use-window-scope-pruning";

function wrapper(
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
) {
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

function settingsReads() {
  return queryOptionsSeen.filter(
    options => Array.isArray(options.queryKey) && options.queryKey.includes("client:test")
  );
}

describe("useDesktopChrome", () => {
  beforeEach(() => {
    queryOptionsSeen.length = 0;
    registration.status = "idle";
    snapshotQuery.live = false;
  });

  it("Should mount without an OsShellContext provider", () => {
    const { result } = renderHook(() => useDesktopChrome(null), { wrapper: wrapper() });

    expect(result.current.shell.manager).toBeDefined();
    expect(result.current.shell.projection).toBeDefined();
    expect(result.current.client).toBeNull();
  });

  it("Should hold the client-scoped settings read until the client is registered", () => {
    const { rerender } = renderHook(() => useDesktopChrome("workspace:one"), {
      wrapper: wrapper(),
    });
    expect(settingsReads().length).toBeGreaterThan(0);
    expect(settingsReads().every(options => options.enabled === false)).toBe(true);

    registration.status = "registered";
    queryOptionsSeen.length = 0;
    rerender();
    expect(settingsReads().at(-1)?.enabled).toBe(true);
  });

  it("Should read no workspace overlay on the User partition", () => {
    renderHook(() => useDesktopChrome("user"), { wrapper: wrapper() });

    expect(settingsReads().length).toBeGreaterThan(0);
    expect(
      settingsReads().every(options => !(options.queryKey as unknown[]).includes("user"))
    ).toBe(true);
  });

  it("Should not rerender chrome for geometry-only snapshots and still prune removed windows", async () => {
    snapshotQuery.live = true;
    const queryClient = new QueryClient();
    const { result: scope } = renderHook(useProfileReadScope, { wrapper: wrapper(queryClient) });
    const key = windowManagerKeys.snapshot("workspace:one", scope.current.destination);
    const window = {
      id: "win_one",
      app: "tasks",
      instanceKey: null,
      route: { pathname: "/tasks", search: {} },
      navStack: [],
      pinned: false,
      placement: "floating",
      desktopId: "desktop-default",
      floatingRect: { x: 0.1, y: 0.1, w: 0.4, h: 0.5 },
      minimized: false,
      zoomed: false,
      returnAnchor: null,
    } as const;
    const snapshot: WindowManagerSnapshot = {
      version: 5,
      workspaceId: "workspace:one",
      revision: 1,
      desktops: [],
      windows: { win_one: window, win_two: { ...window, id: "win_two" } },
      closedEntryCount: 0,
      overrides: {},
      updatedAt: "2026-10-25T00:00:00Z",
    };
    queryClient.setQueryData(key, snapshot);
    let renders = 0;
    renderHook(
      () => {
        renders += 1;
        return useDesktopChrome("workspace:one");
      },
      { wrapper: wrapper(queryClient) }
    );
    // The pre-mount write only seeds the cache. TanStack marks a read fresh
    // since mount (`isFetchedAfterMount`, which pruning requires) only when
    // `dataUpdateCount` advances after the observer exists.
    act(() => {
      queryClient.setQueryData(key, snapshot);
    });
    await waitFor(() =>
      expect(pruneSessionSidebarScopes).toHaveBeenLastCalledWith(
        `workspace:one\u0000${scope.current.destination}`,
        ["win_one", "win_two"]
      )
    );
    const initialRenders = renders;

    await act(async () => {
      queryClient.setQueryData(key, {
        ...snapshot,
        revision: 2,
        windows: {
          ...snapshot.windows,
          win_one: { ...window, floatingRect: { ...window.floatingRect, x: 0.2 } },
        },
      });
      await new Promise(resolve => setTimeout(resolve, 0));
    });
    expect(renders).toBe(initialRenders);

    act(() => {
      queryClient.setQueryData(key, {
        ...snapshot,
        revision: 3,
        windows: { win_two: snapshot.windows.win_two },
      });
    });
    await waitFor(() => {
      expect(pruneSessionSidebarScopes).toHaveBeenLastCalledWith(
        `workspace:one\u0000${scope.current.destination}`,
        ["win_two"]
      );
      expect(pruneWorktreeScopes).toHaveBeenLastCalledWith(
        `workspace:one\u0000${scope.current.destination}`,
        ["win_two"]
      );
    });
  });
});

describe("useWindowScopePruning", () => {
  beforeEach(() => {
    pruneSessionSidebarScopes.mockClear();
    pruneWorktreeScopes.mockClear();
  });

  it("Should wait for a fresh snapshot before pruning against the cached window list", () => {
    const snapshot = {
      workspaceId: "ws_a",
      windowIds: ["win_one", "win_two"],
    };
    const { rerender } = renderHook(
      ({ fresh }) =>
        useWindowScopePruning("default", { data: snapshot, isFetchedAfterMount: fresh }),
      { initialProps: { fresh: false } }
    );
    expect(pruneSessionSidebarScopes).not.toHaveBeenCalled();
    expect(pruneWorktreeScopes).not.toHaveBeenCalled();

    rerender({ fresh: true });
    expect(pruneSessionSidebarScopes).toHaveBeenCalledExactlyOnceWith("ws_a\u0000default", [
      "win_one",
      "win_two",
    ]);
    expect(pruneWorktreeScopes).toHaveBeenCalledExactlyOnceWith("ws_a\u0000default", [
      "win_one",
      "win_two",
    ]);
  });
});
