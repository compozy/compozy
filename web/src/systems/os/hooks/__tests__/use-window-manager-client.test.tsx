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
import type { WindowManagerRegisteredClientView } from "../../lib/window-manager-types";
import { isDesktopShell } from "../../lib/desktop-shell-bridge";
import {
  WINDOW_MANAGER_CLIENT_ID_MEMORY_KEY,
  WINDOW_MANAGER_CLIENT_ID_STORAGE_KEY,
} from "../../lib/window-manager-client-identity";
import { useWindowManagerClient } from "../use-window-manager-client";

vi.mock("../../adapters/window-manager-api", () => ({
  registerWindowManagerClient: vi.fn(),
  unregisterWindowManagerClient: vi.fn(),
}));

vi.mock("../../lib/desktop-shell-bridge", () => ({
  isDesktopShell: vi.fn(() => false),
}));

const STORAGE_KEY = WINDOW_MANAGER_CLIENT_ID_STORAGE_KEY;

function client(presentationRevision = 1): WindowManagerRegisteredClientView {
  return {
    workspaceId: "workspace:test",
    clientId: "client:stable",
    kind: "browser",
    presentationRevision,
    contextRevision: 1,
    activeDesktopId: "desktop:main",
    focusedWindowId: null,
    focusOrder: [],
    stackActive: {},
    paletteContext: {
      windowFocused: false,
      windowFloating: false,
      windowStacked: false,
      desktopWindowCount: 0,
      scopeGlobal: false,
      shellDesktop: false,
      focusedSessionState: null,
      workspaceTrusted: true,
      destinationIntent: null,
    },
    connectedAt: "2026-07-22T00:00:00Z",
    attachmentToken: "attachment-token",
    globalShortcuts: [],
  };
}

beforeEach(() => {
  Reflect.deleteProperty(globalThis, WINDOW_MANAGER_CLIENT_ID_MEMORY_KEY);
  window.localStorage.clear();
  window.localStorage.setItem(STORAGE_KEY, "client:stable");
  vi.mocked(isDesktopShell).mockReturnValue(false);
});

afterEach(() => {
  vi.useRealTimers();
  vi.clearAllMocks();
});

describe("useWindowManagerClient", () => {
  it("Should register once with the stable ID and no active desktop hint", async () => {
    vi.mocked(registerWindowManagerClient).mockResolvedValue(client());
    const { result, rerender, unmount } = renderHook(
      ({ workspaceId }: { workspaceId: string | null }) => useWindowManagerClient(workspaceId),
      { initialProps: { workspaceId: "workspace:test" } }
    );

    await waitFor(() => expect(result.current.client).toEqual(client()));
    rerender({ workspaceId: "workspace:test" });
    unmount();

    expect(registerWindowManagerClient).toHaveBeenCalledOnce();
    expect(registerWindowManagerClient).toHaveBeenCalledWith(
      "workspace:test",
      "client:stable",
      undefined,
      "browser",
      expect.any(AbortSignal)
    );
    expect(unregisterWindowManagerClient).not.toHaveBeenCalled();
  });

  it("Should retry a failed registration with the same stable ID", async () => {
    vi.useFakeTimers();
    vi.mocked(registerWindowManagerClient)
      .mockRejectedValueOnce(new Error("daemon restarting"))
      .mockResolvedValueOnce(client());
    const { result } = renderHook(() => useWindowManagerClient("workspace:test"));

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

  it("Should pause failed-registration retries while the document is hidden", async () => {
    vi.useFakeTimers();
    let visibilityState: DocumentVisibilityState = "visible";
    vi.spyOn(document, "visibilityState", "get").mockImplementation(() => visibilityState);
    vi.mocked(registerWindowManagerClient)
      .mockRejectedValueOnce(new Error("daemon restarting"))
      .mockResolvedValueOnce(client());
    const { result } = renderHook(() => useWindowManagerClient("workspace:test"));

    await act(async () => {
      await Promise.resolve();
    });
    expect(result.current.status).toBe("error");

    act(() => {
      visibilityState = "hidden";
      document.dispatchEvent(new Event("visibilitychange"));
    });
    await act(() => vi.advanceTimersByTimeAsync(8_000));
    expect(registerWindowManagerClient).toHaveBeenCalledOnce();

    act(() => {
      visibilityState = "visible";
      document.dispatchEvent(new Event("visibilitychange"));
    });
    await act(async () => {
      await Promise.resolve();
    });

    expect(registerWindowManagerClient).toHaveBeenCalledTimes(2);
    expect(result.current.status).toBe("registered");
  });

  it("Should perform one stable-ID registration for one explicit recovery request", async () => {
    vi.mocked(registerWindowManagerClient)
      .mockResolvedValueOnce(client(1))
      .mockResolvedValueOnce(client(2));
    const { result } = renderHook(() => useWindowManagerClient("workspace:test"));

    await waitFor(() => expect(result.current.status).toBe("registered"));
    act(() => result.current.reregister());
    expect(result.current.registrationEpoch).toBe(1);
    expect(result.current.client).toBeNull();
    await waitFor(() => expect(result.current.client?.presentationRevision).toBe(2));

    expect(registerWindowManagerClient).toHaveBeenCalledTimes(2);
    expect(result.current.clientId).toBe("client:stable");
  });

  it("Should hide a registered client immediately when the workspace changes", async () => {
    let resolveNext!: (view: WindowManagerRegisteredClientView) => void;
    vi.mocked(registerWindowManagerClient)
      .mockResolvedValueOnce(client())
      .mockImplementationOnce(() => new Promise(resolve => (resolveNext = resolve)));
    const { result, rerender } = renderHook(
      ({ workspaceId }: { workspaceId: string }) => useWindowManagerClient(workspaceId),
      { initialProps: { workspaceId: "workspace:test" } }
    );

    await waitFor(() => expect(result.current.status).toBe("registered"));
    rerender({ workspaceId: "workspace:next" });

    expect(result.current.client).toBeNull();
    expect(result.current.status).toBe("registering");

    resolveNext({ ...client(2), workspaceId: "workspace:next" });
    await waitFor(() => expect(result.current.client?.workspaceId).toBe("workspace:next"));
  });

  it("Should register as a shell client when the desktop bridge is present", async () => {
    vi.mocked(isDesktopShell).mockReturnValue(true);
    vi.mocked(registerWindowManagerClient).mockResolvedValue({ ...client(), kind: "shell" });
    const { result } = renderHook(() => useWindowManagerClient("workspace:test"));

    await waitFor(() => expect(result.current.status).toBe("registered"));
    expect(registerWindowManagerClient).toHaveBeenCalledWith(
      "workspace:test",
      "client:stable",
      undefined,
      "shell",
      expect.any(AbortSignal)
    );
  });
});
