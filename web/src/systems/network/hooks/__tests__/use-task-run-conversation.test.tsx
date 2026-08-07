import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { resetGatewayStreamAuth } from "@/lib/gateway-stream-auth";

import type { TaskRunNetworkProjection } from "../../types";
import { useTaskRunConversation } from "../use-task-run-conversation";

vi.mock("../use-messages", () => ({
  useNetworkMessages: () => ({
    messages: [],
    isLoading: false,
    isFetching: false,
    hasOlder: false,
    isLoadingOlder: false,
    loadOlder: vi.fn(),
    error: null,
  }),
}));

class FakeEventSource {
  static instance: FakeEventSource | null = null;
  static instances: FakeEventSource[] = [];
  readonly url: string;
  readonly listeners = new Map<string, Set<EventListener>>();
  closed = false;

  constructor(url: string | URL) {
    this.url = String(url);
    FakeEventSource.instance = this;
    FakeEventSource.instances.push(this);
  }

  addEventListener(type: string, listener: EventListener): void {
    const listeners = this.listeners.get(type) ?? new Set<EventListener>();
    listeners.add(listener);
    this.listeners.set(type, listeners);
  }

  removeEventListener(type: string, listener: EventListener): void {
    this.listeners.get(type)?.delete(listener);
  }

  emit(type: string, data = ""): void {
    const event = type === "error" ? new Event(type) : new MessageEvent(type, { data });
    for (const listener of this.listeners.get(type) ?? []) listener(event);
  }

  close(): void {
    this.closed = true;
  }
}

const network: TaskRunNetworkProjection = {
  conversation: {
    workspace_id: "ws-1",
    channel: "coord-run-1",
    surface: "thread",
    thread_id: "thread_agent_channel",
    stream_url: "/api/task-runs/run-1/conversation/stream",
  },
  usage: {
    workspace_id: "ws-1",
    details: [],
    total: {
      wake_count: 0,
      reserved_wake_count: 0,
      actual_wake_count: 0,
      unavailable_wake_count: 0,
      charged_wall_time: "0s",
      input_tokens: 0,
      output_tokens: 0,
    },
  },
};

/**
 * The shared stream transport asks the listener whether it needs a ticket before
 * it opens a socket. A local listener answers 404, which is what this lane is.
 */
function stubLocalListener() {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: "not found" }), {
        status: 404,
        headers: { "Content-Type": "application/json" },
      })
    )
  );
}

async function openedSource(index = 0): Promise<FakeEventSource> {
  await waitFor(() => expect(FakeEventSource.instances.length).toBeGreaterThan(index));
  return FakeEventSource.instances[index];
}

describe("useTaskRunConversation", () => {
  beforeEach(() => {
    resetGatewayStreamAuth();
    FakeEventSource.instance = null;
    FakeEventSource.instances = [];
    vi.stubGlobal("EventSource", FakeEventSource);
    stubLocalListener();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    resetGatewayStreamAuth();
  });

  it("Should refresh messages and apply truthful usage from run SSE frames", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const { result, unmount } = renderHook(() => useTaskRunConversation("run-1", network), {
      wrapper,
    });
    const source = await openedSource();
    expect(source.url).toBe("/api/task-runs/run-1/conversation/stream");

    act(() => {
      source.emit("network.message", JSON.stringify({ message: { message_id: "msg-1" } }));
      source.emit(
        "network.usage",
        JSON.stringify({
          usage: {
            ...network.usage,
            total: { ...network.usage.total, wake_count: 1, actual_wake_count: 1 },
          },
        })
      );
    });

    await waitFor(() => expect(result.current?.usage.total.wake_count).toBe(1));
    expect(invalidate).toHaveBeenCalledTimes(1);
    unmount();
    expect(source.closed).toBe(true);
  });

  it("Should disclose reconnect state while EventSource retries", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useTaskRunConversation("run-1", network), { wrapper });
    const source = await openedSource();

    act(() => source.emit("error"));

    await waitFor(() => expect(result.current?.streamError?.message).toMatch(/reconnecting/i));
  });

  it("Should ignore a late usage frame from another workspace run", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const nextNetwork: TaskRunNetworkProjection = {
      ...network,
      conversation: {
        ...network.conversation,
        workspace_id: "ws-2",
        channel: "coord-run-2",
        stream_url: "/api/task-runs/run-2/conversation/stream",
      },
      usage: {
        ...network.usage,
        workspace_id: "ws-2",
        total: { ...network.usage.total, wake_count: 2, actual_wake_count: 2 },
      },
    };
    const { result, rerender } = renderHook(
      ({ runId, projection }) => useTaskRunConversation(runId, projection),
      { initialProps: { runId: "run-1", projection: network }, wrapper }
    );
    const previousSource = await openedSource();
    const lateUsageListener = [...(previousSource.listeners.get("network.usage") ?? [])][0];

    rerender({ runId: "run-2", projection: nextNetwork });
    act(() => {
      lateUsageListener?.(
        new MessageEvent("network.usage", {
          data: JSON.stringify({
            usage: {
              ...network.usage,
              total: { ...network.usage.total, wake_count: 99, actual_wake_count: 99 },
            },
          }),
        })
      );
    });

    await waitFor(() => expect(result.current?.usage.workspace_id).toBe("ws-2"));
    expect(result.current?.usage.total.wake_count).toBe(2);
  });
});
