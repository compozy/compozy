import { QueryClient, QueryClientProvider, type InfiniteData } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import {
  applyBridgeHealthSnapshot,
  useBridgeHealthStream,
} from "@/systems/bridges/hooks/use-bridge-health-stream";
import type { BridgesListResponse } from "@/systems/bridges/types";

class FakeEventSource {
  public close = vi.fn();
  public onerror: ((event: Event) => void) | null = null;
  private readonly listeners = new Map<string, EventListenerOrEventListenerObject[]>();

  addEventListener(type: string, listener: EventListenerOrEventListenerObject) {
    const current = this.listeners.get(type) ?? [];
    current.push(listener);
    this.listeners.set(type, current);
  }

  removeEventListener(type: string, listener: EventListenerOrEventListenerObject) {
    const current = this.listeners.get(type) ?? [];
    this.listeners.set(
      type,
      current.filter(candidate => candidate !== listener)
    );
  }

  emit(type: string, data: unknown) {
    const event = new MessageEvent(type, {
      data: JSON.stringify(data),
    });

    for (const listener of this.listeners.get(type) ?? []) {
      if (typeof listener === "function") {
        listener(event);
        continue;
      }
      listener.handleEvent(event);
    }
  }
}

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

describe("applyBridgeHealthSnapshot", () => {
  it("merges matching health across infinite pages without deleting unrelated state", () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    queryClient.setQueryData<InfiniteData<BridgesListResponse>>(
      ["bridges", "list", "all", "ws_test", ""],
      {
        pageParams: [undefined, "cursor-2"],
        pages: [
          {
            bridge_health: {},
            bridges: [
              {
                created_at: "2026-04-13T12:00:00Z",
                display_name: "Support",
                enabled: true,
                extension_name: "ext-telegram",
                id: "brg_support",
                notification_suppress: false,
                platform: "telegram",
                routing_policy: { include_group: true, include_peer: true, include_thread: true },
                scope: "workspace",
                status: "starting",
                updated_at: "2026-04-13T12:00:00Z",
                workspace_id: "ws_test",
              },
            ],
            facets: {
              platforms: { telegram: 2 },
              statuses: {
                auth_required: 0,
                degraded: 0,
                disabled: 0,
                error: 0,
                ready: 1,
                starting: 1,
              },
            },
            page: { has_more: true, limit: 1, next_cursor: "cursor-2", total: 2 },
          },
          {
            bridge_health: {
              brg_other: {
                auth_failures_total: 0,
                bridge_instance_id: "brg_other",
                delivery_backlog: 9,
                delivery_dropped_total: 0,
                delivery_failures_total: 0,
                route_count: 1,
                status: "ready",
              },
            },
            bridges: [
              {
                created_at: "2026-04-13T12:00:00Z",
                display_name: "Other",
                enabled: true,
                extension_name: "ext-slack",
                id: "brg_other",
                notification_suppress: false,
                platform: "slack",
                routing_policy: { include_group: true, include_peer: true, include_thread: true },
                scope: "workspace",
                status: "ready",
                updated_at: "2026-04-13T12:00:00Z",
                workspace_id: "ws_test",
              },
            ],
            facets: {
              platforms: { telegram: 1 },
              statuses: {
                auth_required: 0,
                degraded: 0,
                disabled: 0,
                error: 0,
                ready: 1,
                starting: 0,
              },
            },
            page: { has_more: false, limit: 1, total: 2 },
          },
        ],
      }
    );
    queryClient.setQueryData(["bridges", "detail", "brg_support"], {
      bridge: {
        created_at: "2026-04-13T12:00:00Z",
        display_name: "Support",
        enabled: true,
        extension_name: "ext-telegram",
        id: "brg_support",
        platform: "telegram",
        routing_policy: { include_group: true, include_peer: true, include_thread: true },
        scope: "workspace",
        status: "starting",
        updated_at: "2026-04-13T12:00:00Z",
        workspace_id: "ws_test",
      },
      health: {
        auth_failures_total: 0,
        bridge_instance_id: "brg_support",
        delivery_backlog: 0,
        delivery_dropped_total: 0,
        delivery_failures_total: 0,
        route_count: 0,
        status: "starting",
      },
    });

    applyBridgeHealthSnapshot(queryClient, {
      bridge_health: {
        brg_other: {
          auth_failures_total: 0,
          bridge_instance_id: "brg_other",
          delivery_backlog: 9,
          delivery_dropped_total: 0,
          delivery_failures_total: 0,
          route_count: 1,
          status: "ready",
        },
        brg_support: {
          auth_failures_total: 1,
          bridge_instance_id: "brg_support",
          delivery_backlog: 2,
          delivery_dropped_total: 0,
          delivery_failures_total: 0,
          route_count: 3,
          status: "ready",
        },
      },
      generated_at: "2026-04-15T12:00:00Z",
    });

    const catalog = queryClient.getQueryData<InfiniteData<BridgesListResponse>>([
      "bridges",
      "list",
      "all",
      "ws_test",
      "",
    ]);
    expect(catalog?.pages).toHaveLength(2);
    expect(catalog?.pages[0].bridge_health.brg_support.status).toBe("ready");
    expect(catalog?.pages[1].bridge_health.brg_other.delivery_backlog).toBe(9);
    expect(catalog?.pages[0].bridge_health.brg_other).toBeUndefined();
    expect(
      queryClient.getQueryData<{
        health: { route_count: number; status: string };
      }>(["bridges", "detail", "brg_support"])?.health
    ).toEqual(
      expect.objectContaining({
        route_count: 3,
        status: "ready",
      })
    );
  });

  it("invalidates cached bridge routes when the live route count changes", () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const invalidateQueries = vi.spyOn(queryClient, "invalidateQueries");

    queryClient.setQueryData(["bridges", "routes", "brg_support"], []);

    applyBridgeHealthSnapshot(queryClient, {
      bridge_health: {
        brg_support: {
          auth_failures_total: 0,
          bridge_instance_id: "brg_support",
          delivery_backlog: 0,
          delivery_dropped_total: 0,
          delivery_failures_total: 0,
          route_count: 1,
          status: "ready",
        },
      },
      generated_at: "2026-04-15T12:00:00Z",
    });

    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: ["bridges", "routes", "brg_support"],
    });
  });
});

