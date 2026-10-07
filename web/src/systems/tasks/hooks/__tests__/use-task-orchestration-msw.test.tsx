import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { getResponse } from "msw";
import { createElement, type ReactNode } from "react";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { handlers } from "@/systems/tasks/mocks/handlers";

import { useTaskContextBundle, useTaskExecutionProfile, useTaskRunReviews } from "@/systems/tasks";

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
});
