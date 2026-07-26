import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createMswFetch } from "@/test/msw-fetch";
import { useToolApprovalGrantsPanel } from "@/systems/tool-approvals";
import { toolApprovalGrantFixtures } from "@/systems/tool-approvals/mocks/fixtures";

const ACTIVE_WS = "ws_active";
const OWNER_WS = "ws_owner";
// A grant owned by a DIFFERENT workspace than the one currently active.
const grant = { ...toolApprovalGrantFixtures[0]!, workspace_id: OWNER_WS };

const workspaceMock = vi.hoisted(() => ({
  value: { activeWorkspaceId: "ws_active" as string | null, hasHydrated: true, isLoading: false },
}));

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => workspaceMock.value,
}));

let deleteRequests: URL[] = [];
let putRequests: Array<{ body: unknown; url: URL }> = [];

function stubFetch() {
  deleteRequests = [];
  putRequests = [];
  vi.stubGlobal(
    "fetch",
    createMswFetch(() => [
      http.get("/api/tool-approval-grants", () => HttpResponse.json({ grants: [], total: 0 })),
      http.put("/api/tool-approval-grants", async ({ request }) => {
        putRequests.push({ body: await request.json(), url: new URL(request.url) });
        return HttpResponse.json({ grant });
      }),
      http.delete("/api/tool-approval-grants/:id", ({ request }) => {
        deleteRequests.push(new URL(request.url));
        return new HttpResponse(null, { status: 204 });
      }),
    ])
  );
}

function setup() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const invalidate = vi.spyOn(queryClient, "invalidateQueries");
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { invalidate, wrapper };
}

describe("useToolApprovalGrantsPanel revoke workspace binding", () => {
  beforeEach(() => {
    workspaceMock.value = { activeWorkspaceId: ACTIVE_WS, hasHydrated: true, isLoading: false };
    stubFetch();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("Should revoke against the grant's owner workspace, not the workspace active at confirmation", async () => {
    const { invalidate, wrapper } = setup();
    const { result } = renderHook(() => useToolApprovalGrantsPanel(), { wrapper });

    act(() => {
      result.current.revoke.open(grant);
    });
    await act(async () => {
      result.current.revoke.confirm();
    });

    await waitFor(() => expect(deleteRequests).toHaveLength(1));
    expect(deleteRequests[0]!.pathname).toBe(`/api/tool-approval-grants/${grant.id}`);
    expect(deleteRequests[0]!.searchParams.get("workspace_id")).toBe(OWNER_WS);

    await waitFor(() =>
      expect(invalidate).toHaveBeenCalledWith({
        queryKey: ["tool-approval-grants", "list", OWNER_WS],
      })
    );
    expect(invalidate).not.toHaveBeenCalledWith({
      queryKey: ["tool-approval-grants", "list", ACTIVE_WS],
    });
  });

  it("Should require an explicit valid wider scope before submission", () => {
    const { wrapper } = setup();
    const { result } = renderHook(() => useToolApprovalGrantsPanel(), { wrapper });

    act(() => {
      result.current.set.open();
    });
    expect(result.current.set.canSubmit).toBe(false);

    act(() => {
      result.current.set.change({
        toolId: "agh__config_set",
        decision: "allow",
        scope: "agent",
        agentName: "",
      });
    });
    expect(result.current.set.canSubmit).toBe(false);

    act(() => {
      result.current.set.change({
        toolId: "agh__config_set",
        decision: "allow",
        scope: "agent",
        agentName: "claude-code",
      });
    });
    expect(result.current.set.canSubmit).toBe(true);
  });

  it("Should set against the workspace captured when the dialog opened", async () => {
    const { invalidate, wrapper } = setup();
    const { result, rerender } = renderHook(() => useToolApprovalGrantsPanel(), { wrapper });

    act(() => {
      result.current.set.open();
      result.current.set.change({
        toolId: "agh__config_set",
        decision: "reject",
        scope: "tool",
        agentName: "must-not-leak",
      });
    });
    workspaceMock.value = {
      activeWorkspaceId: OWNER_WS,
      hasHydrated: true,
      isLoading: false,
    };
    rerender();

    act(() => {
      result.current.set.submit();
    });

    await waitFor(() => expect(putRequests).toHaveLength(1));
    expect(putRequests[0]!.url.searchParams.get("workspace_id")).toBe(ACTIVE_WS);
    expect(putRequests[0]!.body).toEqual({
      tool_id: "agh__config_set",
      decision: "reject",
      scope: "tool",
    });
    await waitFor(() =>
      expect(invalidate).toHaveBeenCalledWith({
        queryKey: ["tool-approval-grants", "list", ACTIVE_WS],
      })
    );
    expect(invalidate).not.toHaveBeenCalledWith({
      queryKey: ["tool-approval-grants", "list", OWNER_WS],
    });
  });
});
