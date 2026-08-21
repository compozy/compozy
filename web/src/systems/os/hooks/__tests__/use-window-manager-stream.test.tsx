// Suite: window-manager stream hook
// Invariant: one client-bound socket survives topology advances, reconnects with the latest fence,
// serializes burst refreshes to the highest announced revision, and accepts only newer bound-client
// presentation frames.
// Owning layer: the WebSocket → TanStack Query/client-presentation bridge.
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { PropsWithChildren } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { statusKeys, type StatusPayload } from "@/systems/status";
import { fetchWorkspaces } from "@/systems/workspace/adapters/workspace-api";
import { workspaceKeys } from "@/systems/workspace/lib/query-keys";
import { useActiveWorkspace } from "@/systems/workspace/hooks/use-active-workspace";
import {
  clearActiveWorkspaceSelection,
  setActiveWorkspaceId,
} from "@/systems/workspace/stores/active-workspace-store";
import type { WorkspacePayload } from "@/systems/workspace/types";

import { fetchWindowManagerSnapshot } from "../../adapters/window-manager-api";
import { windowManagerKeys } from "../../lib/window-manager-query";
import type {
  WindowManagerClientView,
  WindowManagerSnapshot,
} from "../../lib/window-manager-types";
import {
  useWindowManagerStream,
  type WindowManagerClientContextInput,
  type WindowManagerSocket,
  type WindowManagerSocketFactory,
} from "../use-window-manager-stream";

vi.mock("../../adapters/window-manager-api", async importOriginal => {
  const actual = await importOriginal<typeof import("../../adapters/window-manager-api")>();
  return {
    ...actual,
    fetchWindowManagerSnapshot: vi.fn(),
  };
});

vi.mock("@/systems/workspace/adapters/workspace-api", () => ({
  fetchWorkspaces: vi.fn(),
}));

class FakeSocket implements WindowManagerSocket {
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent<unknown>) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  close = vi.fn();
  send = vi.fn();

  open() {
    this.onopen?.({} as Event);
  }

  message(frame: unknown) {
    this.onmessage?.({ data: JSON.stringify(frame) } as MessageEvent<string>);
  }

  fail() {
    this.onerror?.({} as Event);
  }

  disconnect() {
    this.onclose?.({} as CloseEvent);
  }
}

function snapshot(revision: number): WindowManagerSnapshot {
  return {
    version: 3,
    workspaceId: "workspace:test",
    revision,
    desktops: [
      {
        id: "desktop:main",
        name: "Main",
        order: 0,
        purpose: "standard",
        focusOwner: null,
        groups: [],
        floating: [],
        floatingStacks: [],
      },
    ],
    windows: {},
    closedEntryCount: 0,
    overrides: {},
    updatedAt: "2026-07-22T00:00:00Z",
  };
}

function client(presentationRevision: number, clientId = "client:web"): WindowManagerClientView {
  return {
    workspaceId: "workspace:test",
    clientId,
    presentationRevision,
    activeDesktopId: "desktop:main",
    focusedWindowId: null,
    focusOrder: [],
    stackActive: {},
    connectedAt: "2026-07-22T00:00:00Z",
  };
}

function workspace(id: string): WorkspacePayload {
  return {
    id,
    root_dir: `/workspace/${id}`,
    add_dirs: [],
    name: id,
    created_at: "2026-07-22T00:00:00Z",
    updated_at: "2026-07-22T00:00:00Z",
  };
}

function rawClientFrame(presentationRevision: number, clientId = "client:web") {
  return {
    type: "client",
    workspace_id: "workspace:test",
    revision: presentationRevision,
    client: {
      workspace_id: "workspace:test",
      client_id: clientId,
      kind: "browser",
      presentation_revision: presentationRevision,
      context_revision: 1,
      active_desktop_id: "desktop:main",
      focus_order: [],
      stack_active: {},
      palette_context: {
        window_focused: false,
        window_floating: false,
        window_stacked: false,
        desktop_window_count: 0,
        scope_global: false,
        shell_desktop: false,
        workspace_trusted: true,
      },
      connected_at: "2026-07-22T00:00:00Z",
      global_shortcuts: [],
    },
  };
}

