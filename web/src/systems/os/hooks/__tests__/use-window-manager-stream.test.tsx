// Suite: window-manager stream hook
// Invariant: one client-bound socket survives topology advances, reconnects with the latest fence,
// serializes burst refreshes to the highest announced revision, and accepts only newer bound-client
// presentation frames.
// Owning layer: the WebSocket → TanStack Query/client-presentation bridge.
import { act, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { PropsWithChildren } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { fetchWindowManagerSnapshot } from "../../adapters/window-manager-api";
import { windowManagerKeys } from "../../lib/window-manager-query";
import type {
  WindowManagerClientView,
  WindowManagerSnapshot,
} from "../../lib/window-manager-types";
import {
  useWindowManagerStream,
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

class FakeSocket implements WindowManagerSocket {
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent<unknown>) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  close = vi.fn();

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
    version: 1,
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
      },
    ],
    windows: {},
    history: { undo: [], redo: [] },
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
    connectedAt: "2026-07-22T00:00:00Z",
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
      presentation_revision: presentationRevision,
      active_desktop_id: "desktop:main",
      focus_order: [],
      connected_at: "2026-07-22T00:00:00Z",
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
      version: 1,
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
        },
      ],
      windows: {},
      history: { undo: [], redo: [] },
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
  it("Should keep one socket across topology advances and reconnect with the latest cached fence", async () => {
    vi.useFakeTimers();
    const queryClient = new QueryClient();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test"), snapshot(1));
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
      "/api/workspaces/workspace%3Atest/window-manager/stream?after_revision=1&client_id=client%3Aweb"
    );
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test"), snapshot(4));
    rerender({ revision: 5 });
    expect(factory).toHaveBeenCalledOnce();

    act(() => sockets[0]?.disconnect());
    await act(() => vi.advanceTimersByTimeAsync(500));

    expect(factory).toHaveBeenCalledTimes(2);
    expect(factory).toHaveBeenLastCalledWith(
      "/api/workspaces/workspace%3Atest/window-manager/stream?after_revision=5&client_id=client%3Aweb"
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
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test"), snapshot(1));
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
      queryClient.getQueryData<WindowManagerSnapshot>(windowManagerKeys.snapshot("workspace:test"))
        ?.revision
    ).toBe(2);

    await act(async () => revisionThree.resolve(snapshot(3)));
    expect(
      queryClient.getQueryData<WindowManagerSnapshot>(windowManagerKeys.snapshot("workspace:test"))
        ?.revision
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
    expect(onClient).toHaveBeenCalledWith(client(3));

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
    expect(onClient).toHaveBeenCalledWith(client(2));
  });

  it("Should retry a failed snapshot refresh until Query reaches the announced revision", async () => {
    vi.useFakeTimers();
    const queryClient = new QueryClient();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test"), snapshot(1));
    const { factory, sockets } = createSocketFactory();
    const onError = vi.fn();
    vi.mocked(fetchWindowManagerSnapshot)
      .mockRejectedValueOnce(new Error("temporary read failure"))
      .mockResolvedValueOnce(snapshot(3));

    renderHook(
      () =>
        useWindowManagerStream({
          workspaceId: "workspace:test",
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
      queryClient.getQueryData<WindowManagerSnapshot>(windowManagerKeys.snapshot("workspace:test"))
        ?.revision
    ).toBe(3);
  });

  it("Should abort an in-flight snapshot refresh when the binding is disposed", () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test"), snapshot(1));
    const { factory, sockets } = createSocketFactory();
    const pending = deferred<WindowManagerSnapshot>();
    vi.mocked(fetchWindowManagerSnapshot).mockReturnValueOnce(pending.promise);

    const { unmount } = renderHook(
      () =>
        useWindowManagerStream({
          workspaceId: "workspace:test",
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
    const signal = vi.mocked(fetchWindowManagerSnapshot).mock.calls[0]?.[1];
    expect(signal?.aborted).toBe(false);

    unmount();

    expect(signal?.aborted).toBe(true);
  });
});
