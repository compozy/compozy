// @vitest-environment jsdom

import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({ activeWorkspaceId: "ws_alpha" }),
}));

import {
  LAST_READ_STORAGE_KEY_FOR_TESTS,
  buildLastReadStorageKey,
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

    const stored = JSON.parse(
      window.localStorage.getItem(LAST_READ_STORAGE_KEY_FOR_TESTS) ?? "{}"
    ) as Record<string, string>;
    expect(stored).toMatchObject({
      "ws_alpha:builders:thread:thread_one": "2026-04-13T10:00:00Z",
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

    const stored = JSON.parse(
      window.localStorage.getItem(LAST_READ_STORAGE_KEY_FOR_TESTS) ?? "{}"
    ) as Record<string, string>;
    expect(stored).toMatchObject({
      "ws_alpha:builders:thread:thread_one": "2026-04-13T10:00:00Z",
      "ws_alpha:builders:thread:thread_two": "2026-04-13T10:01:00Z",
    });
  });

  it("ignores empty-or-undefined timestamps", () => {
    const { result } = renderHook(() => useLastRead());
    act(() => {
      result.current.markRead(
        { channel: "builders", surface: "thread", containerId: "thread_one" },
        null
      );
      result.current.markRead(
        { channel: "builders", surface: "thread", containerId: "thread_one" },
        undefined
      );
    });
    expect(
      result.current.lastReadAt({
        channel: "builders",
        surface: "thread",
        containerId: "thread_one",
      })
    ).toBeNull();
  });
});
