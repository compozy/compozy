import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createMswFetch } from "@/test/msw-fetch";
import { useRevokeToolApprovalGrant, useSetToolApprovalGrant } from "@/systems/tool-approvals";
import { handlers } from "@/systems/tool-approvals/mocks";
import { toolApprovalGrantFixtures } from "@/systems/tool-approvals/mocks/fixtures";

const WS = "ws_alpha";
const OTHER_WS = "ws_other";
const grantId = toolApprovalGrantFixtures[0]!.id;

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

function setup() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const invalidate = vi.spyOn(queryClient, "invalidateQueries");
  return { invalidate, wrapper: createWrapper(queryClient) };
}

describe("useSetToolApprovalGrant", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      createMswFetch(() => handlers)
    );
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("Should invalidate only the owning workspace list after a successful set", async () => {
    const { invalidate, wrapper } = setup();
    const { result } = renderHook(() => useSetToolApprovalGrant(), { wrapper });

    await result.current.mutateAsync({
      workspaceId: WS,
      request: {
        tool_id: "agh__config_set",
        decision: "allow",
        scope: "agent",
        agent_name: "claude-code",
      },
    });

    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({
        queryKey: ["tool-approval-grants", "list", WS],
      });
    });
    expect(invalidate).not.toHaveBeenCalledWith({
      queryKey: ["tool-approval-grants", "list", OTHER_WS],
    });
  });
});

describe("useRevokeToolApprovalGrant", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      createMswFetch(() => handlers)
    );
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("Should invalidate only the owning workspace list after a successful revoke", async () => {
    const { invalidate, wrapper } = setup();
    const { result } = renderHook(() => useRevokeToolApprovalGrant(), { wrapper });

    await result.current.mutateAsync({ id: grantId, workspaceId: WS });

    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({
        queryKey: ["tool-approval-grants", "list", WS],
      });
    });
    expect(invalidate).not.toHaveBeenCalledWith({
      queryKey: ["tool-approval-grants", "list", OTHER_WS],
    });
  });

  it("Should preserve the cache when a revoke fails", async () => {
    const { invalidate, wrapper } = setup();
    const { result } = renderHook(() => useRevokeToolApprovalGrant(), { wrapper });

    await expect(
      result.current.mutateAsync({ id: "missing-grant", workspaceId: WS })
    ).rejects.toThrow("Remembered decision not found");

    expect(invalidate).not.toHaveBeenCalled();
  });
});
