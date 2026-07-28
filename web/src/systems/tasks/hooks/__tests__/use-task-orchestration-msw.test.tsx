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
  it("Should expose execution profile via real adapter stack", async () => {
    const { result } = renderHook(() => useTaskExecutionProfile("task_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.data?.task_id).toBe("task_001");
    });
  });

  it("Should expose task context bundle via /api/agent/context handler", async () => {
    const { result } = renderHook(() => useTaskContextBundle(), {
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
      expect(cursor?.consumer_id).toContain("bridge_task_subscription:");
      expect(cursor?.stream_name).toBe("task_events");
    });
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
