// Suite: Session runtime selection mutation
// Invariant: the lifecycle-owned abort signal reaches the HTTP adapter unchanged.
// Boundary IN: TanStack mutation hook. Boundary OUT: HTTP behavior, owned by the adapter suite.

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

const sessionApiMocks = vi.hoisted(() => ({
  setSessionRuntime: vi.fn(),
}));

vi.mock("../../adapters/session-api", () => ({
  SessionApiError: class SessionApiError extends Error {
    status = 500;
  },
  setSessionRuntime: sessionApiMocks.setSessionRuntime,
}));

import { primarySessionFixture } from "../../mocks";
import { useSetSessionRuntime } from "../use-session-runtime-selection";

function createWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  return function Wrapper({ children }: { children: ReactNode }) {
    return createElement(QueryClientProvider, { client: queryClient }, children);
  };
}

describe("useSetSessionRuntime", () => {
  it("Should forward the caller-owned cancellation signal", async () => {
    sessionApiMocks.setSessionRuntime.mockResolvedValueOnce(primarySessionFixture);
    const workspaceId = primarySessionFixture.workspace_id;
    if (!workspaceId) throw new Error("test fixture workspace_id is required");
    const controller = new AbortController();
    const { result } = renderHook(() => useSetSessionRuntime(workspaceId), {
      wrapper: createWrapper(),
    });

    act(() => {
      result.current.mutate({
        id: primarySessionFixture.id,
        request: {
          expected_revision: primarySessionFixture.runtime.selection_revision,
          runtime: { provider: "codex" },
        },
        signal: controller.signal,
      });
    });

    await waitFor(() => expect(sessionApiMocks.setSessionRuntime).toHaveBeenCalledOnce());
    expect(sessionApiMocks.setSessionRuntime).toHaveBeenCalledWith(
      workspaceId,
      primarySessionFixture.id,
      expect.objectContaining({ expected_revision: 0 }),
      controller.signal
    );
  });
});
