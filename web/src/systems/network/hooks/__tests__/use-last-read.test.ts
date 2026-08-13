// @vitest-environment jsdom

import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({ runtimeWorkspaceId: "ws_alpha" }),
}));

import {
  LAST_READ_STORAGE_KEY,
  buildLastReadStorageKey,
  networkLastReadLogic,
  useLastRead,
} from "../use-last-read";

describe("useLastRead", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    window.localStorage.clear();
  });

  it("namespaces the storage key with workspace + channel + surface + container id", () => {
    const key = buildLastReadStorageKey({
      workspaceId: "ws_alpha",
      channel: "builders",
      surface: "thread",
      containerId: "thread_one",
    });
    expect(key).toBe("ws_alpha:builders:thread:thread_one");
  });

  it("treats threads-tab and directs-tab boundaries as different containers", () => {
    const threadKey = buildLastReadStorageKey({
      workspaceId: "ws_alpha",
      channel: "builders",
      surface: "thread",
      containerId: "shared-id",
    });
    const directKey = buildLastReadStorageKey({
      workspaceId: "ws_alpha",
      channel: "builders",
      surface: "direct",
      containerId: "shared-id",
    });
    expect(threadKey).not.toBe(directKey);
  });

  it("persists per-container last-read marks across visits", () => {
    const { result } = renderHook(() => useLastRead());
    act(() => {
      result.current.markRead(
        { channel: "builders", surface: "thread", containerId: "thread_one" },
        "2026-04-13T10:00:00Z"
      );
    });
    expect(
      result.current.lastReadAt({
        channel: "builders",
        surface: "thread",
        containerId: "thread_one",
      })
    ).toBe("2026-04-13T10:00:00Z");

    const stored = JSON.parse(window.localStorage.getItem(LAST_READ_STORAGE_KEY) ?? "{}") as {
      context?: { byWorkspace?: Record<string, Record<string, string>> };
    };
    expect(stored.context?.byWorkspace).toMatchObject({
      ws_alpha: {
        "ws_alpha:builders:thread:thread_one": "2026-04-13T10:00:00Z",
      },
    });
  });

  it("does not bleed last-read between threads-tab and directs-tab same id", () => {
    const { result } = renderHook(() => useLastRead());
    act(() => {
      result.current.markRead(
        { channel: "builders", surface: "thread", containerId: "shared-id" },
        "2026-04-13T10:00:00Z"
      );
    });
    expect(
      result.current.lastReadAt({
        channel: "builders",
        surface: "direct",
        containerId: "shared-id",
      })
    ).toBeNull();
  });

  it("preserves rapid marks written before React commits a render", () => {
    const { result } = renderHook(() => useLastRead());
    act(() => {
      result.current.markRead(
        { channel: "builders", surface: "thread", containerId: "thread_one" },
        "2026-04-13T10:00:00Z"
      );
      result.current.markRead(
        { channel: "builders", surface: "thread", containerId: "thread_two" },
        "2026-04-13T10:01:00Z"
      );
    });

    const stored = JSON.parse(window.localStorage.getItem(LAST_READ_STORAGE_KEY) ?? "{}") as {
      context?: { byWorkspace?: Record<string, Record<string, string>> };
    };
    expect(stored.context?.byWorkspace).toMatchObject({
      ws_alpha: {
        "ws_alpha:builders:thread:thread_one": "2026-04-13T10:00:00Z",
        "ws_alpha:builders:thread:thread_two": "2026-04-13T10:01:00Z",
      },
    });
  });

  it("keeps workspace marks isolated in pure transitions", () => {
    const store = networkLastReadLogic.createStore();
    const [alpha] = store.transition(store.getInitialSnapshot(), {
      type: "lastReadMarked",
      workspaceId: "ws_alpha",
      key: "ws_alpha:builders:thread:thread_one",
      timestamp: "2026-04-13T10:00:00Z",
    });
    const [beta] = store.transition(alpha, {
      type: "lastReadMarked",
      workspaceId: "ws_beta",
      key: "ws_beta:builders:thread:thread_one",
      timestamp: "2026-04-13T10:01:00Z",
    });

    expect(beta.context.byWorkspace).toEqual({
      ws_alpha: { "ws_alpha:builders:thread:thread_one": "2026-04-13T10:00:00Z" },
      ws_beta: { "ws_beta:builders:thread:thread_one": "2026-04-13T10:01:00Z" },
    });
  });

  it("rehydrates a workspace mark broadcast by another tab", async () => {
    const { result } = renderHook(() => useLastRead({ workspaceId: "ws_cross_tab" }));
    const key = buildLastReadStorageKey({
      workspaceId: "ws_cross_tab",
      channel: "builders",
      surface: "thread",
      containerId: "thread_external",
    });
    const state = JSON.stringify({
      context: {
        byWorkspace: {
          ws_cross_tab: {
            [key]: "2026-04-13T10:02:00Z",
            "ws_cross_tab:builders:thread:invalid": "not-a-timestamp",
          },
        },
      },
      version: 0,
    });

    window.localStorage.setItem(LAST_READ_STORAGE_KEY, state);
    act(() => {
      window.dispatchEvent(
        new StorageEvent("storage", { key: LAST_READ_STORAGE_KEY, newValue: state })
      );
    });

    await waitFor(() => {
      expect(
        result.current.lastReadAt({
          channel: "builders",
          surface: "thread",
          containerId: "thread_external",
        })
      ).toBe("2026-04-13T10:02:00Z");
      expect(
        result.current.lastReadAt({
          channel: "builders",
          surface: "thread",
          containerId: "invalid",
        })
      ).toBeNull();
    });
  });

  it("ignores empty, undefined, or invalid timestamps", () => {
    const { result } = renderHook(() => useLastRead());
    act(() => {
      result.current.markRead(
        { channel: "builders", surface: "thread", containerId: "empty_timestamp" },
        null
      );
      result.current.markRead(
        { channel: "builders", surface: "thread", containerId: "empty_timestamp" },
        undefined
      );
      result.current.markRead(
        { channel: "builders", surface: "thread", containerId: "empty_timestamp" },
        "not-a-timestamp"
      );
    });
    expect(
      result.current.lastReadAt({
        channel: "builders",
        surface: "thread",
        containerId: "empty_timestamp",
      })
    ).toBeNull();
  });
});
