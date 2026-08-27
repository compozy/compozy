// Suite: connected call detail
// Invariant: every control the detail view permits is actually wired to a real
// workspace-routed operation — Call again fetches the exact ask and targets the
// living child, Message child sends and shows its receipt, and neither is a
// prop the location forgot to pass.
// Owning layer: `useAgentCall`. Canonical suite: this hook test.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createMswFetch } from "@/test/msw-fetch";
import { compozyApiMock } from "@/storybook/openapi-msw";

import {
  callFixtureWorkspaceId,
  completedCallFixture,
  expiredCallFixture,
  handlers,
  invalidResultCallFixture,
  resetAgentCommsMockState,
  runningCallFixture,
  setAgentCommsMockCalls,
} from "@/systems/agent-comms/mocks";

import { useAgentCall } from "../use-agent-call";

vi.mock("@tanstack/react-router", () => ({ useNavigate: () => vi.fn() }));

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({
    runtimeWorkspaceId: callFixtureWorkspaceId,
    hasHydrated: true,
    isLoading: false,
  }),
}));

// The child session lookup and its usage own their own suites; here they only
// decide whether the counterpart still resolves.
const childSession = vi.hoisted(() => ({ isPending: false, isError: false }));
vi.mock("@/systems/session", () => ({
  useSession: () => childSession,
  useSessionUsage: () => ({ data: undefined }),
}));

vi.mock("../../../hooks/use-window-live-data-enabled", () => ({
  useWindowLiveDataEnabled: () => false,
}));

let requests: URL[] = [];
let exactPrompt: string | null = null;
let recentCallsFail = false;

function setup(callId: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return renderHook(() => useAgentCall(callId, "window:one"), { wrapper });
}