function rawEventFrame(revision: number) {
  return {
    type: "event",
    workspace_id: "workspace:test",
    revision,
    event: {
      workspace_id: "workspace:test",
      revision,
      command_id: "window.navigate",
      changes: {},
      actor: { kind: "web", id: "client:web" },
      origin: "web",
      occurred_at: "2026-07-22T00:00:00Z",
    },
  };
}

function rawSnapshotFrame(revision: number) {
  return {
    type: "snapshot",
    workspace_id: "workspace:test",
    revision,
    snapshot: {
      version: 3,
      workspace_id: "workspace:test",
      revision,
      desktops: [
        {
          id: "desktop:main",
          name: "Main",
          order: 0,
          purpose: "standard",
          groups: [],
          floating: [],
          floating_stacks: [],
        },
      ],
      windows: {},
      closed_entry_count: 0,
      overrides: {},
      updated_at: "2026-07-22T00:00:00Z",
    },
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>(done => {
    resolve = done;
  });
  return { promise, resolve };
}

function wrapper(queryClient: QueryClient) {
  return function WindowManagerQueryProvider({ children }: PropsWithChildren) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  };
}

function createSocketFactory() {
  const sockets: FakeSocket[] = [];
  const factory: WindowManagerSocketFactory = vi.fn(() => {
    const socket = new FakeSocket();
    sockets.push(socket);
    return socket;
  });
  return { factory, sockets };
}

afterEach(() => {
  vi.useRealTimers();
  vi.clearAllMocks();
});

