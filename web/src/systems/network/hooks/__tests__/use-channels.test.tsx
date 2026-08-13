// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor, act } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

import {
  createPinnedChannelsStore,
  PINNED_CHANNELS_STORAGE_KEY,
  rehydratePinnedChannelsStore,
} from "../pinned-channels-store";
import { useNetworkChannels } from "../use-channels";

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({ runtimeWorkspaceId: "w1" }),
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

  it("Should defer pinned-channel storage reads until explicit hydration", async () => {
    window.localStorage.setItem(PINNED_CHANNELS_STORAGE_KEY, JSON.stringify({ w1: ["ops"] }));
    const getItem = vi.spyOn(Storage.prototype, "getItem");

    const store = createPinnedChannelsStore("w1");

    expect(getItem).not.toHaveBeenCalled();
    expect(store.getSnapshot().context.pinnedIds).toEqual([]);

    await rehydratePinnedChannelsStore(store);

    expect(getItem).toHaveBeenCalledWith(PINNED_CHANNELS_STORAGE_KEY);
    expect(store.getSnapshot().context.pinnedIds).toEqual(["ops"]);
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
      window.localStorage.getItem(PINNED_CHANNELS_STORAGE_KEY) ?? "{}"
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
    expect(JSON.parse(window.localStorage.getItem(PINNED_CHANNELS_STORAGE_KEY) ?? "{}")).toEqual({
      w1: ["design", "ops"],
    });
  });

  it("Should isolate workspace pins and reject a stale storage notification", async () => {
    window.localStorage.setItem(
      PINNED_CHANNELS_STORAGE_KEY,
      JSON.stringify({ w1: ["ops"], w2: ["design"] })
    );
    const staleStorageHandlers: EventListener[] = [];
    const originalAddEventListener = window.addEventListener;
    const addEventListenerSpy = vi
      .spyOn(window, "addEventListener")
      .mockImplementation((type, listener, options) => {
        if (type === "storage") staleStorageHandlers.push(listener as EventListener);
        originalAddEventListener.call(window, type, listener, options);
      });

    try {
      const { result, rerender } = renderHook(
        ({ workspaceId }: { workspaceId: string }) =>
          useNetworkChannels({ enabled: true, workspaceId }),
        {
          initialProps: { workspaceId: "w1" },
          wrapper: makeWrapper(),
        }
      );

      await waitFor(() => {
        expect(result.current.pinnedIds).toEqual(["ops"]);
      });
      const staleHandler = staleStorageHandlers[0];
      if (!staleHandler) throw new Error("The initial storage listener did not mount.");

      rerender({ workspaceId: "w2" });

      await waitFor(() => {
        expect(result.current.pinnedIds).toEqual(["design"]);
      });
      const currentHandler = staleStorageHandlers.at(-1);
      if (!currentHandler) throw new Error("The current storage listener did not mount.");

      window.localStorage.setItem(
        PINNED_CHANNELS_STORAGE_KEY,
        JSON.stringify({ w1: ["ops"], w2: ["ops", "design"] })
      );
      act(() => {
        currentHandler(new StorageEvent("storage", { key: PINNED_CHANNELS_STORAGE_KEY }));
      });
      await waitFor(() => {
        expect(result.current.pinnedIds).toEqual(["ops", "design"]);
      });

      act(() => {
        staleHandler(new StorageEvent("storage", { key: PINNED_CHANNELS_STORAGE_KEY }));
      });
      expect(result.current.pinnedIds).toEqual(["ops", "design"]);
      expect(result.current.pinned.map(channel => channel.channel)).toEqual(["ops", "design"]);
    } finally {
      addEventListenerSpy.mockRestore();
    }
  });
});