beforeEach(() => {
  requests = [];
  exactPrompt = null;
  recentCallsFail = false;
  childSession.isPending = false;
  childSession.isError = false;
  resetAgentCommsMockState();
  const mswFetch = createMswFetch(() => [
    ...(recentCallsFail
      ? [
          compozyApiMock.get("/api/workspaces/{workspace_id}/calls", ({ response }) =>
            response(503).json({ error: "call history unavailable" })
          ),
        ]
      : []),
    ...(exactPrompt === null
      ? []
      : [
          compozyApiMock.get(
            "/api/workspaces/{workspace_id}/calls/{call_id}/prompt",
            ({ params, response }) =>
              response(200).json({ call_id: String(params.call_id), prompt: exactPrompt! })
          ),
        ]),
    ...handlers,
  ]);
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

describe("useAgentCall — reading one call", () => {
  it("Should read the record through its workspace route, not a global identity route", async () => {
    const { result } = setup(completedCallFixture.call_id);
    await waitFor(() => expect(result.current.view).not.toBeNull());

    const detailRead = requests.find(url => url.pathname.endsWith(completedCallFixture.call_id));
    expect(detailRead?.pathname).toBe(
      `/api/workspaces/${callFixtureWorkspaceId}/calls/${completedCallFixture.call_id}`
    );
  });

  it("Should not issue a recent-call roster read from call-again", async () => {
    recentCallsFail = true;
    const { result } = setup(completedCallFixture.call_id);

    await waitFor(() => expect(result.current.view).not.toBeNull());

    expect(result.current.compose.recentCallsError).toBeNull();
    expect(result.current.compose.recentCalls).toEqual([]);
    expect(
      requests.some(
        url =>
          url.pathname === `/api/workspaces/${callFixtureWorkspaceId}/calls` &&
          url.searchParams.get("agent") === completedCallFixture.agent
      )
    ).toBe(false);
  });
});

describe("useAgentCall — Call again", () => {
  it("Should fetch the exact ask and prefill it, never the bounded preview", async () => {
    exactPrompt = "Review the full release packet, including the appendix and rollback matrix.";
    setAgentCommsMockCalls([
      {
        ...completedCallFixture,
        prompt_preview: "Review the full release packet…",
        prompt_bytes: new TextEncoder().encode(exactPrompt).length,
      },
    ]);
    const { result } = setup(completedCallFixture.call_id);
    await waitFor(() => expect(result.current.view).not.toBeNull());

    act(() => result.current.callAgain());

    await waitFor(() => expect(result.current.compose.prompt).not.toBe(""));
    expect(result.current.composing).toBe("call-again");
    expect(requests.some(url => url.pathname.endsWith("/prompt"))).toBe(true);
    expect(result.current.compose.prompt).toBe(exactPrompt);
    expect(result.current.compose.prompt).not.toBe("Review the full release packet…");
  });

  it("Should target the living child, so the helper revives with what it knows", async () => {
    const { result } = setup(completedCallFixture.call_id);
    await waitFor(() => expect(result.current.view).not.toBeNull());

    expect(result.current.callAgainTarget).toEqual({
      kind: "session",
      sessionId: completedCallFixture.child_session_id,
      agentName: completedCallFixture.agent,
    });
  });

  it("Should target the definition when the child expired, because there is nothing to revive", async () => {
    setAgentCommsMockCalls([expiredCallFixture]);
    const { result } = setup(expiredCallFixture.call_id);
    await waitFor(() => expect(result.current.view).not.toBeNull());

    expect(result.current.callAgainTarget).toEqual({ kind: "agent" });
  });

  it("Should refuse to send a contracted call again without a contract", async () => {
    // Only `expect_digest` survives, and a fingerprint cannot be turned back
    // into a shape. Sending anyway would downgrade a checked call in silence.
    setAgentCommsMockCalls([invalidResultCallFixture]);
    const { result } = setup(invalidResultCallFixture.call_id);
    await waitFor(() => expect(result.current.view).not.toBeNull());
    expect(result.current.view!.expectDigest).not.toBeNull();

    act(() => result.current.callAgain());
    await waitFor(() => expect(result.current.compose.prompt).not.toBe(""));
    act(() => result.current.compose.submit());

    expect(result.current.compose.accepted).toBeNull();
    expect(result.current.compose.failure?.code).toBe("call_expect_required");
    expect(
      requests.some(url => url.pathname === `/api/workspaces/${callFixtureWorkspaceId}/calls`)
    ).toBe(false);
  });

  it("Should accept the repeat once a contract is supplied", async () => {
    setAgentCommsMockCalls([invalidResultCallFixture]);
    const { result } = setup(invalidResultCallFixture.call_id);
    await waitFor(() => expect(result.current.view).not.toBeNull());

    act(() => result.current.callAgain());
    await waitFor(() => expect(result.current.compose.prompt).not.toBe(""));
    act(() => result.current.compose.setExpect('{"verdict":"ok"}'));
    act(() => result.current.compose.submit());

    await waitFor(() => expect(result.current.compose.accepted).not.toBeNull());
    expect(result.current.compose.failure).toBeNull();
  });
});

describe("useAgentCall — Message child", () => {
  it("Should send to the child and surface the daemon's receipt", async () => {
    const { result } = setup(completedCallFixture.call_id);
    await waitFor(() => expect(result.current.view).not.toBeNull());

    act(() => result.current.messageChild());
    act(() => result.current.setMessageText("check the retry path again"));
    act(() => result.current.sendMessage());

    await waitFor(() => expect(result.current.messageAccepted).not.toBeNull());
    expect(result.current.messageAccepted!.delivery).toBe("queued");
    expect(
      requests.some(
        url =>
          url.pathname === `/api/workspaces/${callFixtureWorkspaceId}/messages` && url.search !== ""
      )
    ).toBe(true);
  });

  it("Should not send an empty message", async () => {
    const { result } = setup(completedCallFixture.call_id);
    await waitFor(() => expect(result.current.view).not.toBeNull());

    act(() => result.current.messageChild());
    act(() => result.current.sendMessage());

    expect(result.current.messageAccepted).toBeNull();
    expect(result.current.messagePending).toBe(false);
  });
});

describe("useAgentCall — Cancel", () => {
  it("Should name the state a successful cancel produced", async () => {
    setAgentCommsMockCalls([runningCallFixture]);
    const { result } = setup(runningCallFixture.call_id);
    await waitFor(() => expect(result.current.view).not.toBeNull());

    act(() => result.current.cancel());

    await waitFor(() => expect(result.current.cancelOutcome).not.toBeNull());
    expect(result.current.cancelOutcome).toEqual({ state: "canceled", stale: false });
  });

  it("Should mark a cancel stale when the call had already settled another way", async () => {
    // The daemon's cancel is idempotent and answers with the real terminal
    // state, so a call that completed first comes back `completed` — the
    // operator acted on a snapshot that had moved on.
    setAgentCommsMockCalls([completedCallFixture]);
    const { result } = setup(completedCallFixture.call_id);
    await waitFor(() => expect(result.current.view).not.toBeNull());

    act(() => result.current.cancel());

    await waitFor(() => expect(result.current.cancelOutcome).not.toBeNull());
    expect(result.current.cancelOutcome).toEqual({ state: "completed", stale: true });
  });

  it("Should say nothing before a cancel has been answered", async () => {
    setAgentCommsMockCalls([runningCallFixture]);
    const { result } = setup(runningCallFixture.call_id);
    await waitFor(() => expect(result.current.view).not.toBeNull());

    expect(result.current.cancelOutcome).toBeNull();
  });
});
