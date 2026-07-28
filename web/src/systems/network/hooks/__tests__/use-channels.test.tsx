// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor, act } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

import { PINNED_CHANNELS_STORAGE_KEY_FOR_TESTS, useNetworkChannels } from "../use-channels";

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({ activeWorkspaceId: "w1" }),
}));

vi.mock("../../adapters/network-api", () => ({
  listNetworkChannels: vi.fn().mockResolvedValue({
    channels: [
      { channel: "ops", workspace_id: "w1", created_at: "2026-04-17T14:00:00Z", created_by: "ops" },
      {
        channel: "alpha",
        workspace_id: "w1",
        created_at: "2026-04-17T14:00:00Z",
        created_by: "ops",
      },
      {
        channel: "design",
        workspace_id: "w1",
        created_at: "2026-04-17T14:00:00Z",
        created_by: "ops",
      },
    ],
  }),
}));

function makeWrapper() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Number.POSITIVE_INFINITY } },
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  };
}

describe("useNetworkChannels", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    window.localStorage.clear();
  });

  it("preserves backend channel order and surfaces pinned channels separately", async () => {
    const { result } = renderHook(() => useNetworkChannels({ enabled: true }), {
      wrapper: makeWrapper(),
    });

    await waitFor(() => {
      expect(result.current.channels.map(channel => channel.channel)).toEqual([
        "ops",
        "alpha",
        "design",
      ]);
    });

    expect(result.current.pinned).toEqual([]);
    expect(result.current.unpinned.map(channel => channel.channel)).toEqual([
      "ops",
      "alpha",
      "design",
    ]);
  });

  it("toggles pinned channels through localStorage", async () => {
    const { result } = renderHook(() => useNetworkChannels({ enabled: true }), {
      wrapper: makeWrapper(),
    });

    await waitFor(() => {
      expect(result.current.channels.length).toBe(3);
    });

    act(() => {
      result.current.togglePinned("ops");
    });

    await waitFor(() => {
      expect(result.current.isPinned("ops")).toBe(true);
    });
    expect(result.current.pinned.map(channel => channel.channel)).toEqual(["ops"]);

    const stored = JSON.parse(
      window.localStorage.getItem(PINNED_CHANNELS_STORAGE_KEY_FOR_TESTS) ?? "{}"
    ) as Record<string, string[]>;
    expect(stored).toEqual({ w1: ["ops"] });

    act(() => {
      result.current.togglePinned("ops");
    });
    await waitFor(() => {
      expect(result.current.isPinned("ops")).toBe(false);
    });
  });

  it("preserves rapid pin updates before React commits a render", async () => {
    const { result } = renderHook(() => useNetworkChannels({ enabled: true }), {
      wrapper: makeWrapper(),
    });

    await waitFor(() => {
      expect(result.current.channels.length).toBe(3);
    });

    act(() => {
      result.current.togglePinned("ops");
      result.current.togglePinned("design");
    });

    expect(result.current.pinnedIds).toEqual(["design", "ops"]);
    expect(
      JSON.parse(window.localStorage.getItem(PINNED_CHANNELS_STORAGE_KEY_FOR_TESTS) ?? "{}")
    ).toEqual({ w1: ["design", "ops"] });
  });
});