describe("useBridgeHealthStream", () => {
  it("subscribes to the stream and closes the event source on unmount", () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const eventSource = new FakeEventSource();
    const eventSourceFactory = vi.fn((_url: string) => eventSource);

    const { unmount } = renderHook(
      () =>
        useBridgeHealthStream({
          bridgeIds: ["brg_support"],
          eventSourceFactory,
          filters: { scope: "all", workspace_id: "ws_test" },
        }),
      {
        wrapper: createWrapper(queryClient),
      }
    );

    expect(eventSourceFactory).toHaveBeenCalledWith(
      "/api/bridges/health/stream?bridge_ids=brg_support&scope=all&workspace_id=ws_test"
    );

    act(() => {
      eventSource.emit("snapshot", {
        bridge_health: {
          brg_support: {
            auth_failures_total: 0,
            bridge_instance_id: "brg_support",
            delivery_backlog: 1,
            delivery_dropped_total: 0,
            delivery_failures_total: 0,
            route_count: 1,
            status: "ready",
          },
        },
        generated_at: "2026-04-15T12:00:00Z",
      });
    });

    unmount();

    expect(eventSource.close).toHaveBeenCalledTimes(1);
  });

  it("does not subscribe without ids and chunks every authorized request at 200 ids", () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const sources: FakeEventSource[] = [];
    const eventSourceFactory = vi.fn((_url: string) => {
      const source = new FakeEventSource();
      sources.push(source);
      return source;
    });
    const wrapper = createWrapper(queryClient);
    const empty = renderHook(() => useBridgeHealthStream({ bridgeIds: [], eventSourceFactory }), {
      wrapper,
    });
    expect(eventSourceFactory).not.toHaveBeenCalled();
    empty.unmount();

    const ids = Array.from({ length: 201 }, (_, index) => `brg_${index}`);
    const chunked = renderHook(
      () => useBridgeHealthStream({ bridgeIds: ids, eventSourceFactory }),
      { wrapper }
    );
    expect(eventSourceFactory).toHaveBeenCalledTimes(2);
    const idCounts = eventSourceFactory.mock.calls.map(([url]) => {
      const query = new URL(url, "http://agh.local").searchParams.get("bridge_ids") ?? "";
      return query.split(",").length;
    });
    expect(idCounts).toEqual([200, 1]);
    chunked.unmount();
    expect(sources.every(source => source.close.mock.calls.length === 1)).toBe(true);
  });
});
