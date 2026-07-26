// Suite: window-manager client registration hook
// Invariant: browser registration reuses one stable client ID, omits active_desktop_id, never
// unregisters during React cleanup, and retries registration with bounded backoff.
// Owning layer: the browser-client registration lifecycle.
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  registerWindowManagerClient,
  unregisterWindowManagerClient,
} from "../../adapters/window-manager-api";
import type { WindowManagerClientView } from "../../lib/window-manager-types";
import { useWindowManagerClient } from "../use-window-manager-client";

vi.mock("../../adapters/window-manager-api", () => ({
  registerWindowManagerClient: vi.fn(),
  unregisterWindowManagerClient: vi.fn(),
}));

const STORAGE_KEY = "agh.window-manager.client-id";

function client(presentationRevision = 1): WindowManagerClientView {
  return {
    workspaceId: "workspace:test",
    clientId: "client:stable",
    presentationRevision,
    activeDesktopId: "desktop:main",
    focusedWindowId: null,
    focusOrder: [],
    connectedAt: "2026-07-22T00:00:00Z",
  };
}

beforeEach(() => {
  window.localStorage.clear();
  window.localStorage.setItem(STORAGE_KEY, "client:stable");
});

afterEach(() => {
  vi.useRealTimers();
  vi.clearAllMocks();
});

describe("useWindowManagerClient", () => {
  it("Should register once with the stable ID and no active desktop hint", async () => {
    vi.mocked(registerWindowManagerClient).mockResolvedValue(client());
    const onClientChange = vi.fn();
    const { rerender, unmount } = renderHook(
      ({ workspaceId }: { workspaceId: string | null }) =>
        useWindowManagerClient(workspaceId, onClientChange),
      { initialProps: { workspaceId: "workspace:test" } }
    );

    await waitFor(() => expect(onClientChange).toHaveBeenCalledWith(client()));
    rerender({ workspaceId: "workspace:test" });
    unmount();

    expect(registerWindowManagerClient).toHaveBeenCalledOnce();
    expect(registerWindowManagerClient).toHaveBeenCalledWith(
      "workspace:test",
      "client:stable",
      undefined,
      expect.any(AbortSignal)
    );
    expect(unregisterWindowManagerClient).not.toHaveBeenCalled();
  });

  it("Should retry a failed registration with the same stable ID", async () => {
    vi.useFakeTimers();
    vi.mocked(registerWindowManagerClient)
      .mockRejectedValueOnce(new Error("daemon restarting"))
      .mockResolvedValueOnce(client());
    const onClientChange = vi.fn();
    const { result } = renderHook(() => useWindowManagerClient("workspace:test", onClientChange));

    await act(async () => {
      await Promise.resolve();
    });
    expect(result.current.status).toBe("error");

    await act(() => vi.advanceTimersByTimeAsync(500));
    await act(async () => {
      await Promise.resolve();
    });

    expect(result.current.status).toBe("registered");
    expect(registerWindowManagerClient).toHaveBeenCalledTimes(2);
    expect(vi.mocked(registerWindowManagerClient).mock.calls.map(call => call.slice(0, 3))).toEqual(
      [
        ["workspace:test", "client:stable", undefined],
        ["workspace:test", "client:stable", undefined],
      ]
    );
  });

  it("Should perform one stable-ID registration for one explicit recovery request", async () => {
    vi.mocked(registerWindowManagerClient)
      .mockResolvedValueOnce(client(1))
      .mockResolvedValueOnce(client(2));
    const onClientChange = vi.fn();
    const { result } = renderHook(() => useWindowManagerClient("workspace:test", onClientChange));

    await waitFor(() => expect(result.current.status).toBe("registered"));
    onClientChange.mockClear();
    act(() => result.current.reregister());
    expect(result.current.registrationEpoch).toBe(1);
    expect(result.current.client).toBeNull();
    expect(onClientChange).toHaveBeenCalledWith(null);
    await waitFor(() => expect(result.current.client?.presentationRevision).toBe(2));

    expect(registerWindowManagerClient).toHaveBeenCalledTimes(2);
    expect(result.current.clientId).toBe("client:stable");
  });

  it("Should hide a registered client immediately when the workspace changes", async () => {
    let resolveNext!: (view: WindowManagerClientView) => void;
    vi.mocked(registerWindowManagerClient)
      .mockResolvedValueOnce(client())
      .mockImplementationOnce(() => new Promise(resolve => (resolveNext = resolve)));
    const onClientChange = vi.fn();
    const { result, rerender } = renderHook(
      ({ workspaceId }: { workspaceId: string }) =>
        useWindowManagerClient(workspaceId, onClientChange),
      { initialProps: { workspaceId: "workspace:test" } }
    );

    await waitFor(() => expect(result.current.status).toBe("registered"));
    rerender({ workspaceId: "workspace:next" });

    expect(result.current.client).toBeNull();
    expect(result.current.status).toBe("registering");

    resolveNext({ ...client(2), workspaceId: "workspace:next" });
    await waitFor(() => expect(result.current.client?.workspaceId).toBe("workspace:next"));
  });
});
