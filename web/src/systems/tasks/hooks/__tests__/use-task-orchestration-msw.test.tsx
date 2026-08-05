import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { getResponse } from "msw";
import { createElement, type ReactNode } from "react";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { handlers } from "@/systems/tasks/mocks/handlers";

import {
  useTaskBridgeNotificationSubscriptions,
  useTaskContextBundle,
  useTaskExecutionProfile,
  useTaskRunReviews,
} from "@/systems/tasks";

const originalFetch = globalThis.fetch;

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

beforeAll(() => {
  const baseUrl = typeof window === "undefined" ? "http://localhost" : window.location.origin;
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    const request = input instanceof Request ? input.clone() : new Request(input, init);
    const response = await getResponse(handlers, request, { baseUrl });
    if (!response) {
      throw new Error(`No MSW handler matched: ${request.method} ${request.url}`);
    }
    return response;
  }) as typeof globalThis.fetch;
});

afterAll(() => {
  globalThis.fetch = originalFetch;
});

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  vi.restoreAllMocks();
});

function testApiUrl(path: string): string {
  const baseUrl = typeof window === "undefined" ? "http://localhost" : window.location.origin;
  return new URL(path, baseUrl).toString();
}

describe("orchestration hooks against MSW handlers", () => {
  const agentIdentity = { agentName: "worker", sessionId: "session_001" };

  it("Should expose execution profile via real adapter stack", async () => {
    const { result } = renderHook(() => useTaskExecutionProfile("task_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.data?.task_id).toBe("task_001");
    });
  });

  it("Should expose task context bundle via /api/agent/context handler", async () => {
    const { result } = renderHook(() => useTaskContextBundle(agentIdentity), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.data?.task.id).toBe("task_001");
      expect(result.current.data?.latest_event_seq).toBeGreaterThanOrEqual(0);
    });
  });

  it("Should list run reviews from MSW", async () => {
    const { result } = renderHook(() => useTaskRunReviews("run_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(Array.isArray(result.current.data)).toBe(true);
      expect((result.current.data ?? []).length).toBeGreaterThan(0);
    });
  });

  it("Should list bridge notification subscriptions with cursor diagnostics", async () => {
    const { result } = renderHook(() => useTaskBridgeNotificationSubscriptions("task_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.data).toBeDefined();
      const list = result.current.data ?? [];
      expect(list.length).toBeGreaterThan(0);
      const cursor = list[0]?.cursor;
      expect(cursor?.consumer_id).toBe(list[0]?.subscription_id);
      expect(cursor?.stream_name).toBe("task_events");
    });
  });

  it("Should create bridge notification cursor identity from the exact subscription id", async () => {
    const request = {
      bridge_instance_id: " bridge_instance_alpha ",
      delivery_mode: "reply",
      scope: "workspace",
      workspace_id: " ws_alpha ",
      peer_id: " peer_alpha ",
      group_id: " group_alpha ",
      thread_id: " thread_alpha ",
      subscription_id: " bsub_exact ",
    } as const;
    const response = await fetch(testApiUrl("/api/tasks/task_001/notifications/bridges"), {
      body: JSON.stringify(request),
      headers: { "Content-Type": "application/json" },
      method: "POST",
    });
    const payload = (await response.json()) as {
      subscription: {
        bridge_instance_id: string;
        cursor: { consumer_id: string };
        group_id?: string;
        peer_id?: string;
        subscription_id: string;
        thread_id?: string;
        workspace_id?: string;
      };
    };

    expect(response.status).toBe(201);
    expect(payload.subscription).toMatchObject({
      bridge_instance_id: request.bridge_instance_id,
      group_id: request.group_id,
      peer_id: request.peer_id,
      subscription_id: request.subscription_id,
      thread_id: request.thread_id,
      workspace_id: request.workspace_id,
    });
    expect(payload.subscription.cursor.consumer_id).toBe(payload.subscription.subscription_id);
  });

  it("Should reject recover in MSW when a task is not escalated", async () => {
    const response = await fetch(testApiUrl("/api/tasks/task_recover_ready/recover"), {
      method: "POST",
    });
    const body = await response.json();

    expect(response.status).toBe(409);
    expect(body.error).toMatch(/not in needs_attention/i);
  });

  it("Should recover escalated tasks in MSW", async () => {
    const response = await fetch(testApiUrl("/api/tasks/task_recoverable/recover"), {
      method: "POST",
    });
    const body = await response.json();

    expect(response.status).toBe(200);
    expect(body.task.status).toBe("ready");
    expect(body.task.needs_attention).toBe(false);
  });
});
