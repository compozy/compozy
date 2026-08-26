// Suite: delegation attention query adapter
// Invariant: the bell's delegation causes come from the daemon's `attention`
// filter, so a cause clears on its own once someone addresses the child again —
// never from a permanent state list, and never by dismissal.
// Owning layer: `useAttentionCalls`. Canonical suite: this hook test.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createMswFetch } from "@/test/msw-fetch";

import {
  buildCallFixture,
  callFixtureWorkspaceId,
  handlers,
  invalidResultCallFixture,
  resetAgentCommsMockState,
  setAgentCommsMockCalls,
  setAgentCommsMockMessages,
} from "@/systems/agent-comms/mocks";

import { useAttentionCalls } from "../use-attention-calls";

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({
    runtimeWorkspaceId: callFixtureWorkspaceId,
    hasHydrated: true,
    isLoading: false,
  }),
}));

let requests: URL[] = [];

function setup() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return renderHook(() => useAttentionCalls([], true, false), { wrapper });
}

beforeEach(() => {
  requests = [];
  resetAgentCommsMockState();
  setAgentCommsMockMessages([]);
  const mswFetch = createMswFetch(() => handlers);
  vi.stubGlobal("fetch", ((input: RequestInfo | URL, init?: RequestInit) => {
    const raw = input instanceof Request ? input.url : String(input);
    requests.push(new URL(raw, "http://localhost"));
    return mswFetch(input, init);
  }) as typeof globalThis.fetch);
});

afterEach(() => {
  vi.unstubAllGlobals();
  resetAgentCommsMockState();
});

describe("useAttentionCalls", () => {
  it("Should ask the daemon for the unresolved subset, never a raw state list", async () => {
    setAgentCommsMockCalls([invalidResultCallFixture]);
    const { result } = setup();

    await waitFor(() => expect(result.current.count).toBe(1));

    const attentionReads = requests.filter(url => url.searchParams.get("attention") === "true");
    expect(attentionReads.length).toBeGreaterThan(0);
    // `invalid-result` is terminal and permanent: reading it by state would
    // light the bell forever, which is the bug this filter exists to prevent.
    expect(requests.some(url => url.searchParams.get("state")?.includes("invalid-result"))).toBe(
      false
    );
    // Workspace-routed, because the daemon scopes a call by the route it arrives on.
    expect(attentionReads[0]!.pathname).toBe(`/api/workspaces/${callFixtureWorkspaceId}/calls`);
  });

  it("Should clear the badge once a later call addresses the same child, with no dismissal", async () => {
    setAgentCommsMockCalls([invalidResultCallFixture]);
    const { result } = setup();
    await waitFor(() => expect(result.current.count).toBe(1));

    // Someone calls that child again — the operator retrying, or the parent
    // agent recovering on its own. Nothing dismisses anything.
    setAgentCommsMockCalls([
      invalidResultCallFixture,
      buildCallFixture({
        call_id: "call_retry",
        child_session_id: invalidResultCallFixture.child_session_id!,
        state: "running",
        created_at: "2026-08-20T19:00:00Z",
      }),
    ]);

    const { result: after } = setup();
    await waitFor(() => expect(after.current.loading).toBe(false));
    expect(after.current.count).toBe(0);
    expect(after.current.rows).toEqual([]);
  });

  it("Should load every attention page before building bell rows", async () => {
    setAgentCommsMockCalls(
      Array.from({ length: 130 }, (_, index) =>
        buildCallFixture({
          call_id: `call_attention_${index}`,
          child_session_id: `ses_child_${index}`,
          root_session_id: `ses_root_${index}`,
          state: "invalid-result",
          settled_at: "2026-08-20T18:30:00Z",
          updated_at: "2026-08-20T18:30:00Z",
        })
      )
    );
    const { result } = setup();

    await waitFor(() => expect(result.current.rows).toHaveLength(130));
    expect(result.current.count).toBe(130);
    expect(requests.filter(url => url.searchParams.get("attention") === "true").length).toBe(2);
  });
});