describe("useWindowManagerStream", () => {
  it("Should close and reopen the stream when only the profile binding changes", () => {
    const queryClient = new QueryClient();
    const { factory, sockets } = createSocketFactory();
    const callbacks = {
      onStatusChange: vi.fn(),
      onSnapshot: vi.fn(),
      onClient: vi.fn(),
      onClientInvalidated: vi.fn(),
      onError: vi.fn(),
    };
    const { rerender } = renderHook(
      ({ profileId }: { profileId: string }) =>
        useWindowManagerStream({
          workspaceId: "workspace:test",
          profileId,
          clientId: "client:web",
          registrationEpoch: 0,
          currentClient: null,
          enabled: true,
          afterRevision: 0,
          socketFactory: factory,
          ...callbacks,
        }),
      { initialProps: { profileId: "marketing" }, wrapper: wrapper(queryClient) }
    );

    expect(factory).toHaveBeenCalledExactlyOnceWith(
      "/api/workspaces/workspace%3Atest/window-manager/stream?after_revision=0&client_id=client%3Aweb&profile=marketing"
    );

    rerender({ profileId: "research" });

    expect(sockets[0]?.close).toHaveBeenCalledOnce();
    expect(factory).toHaveBeenLastCalledWith(
      "/api/workspaces/workspace%3Atest/window-manager/stream?after_revision=0&client_id=client%3Aweb&profile=research"
    );
    expect(factory).toHaveBeenCalledTimes(2);
  });

  it("Should reconcile a removed workspace before opening the replacement stream", async () => {
    const staleWorkspace = workspace("workspace:stale");
    const currentWorkspace = workspace("workspace:current");
    const homeRow = { ...workspace("workspace:home"), root_dir: "/Users/operator" };
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    // Scope resolution needs `$HOME` before it claims a workspace binding.
    queryClient.setQueryData(statusKeys.current(), {
      daemon: { user_home_dir: "/Users/operator" },
    } as StatusPayload);
    queryClient.setQueryData(workspaceKeys.list(), [staleWorkspace, homeRow]);
    setActiveWorkspaceId(staleWorkspace.id);
    vi.mocked(fetchWorkspaces).mockResolvedValue([currentWorkspace, homeRow]);
    const { factory, sockets } = createSocketFactory();

    try {
      const { result } = renderHook(
        () => {
          // Mirrors the desktop shell: the stream binds the runtime workspace,
          // which is the home row while Global scope is on.
          const { runtimeWorkspaceId } = useActiveWorkspace();
          useWindowManagerStream({
            workspaceId: runtimeWorkspaceId,
            profileId: "marketing",
            clientId: "client:web",
            registrationEpoch: 0,
            currentClient: null,
            enabled: runtimeWorkspaceId !== null,
            afterRevision: 0,
            socketFactory: factory,
            onStatusChange: vi.fn(),
            onSnapshot: vi.fn(),
            onClient: vi.fn(),
            onClientInvalidated: vi.fn(),
            onError: vi.fn(),
          });
          return runtimeWorkspaceId;
        },
        { wrapper: wrapper(queryClient) }
      );

      expect(result.current).toBe(staleWorkspace.id);
      expect(factory).toHaveBeenCalledWith(
        "/api/workspaces/workspace%3Astale/window-manager/stream?after_revision=0&client_id=client%3Aweb&profile=marketing"
      );

      act(() => {
        sockets[0]?.message({
          type: "error",
          error: {
            error: "window_manager_workspace_not_found",
            code: "window_manager_workspace_not_found",
            workspace_id: staleWorkspace.id,
          },
        });
      });

      // A pruned selection never adopts another project — it falls back to
      // Global, whose runtime binding is the operator-home row.
      await waitFor(() => expect(result.current).toBe(homeRow.id));
      expect(fetchWorkspaces).toHaveBeenCalledOnce();
      expect(factory).toHaveBeenLastCalledWith(
        "/api/workspaces/workspace%3Ahome/window-manager/stream?after_revision=0&client_id=client%3Aweb&profile=marketing"
      );
    } finally {
      act(() => clearActiveWorkspaceSelection());
    }
  });

  it("Should keep one socket across topology advances and reconnect with the latest cached fence", async () => {
    vi.useFakeTimers();
    const queryClient = new QueryClient();
    queryClient.setQueryData(
      windowManagerKeys.snapshot("workspace:test", "marketing"),
      snapshot(1)
    );
    const { factory, sockets } = createSocketFactory();
    const callbacks = {
      onStatusChange: vi.fn(),
      onSnapshot: vi.fn(),
      onClient: vi.fn(),
      onClientInvalidated: vi.fn(),
      onError: vi.fn(),
    };

    const { rerender } = renderHook(
      ({ revision }: { revision: number }) =>
        useWindowManagerStream({
          workspaceId: "workspace:test",
          profileId: "marketing",
          clientId: "client:web",
          registrationEpoch: 0,
          currentClient: client(1),
          enabled: true,
          afterRevision: revision,
          socketFactory: factory,
          ...callbacks,
        }),
      { initialProps: { revision: 1 }, wrapper: wrapper(queryClient) }
    );

    expect(factory).toHaveBeenCalledWith(
      "/api/workspaces/workspace%3Atest/window-manager/stream?after_revision=1&client_id=client%3Aweb&profile=marketing"
    );
    queryClient.setQueryData(
      windowManagerKeys.snapshot("workspace:test", "marketing"),
      snapshot(4)
    );
    rerender({ revision: 5 });
    expect(factory).toHaveBeenCalledOnce();

    act(() => sockets[0]?.disconnect());
    await act(() => vi.advanceTimersByTimeAsync(500));

    expect(factory).toHaveBeenCalledTimes(2);
    expect(factory).toHaveBeenLastCalledWith(
      "/api/workspaces/workspace%3Atest/window-manager/stream?after_revision=5&client_id=client%3Aweb&profile=marketing"
    );

    act(() => sockets[1]?.message(rawSnapshotFrame(5)));
    act(() => sockets[1]?.disconnect());
    await act(() => vi.advanceTimersByTimeAsync(499));
    expect(factory).toHaveBeenCalledTimes(2);
    await act(() => vi.advanceTimersByTimeAsync(1));
    expect(factory).toHaveBeenCalledTimes(3);
  });

  it("Should serialize burst refreshes until Query reaches the highest announced revision", async () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(
      windowManagerKeys.snapshot("workspace:test", "marketing"),
      snapshot(1)
    );
    const { factory, sockets } = createSocketFactory();
    const revisionTwo = deferred<WindowManagerSnapshot>();
    const revisionThree = deferred<WindowManagerSnapshot>();
    vi.mocked(fetchWindowManagerSnapshot)
      .mockReturnValueOnce(revisionTwo.promise)
      .mockReturnValueOnce(revisionThree.promise);

    renderHook(
      () =>
        useWindowManagerStream({
          workspaceId: "workspace:test",
          profileId: "marketing",
          clientId: "client:web",
          registrationEpoch: 0,
          currentClient: client(1),
          enabled: true,
          afterRevision: 1,
          socketFactory: factory,
          onStatusChange: vi.fn(),
          onSnapshot: vi.fn(),
          onClient: vi.fn(),
          onClientInvalidated: vi.fn(),
          onError: vi.fn(),
        }),
      { wrapper: wrapper(queryClient) }
    );

    act(() => sockets[0]?.message(rawEventFrame(2)));
    expect(fetchWindowManagerSnapshot).toHaveBeenCalledOnce();
    act(() => sockets[0]?.message(rawEventFrame(3)));
    expect(fetchWindowManagerSnapshot).toHaveBeenCalledOnce();

    await act(async () => revisionTwo.resolve(snapshot(2)));
    expect(fetchWindowManagerSnapshot).toHaveBeenCalledTimes(2);
    expect(
      queryClient.getQueryData<WindowManagerSnapshot>(
        windowManagerKeys.snapshot("workspace:test", "marketing")
      )?.revision
    ).toBe(2);

    await act(async () => revisionThree.resolve(snapshot(3)));
    expect(
      queryClient.getQueryData<WindowManagerSnapshot>(
        windowManagerKeys.snapshot("workspace:test", "marketing")
      )?.revision
    ).toBe(3);
  });

  it("Should accept only newer bound-client frames and recover only explicit missing clients", () => {
    const queryClient = new QueryClient();
    const { factory, sockets } = createSocketFactory();
    const onClient = vi.fn();
    const onClientInvalidated = vi.fn();

    const { rerender } = renderHook(
      ({
        currentClient,
        registrationEpoch,
      }: {
        currentClient: WindowManagerClientView;
        registrationEpoch: number;
      }) =>
        useWindowManagerStream({
          workspaceId: "workspace:test",
          profileId: "marketing",
          clientId: "client:web",
          registrationEpoch,
          currentClient,
          enabled: true,
          afterRevision: 1,
          socketFactory: factory,
          onStatusChange: vi.fn(),
          onSnapshot: vi.fn(),
          onClient,
          onClientInvalidated,
          onError: vi.fn(),
        }),
      {
        initialProps: { currentClient: client(2), registrationEpoch: 0 },
        wrapper: wrapper(queryClient),
      }
    );

    act(() => {
      sockets[0]?.message(rawClientFrame(2));
      sockets[0]?.message(rawClientFrame(3, "client:peer"));
      sockets[0]?.message(rawClientFrame(3));
    });
    expect(onClient).toHaveBeenCalledOnce();
    expect(onClient).toHaveBeenCalledWith(expect.objectContaining(client(3)));

    act(() => sockets[0]?.fail());
    expect(onClientInvalidated).not.toHaveBeenCalled();

    act(() => {
      sockets[0]?.message({
        type: "error",
        error: {
          error: "client not found",
          code: "window_manager_client_not_found",
          workspace_id: "workspace:test",
        },
      });
      sockets[0]?.message({
        type: "error",
        error: {
          error: "client not found",
          code: "window_manager_client_not_found",
          workspace_id: "workspace:test",
        },
      });
      sockets[0]?.fail();
    });
    expect(onClientInvalidated).toHaveBeenCalledOnce();

    onClient.mockClear();
    rerender({ currentClient: client(1), registrationEpoch: 1 });
    act(() => sockets[1]?.message(rawClientFrame(2)));
    expect(onClient).toHaveBeenCalledOnce();
    expect(onClient).toHaveBeenCalledWith(expect.objectContaining(client(2)));
  });

  it("Should retry a failed snapshot refresh until Query reaches the announced revision", async () => {
    vi.useFakeTimers();
    const queryClient = new QueryClient();
    queryClient.setQueryData(
      windowManagerKeys.snapshot("workspace:test", "marketing"),
      snapshot(1)
    );
    const { factory, sockets } = createSocketFactory();
    const onError = vi.fn();
    vi.mocked(fetchWindowManagerSnapshot)
      .mockRejectedValueOnce(new Error("temporary read failure"))
      .mockResolvedValueOnce(snapshot(3));

    renderHook(
      () =>
        useWindowManagerStream({
          workspaceId: "workspace:test",
          profileId: "marketing",
          clientId: "client:web",
          registrationEpoch: 0,
          currentClient: client(1),
          enabled: true,
          afterRevision: 1,
          socketFactory: factory,
          onStatusChange: vi.fn(),
          onSnapshot: vi.fn(),
          onClient: vi.fn(),
          onClientInvalidated: vi.fn(),
          onError,
        }),
      { wrapper: wrapper(queryClient) }
    );

    act(() => sockets[0]?.message(rawEventFrame(3)));
    await act(async () => {
      await Promise.resolve();
    });
    expect(onError).toHaveBeenCalledWith(new Error("temporary read failure"));
    expect(fetchWindowManagerSnapshot).toHaveBeenCalledOnce();

    await act(() => vi.advanceTimersByTimeAsync(500));

    expect(fetchWindowManagerSnapshot).toHaveBeenCalledTimes(2);
    expect(
      queryClient.getQueryData<WindowManagerSnapshot>(
        windowManagerKeys.snapshot("workspace:test", "marketing")
      )?.revision
    ).toBe(3);
  });

  it("Should abort an in-flight snapshot refresh when the binding is disposed", () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(
      windowManagerKeys.snapshot("workspace:test", "marketing"),
      snapshot(1)
    );
    const { factory, sockets } = createSocketFactory();
    const pending = deferred<WindowManagerSnapshot>();
    vi.mocked(fetchWindowManagerSnapshot).mockReturnValueOnce(pending.promise);

    const { unmount } = renderHook(
      () =>
        useWindowManagerStream({
          workspaceId: "workspace:test",
          profileId: "marketing",
          clientId: "client:web",
          registrationEpoch: 0,
          currentClient: client(1),
          enabled: true,
          afterRevision: 1,
          socketFactory: factory,
          onStatusChange: vi.fn(),
          onSnapshot: vi.fn(),
          onClient: vi.fn(),
          onClientInvalidated: vi.fn(),
          onError: vi.fn(),
        }),
      { wrapper: wrapper(queryClient) }
    );

    act(() => sockets[0]?.message(rawEventFrame(2)));
    const signal = vi.mocked(fetchWindowManagerSnapshot).mock.calls[0]?.[2];
    expect(signal?.aborted).toBe(false);

    unmount();

    expect(signal?.aborted).toBe(true);
  });

  it("Should acknowledge client commands before returning their terminal result", async () => {
    const queryClient = new QueryClient();
    const { factory, sockets } = createSocketFactory();
    const onClientCommand = vi.fn().mockResolvedValue({ opened: true });

    renderHook(
      () =>
        useWindowManagerStream({
          workspaceId: "workspace:test",
          profileId: "marketing",
          clientId: "client:web",
          registrationEpoch: 0,
          currentClient: client(1),
          enabled: true,
          afterRevision: 1,
          socketFactory: factory,
          onStatusChange: vi.fn(),
          onSnapshot: vi.fn(),
          onClient: vi.fn(),
          onClientCommand,
          onClientInvalidated: vi.fn(),
          onError: vi.fn(),
        }),
      { wrapper: wrapper(queryClient) }
    );

    act(() =>
      sockets[0]?.message({
        type: "client_command",
        workspace_id: "workspace:test",
        command_id: "invocation-a",
        op: "palette.open",
        payload: { args: {} },
      })
    );

    expect(sockets[0]?.send).toHaveBeenNthCalledWith(
      1,
      JSON.stringify({ type: "client_command_ack", command_id: "invocation-a" })
    );
    await waitFor(() => expect(sockets[0]?.send).toHaveBeenCalledTimes(2));
    expect(sockets[0]?.send).toHaveBeenNthCalledWith(
      2,
      JSON.stringify({
        type: "client_command_result",
        command_id: "invocation-a",
        result: { opened: true },
      })
    );
    expect(onClientCommand).toHaveBeenCalledWith({
      commandId: "invocation-a",
      op: "palette.open",
      payload: { args: {} },
    });
  });

  it("Should acknowledge a rejected client command and write its error result", async () => {
    const queryClient = new QueryClient();
    const { factory, sockets } = createSocketFactory();
    const onClientCommand = vi.fn().mockRejectedValueOnce(new Error("palette is busy"));

    renderHook(
      () =>
        useWindowManagerStream({
          workspaceId: "workspace:test",
          profileId: "marketing",
          clientId: "client:web",
          registrationEpoch: 0,
          currentClient: client(1),
          enabled: true,
          afterRevision: 1,
          socketFactory: factory,
          onStatusChange: vi.fn(),
          onSnapshot: vi.fn(),
          onClient: vi.fn(),
          onClientCommand,
          onClientInvalidated: vi.fn(),
          onError: vi.fn(),
        }),
      { wrapper: wrapper(queryClient) }
    );

    act(() =>
      sockets[0]?.message({
        type: "client_command",
        workspace_id: "workspace:test",
        command_id: "invocation-b",
        op: "palette.open",
        payload: { args: {} },
      })
    );

    expect(sockets[0]?.send).toHaveBeenNthCalledWith(
      1,
      JSON.stringify({ type: "client_command_ack", command_id: "invocation-b" })
    );
    await waitFor(() => expect(sockets[0]?.send).toHaveBeenCalledTimes(2));
    expect(sockets[0]?.send).toHaveBeenNthCalledWith(
      2,
      JSON.stringify({
        type: "client_command_result",
        command_id: "invocation-b",
        error: "palette is busy",
      })
    );
  });

  it("Should ignore client commands whose frame workspace does not match the bound workspace", () => {
    const queryClient = new QueryClient();
    const { factory, sockets } = createSocketFactory();
    const onClientCommand = vi.fn();

    renderHook(
      () =>
        useWindowManagerStream({
          workspaceId: "workspace:test",
          profileId: "marketing",
          clientId: "client:web",
          registrationEpoch: 0,
          currentClient: client(1),
          enabled: true,
          afterRevision: 1,
          socketFactory: factory,
          onStatusChange: vi.fn(),
          onSnapshot: vi.fn(),
          onClient: vi.fn(),
          onClientCommand,
          onClientInvalidated: vi.fn(),
          onError: vi.fn(),
        }),
      { wrapper: wrapper(queryClient) }
    );

    act(() =>
      sockets[0]?.message({
        type: "client_command",
        workspace_id: "workspace:other",
        command_id: "invocation-foreign",
        op: "palette.open",
        payload: { args: {} },
      })
    );

    expect(onClientCommand).not.toHaveBeenCalled();
    expect(sockets[0]?.send).not.toHaveBeenCalled();
  });

  it("Should debounce client context refreshes over the bound socket", async () => {
    vi.useFakeTimers();
    const queryClient = new QueryClient();
    const { factory, sockets } = createSocketFactory();
    const initialContext: WindowManagerClientContextInput = {
      scopeGlobal: false,
      focusedSessionState: null,
      workspaceTrusted: true,
      destinationIntent: null,
      globalShortcuts: [],
    };

    const { rerender } = renderHook(
      ({ clientContext }: { clientContext: WindowManagerClientContextInput }) =>
        useWindowManagerStream({
          workspaceId: "workspace:test",
          profileId: "marketing",
          clientId: "client:web",
          registrationEpoch: 0,
          currentClient: client(1),
          clientContext,
          enabled: true,
          afterRevision: 1,
          socketFactory: factory,
          onStatusChange: vi.fn(),
          onSnapshot: vi.fn(),
          onClient: vi.fn(),
          onClientInvalidated: vi.fn(),
          onError: vi.fn(),
        }),
      { initialProps: { clientContext: initialContext }, wrapper: wrapper(queryClient) }
    );

    act(() => sockets[0]?.open());
    rerender({
      clientContext: { ...initialContext, focusedSessionState: "waiting" },
    });
    rerender({
      clientContext: {
        ...initialContext,
        scopeGlobal: true,
        focusedSessionState: "running",
        destinationIntent: { pathname: "/sessions/session-a", search: {} },
      },
    });

    await act(() => vi.advanceTimersByTimeAsync(74));
    expect(sockets[0]?.send).not.toHaveBeenCalled();
    await act(() => vi.advanceTimersByTimeAsync(1));
    expect(sockets[0]?.send).toHaveBeenCalledOnce();
    expect(sockets[0]?.send).toHaveBeenCalledWith(
      JSON.stringify({
        type: "client_context",
        context: {
          scope_global: true,
          focused_session_state: "running",
          workspace_trusted: true,
          destination_intent: { pathname: "/sessions/session-a", search: {} },
          global_shortcuts: [],
        },
      })
    );
  });
});
