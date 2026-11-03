// Suite: desktop chrome boot
// Invariant: useDesktopChrome owns OsShellHandle and mounts without OsShellContext;
// the settings read that names this client waits for its registration and reads no
// workspace overlay on a reserved partition (`global`, `user`); per-window
// sessions-rail state and worktree picks are pruned only against a snapshot fresh
// since mount, never the cached window list a returning destination starts from;
// an explicit switch into one profile closes, once its desk is live, exactly the
// session windows another profile owns (#1184).
// Boundary IN: chrome hook projection reads and provider ownership.
// Boundary OUT: window-manager stream, client registration, and rendered DesktopShell.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createAtom } from "@xstate/store";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const queryOptionsSeen = vi.hoisted(() => [] as Array<{ queryKey: unknown; enabled?: unknown }>);
const registration = vi.hoisted(() => ({ status: "idle" as string }));
const pruneSessionSidebarScopes = vi.hoisted(() => vi.fn());
const pruneWorktreeScopes = vi.hoisted(() => vi.fn());
const profileScope = vi.hoisted(() => ({
  aggregate: false,
  ownerId: "p_yankee",
}));
const sessionOwners = vi.hoisted(() => new Map<string, string>());

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
    useQuery: (options: { queryKey: unknown; enabled?: unknown }) => {
      queryOptionsSeen.push(options);
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
  sessionAcrossProfilesOptions: (sessionId: string) => ({
    queryKey: ["session-owner", sessionId],
    queryFn: () => {
      const owner = sessionOwners.get(sessionId);
      if (owner === undefined) throw new Error("session not found");
      return { profile_id: owner, profile_name: owner, profile_color: "#000000" };
    },
  }),
}));

vi.mock("@/systems/profiles", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/profiles")>();
  return {
    ...actual,
    useProfileReadScope: () => ({
      view: profileScope.aggregate ? { kind: "aggregate" } : { kind: "profile", profile: "yankee" },
      aggregate: profileScope.aggregate,
      destination: "yankee",
      destinationOwner: profileScope.aggregate ? null : { id: profileScope.ownerId },
    }),
  };
});

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

// A pass-through spy: the chrome cases see the call, the sweep cases still run the real hook.
vi.mock("../use-profile-switch-window-sweep", async importOriginal => {
  const actual = await importOriginal<typeof import("../use-profile-switch-window-sweep")>();
  return { useProfileSwitchWindowSweep: vi.fn(actual.useProfileSwitchWindowSweep) };
});

vi.mock("../use-global-shortcut-reconciliation", () => ({
  useGlobalShortcutReconciliation: () => ({ registrations: [] }),
}));

import { resetProfileViews, setProfileView } from "@/systems/profiles/stores/profile-view-store";

import type { OsShellHandle } from "../../contexts/os-shell-context";
import type { OsDesktopRuntimeStore, OsWindow } from "../../lib/os-types";
import type { WindowManagerSnapshot } from "../../lib/window-manager-types";
import { useDesktopChrome } from "../use-desktop-chrome";
import { useProfileSwitchWindowSweep } from "../use-profile-switch-window-sweep";
import { useWindowScopePruning } from "../use-window-scope-pruning";

function wrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
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

  it("Should sweep the desk it presents with its own shell", () => {
    const { result } = renderHook(() => useDesktopChrome("ws_a"), { wrapper: wrapper() });

    expect(useProfileSwitchWindowSweep).toHaveBeenLastCalledWith(result.current.shell, "ws_a");
  });

  it("Should read no workspace overlay on the User partition", () => {
    renderHook(() => useDesktopChrome("user"), { wrapper: wrapper() });

    expect(settingsReads().length).toBeGreaterThan(0);
    expect(
      settingsReads().every(options => !(options.queryKey as unknown[]).includes("user"))
    ).toBe(true);
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
      windows: { win_one: {}, win_two: {} },
    } as unknown as WindowManagerSnapshot;
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

describe("useProfileSwitchWindowSweep", () => {
  beforeEach(() => {
    profileScope.aggregate = false;
    profileScope.ownerId = "p_yankee";
    sessionOwners.clear();
    resetProfileViews();
  });

  function deskWindow(id: string, pathname: string): OsWindow {
    return { id, route: { pathname, search: {} } } as unknown as OsWindow;
  }

  function fakeShell(live: boolean, duringFirstClose?: () => void, refuseFirst = false) {
    let windows: Record<string, OsWindow> = {
      w_own: deskWindow("w_own", "/agents/builder/sessions/s_own"),
      w_xray: deskWindow("w_xray", "/agents/builder/sessions/s_xray"),
      w_zulu: deskWindow("w_zulu", "/agents/builder/sessions/s_zulu"),
      w_gone: deskWindow("w_gone", "/agents/builder/sessions/s_gone"),
      // A malformed escape the route parser throws on; it must not abort the sweep.
      w_bad: deskWindow("w_bad", "/agents/builder/sessions/%E0%A4%A"),
      w_tasks: deskWindow("w_tasks", "/tasks"),
    };
    const state = (isLive: boolean) =>
      ({
        hydration: isLive ? "live" : "pending",
        client: isLive ? {} : null,
        connectionStatus: isLive ? "connected" : "connecting",
        windows,
      }) as unknown as OsDesktopRuntimeStore;
    const projection = createAtom(state(live));
    // Like the runtime's close guard: one close at a time, the rest refused while it is pending.
    let pending = false;
    const closed: string[] = [];
    let refused = !refuseFirst;
    const userClose = vi.fn(async (windowId: string) => {
      if (pending) return false;
      if (!refused) {
        // Another surface's close is pending for a while (a terminal confirmation, say): the
        // runtime refuses this one and keeps refusing until that close settles.
        refused = true;
        pending = true;
        setTimeout(() => (pending = false), 300);
        return false;
      }
      pending = true;
      if (closed.length === 0) duringFirstClose?.();
      await Promise.resolve();
      pending = false;
      closed.push(windowId);
      return true;
    });
    const shell = {
      projection,
      manager: { getState: () => projection.get() },
      coordinator: { userClose },
    } as unknown as OsShellHandle;
    return {
      shell,
      userClose,
      closed,
      goLive: () => projection.set(state(true)),
      showIn: (windowId: string, pathname: string) => {
        windows = { ...windows, [windowId]: deskWindow(windowId, pathname) };
        projection.set(state(true));
      },
    };
  }

  function switchTo(view: Parameters<typeof setProfileView>[0]) {
    act(() => setProfileView(view));
  }

  it("Should close every session window another profile owns once the switched-to desk is live", async () => {
    sessionOwners.set("s_own", "p_yankee");
    sessionOwners.set("s_xray", "p_xray");
    sessionOwners.set("s_zulu", "p_zulu");
    const { shell, closed, goLive } = fakeShell(false);
    renderHook(() => useProfileSwitchWindowSweep(shell, "ws_a"), { wrapper: wrapper() });

    switchTo({ kind: "profile", profile: "yankee" });
    expect(closed).toEqual([]);

    act(() => goLive());
    await waitFor(() => expect(closed).toEqual(["w_xray", "w_zulu"]));
  });

  it("Should sweep each desk of the profile the first time it is live after the switch", async () => {
    sessionOwners.set("s_xray", "p_xray");
    const { shell, userClose } = fakeShell(true);
    const { rerender } = renderHook(
      ({ partition }) => useProfileSwitchWindowSweep(shell, partition),
      { wrapper: wrapper(), initialProps: { partition: "ws_a" } }
    );

    switchTo({ kind: "profile", profile: "yankee" });
    await waitFor(() => expect(userClose).toHaveBeenCalledTimes(1));
    // Moving to All projects later presents another desk of the same profile: it is swept too.
    rerender({ partition: "global" });
    await waitFor(() => expect(userClose).toHaveBeenCalledTimes(2));
    // Coming back to a desk already swept for this switch does not sweep it again.
    rerender({ partition: "ws_a" });
    await new Promise(resolve => setTimeout(resolve, 0));
    expect(userClose).toHaveBeenCalledTimes(2);
  });

  it("Should retry a close the runtime refused while another close was pending", async () => {
    sessionOwners.set("s_xray", "p_xray");
    const { shell, closed } = fakeShell(true, undefined, true);
    renderHook(() => useProfileSwitchWindowSweep(shell, "ws_a"), { wrapper: wrapper() });

    switchTo({ kind: "profile", profile: "yankee" });
    await waitFor(() => expect(closed).toEqual(["w_xray"]), { timeout: 2000 });
  });

  it("Should back off a close that keeps failing instead of hammering the daemon", async () => {
    sessionOwners.set("s_xray", "p_xray");
    const { shell, userClose } = fakeShell(true);
    userClose.mockImplementation(async () => false);
    const { unmount } = renderHook(() => useProfileSwitchWindowSweep(shell, "ws_a"), {
      wrapper: wrapper(),
    });

    switchTo({ kind: "profile", profile: "yankee" });
    await waitFor(() => expect(userClose).toHaveBeenCalled());
    await new Promise(resolve => setTimeout(resolve, 1300));
    // Attempts at about 0, 250 and 750 ms; a fixed 250 ms retry would have made six.
    expect(userClose.mock.calls.length).toBeLessThanOrEqual(3);
    unmount();
  });

  it("Should stop retrying a refused close once the shell unmounts", async () => {
    sessionOwners.set("s_xray", "p_xray");
    const { shell, userClose } = fakeShell(true);
    userClose.mockImplementation(async () => false);
    const { unmount } = renderHook(() => useProfileSwitchWindowSweep(shell, "ws_a"), {
      wrapper: wrapper(),
    });

    switchTo({ kind: "profile", profile: "yankee" });
    await waitFor(() => expect(userClose).toHaveBeenCalled());
    unmount();
    const attempts = userClose.mock.calls.length;
    await new Promise(resolve => setTimeout(resolve, 600));
    expect(userClose).toHaveBeenCalledTimes(attempts);
  });

  it("Should skip a window the operator moved to another session before its close", async () => {
    sessionOwners.set("s_own", "p_yankee");
    sessionOwners.set("s_xray", "p_xray");
    sessionOwners.set("s_zulu", "p_zulu");
    let showIn: (windowId: string, pathname: string) => void = () => {};
    const fake = fakeShell(true, () => showIn("w_zulu", "/agents/builder/sessions/s_own"));
    showIn = fake.showIn;
    renderHook(() => useProfileSwitchWindowSweep(fake.shell, "ws_a"), { wrapper: wrapper() });

    switchTo({ kind: "profile", profile: "yankee" });
    await waitFor(() => expect(fake.closed).toEqual(["w_xray"]));
    await new Promise(resolve => setTimeout(resolve, 0));
    expect(fake.userClose).not.toHaveBeenCalledWith("w_zulu");
  });

  it("Should leave the desk alone without a switch or when the switch is to All profiles", async () => {
    sessionOwners.set("s_xray", "p_xray");
    const { shell, userClose } = fakeShell(true);
    const { rerender } = renderHook(() => useProfileSwitchWindowSweep(shell, "ws_a"), {
      wrapper: wrapper(),
    });
    rerender();

    profileScope.aggregate = true;
    switchTo({ kind: "aggregate" });
    // Let any owner read the hook might have started settle before asserting nothing closed.
    await new Promise(resolve => setTimeout(resolve, 0));
    expect(userClose).not.toHaveBeenCalled();
  });

  it("Should abandon a sweep when the view changes while the owners are still being read", async () => {
    let releaseOwners = () => {};
    const ownersRead = new Promise<void>(resolve => {
      releaseOwners = resolve;
    });
    sessionOwners.set("s_xray", "p_xray");
    const { shell, userClose } = fakeShell(true);
    const queryClient = new QueryClient();
    const fetchQuery = queryClient.fetchQuery.bind(queryClient);
    vi.spyOn(queryClient, "fetchQuery").mockImplementation(async options => {
      await ownersRead;
      return fetchQuery(options);
    });
    renderHook(() => useProfileSwitchWindowSweep(shell, "ws_a"), {
      wrapper: ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      ),
    });

    switchTo({ kind: "profile", profile: "yankee" });
    await waitFor(() => expect(queryClient.fetchQuery).toHaveBeenCalled());
    // Back to All profiles on the same desk: the windows belong on screen again.
    profileScope.aggregate = true;
    switchTo({ kind: "aggregate" });
    releaseOwners();
    await new Promise(resolve => setTimeout(resolve, 0));
    expect(userClose).not.toHaveBeenCalled();
  });
});
