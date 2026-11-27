import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ComputerScopeContext } from "@/lib/computer-scope-context";
import { filesKeys } from "@/systems/files";
import { workspaceKeys } from "@/systems/workspace";

import { sessionKeys } from "../../lib/query-keys";
import { sessionChangesKeys } from "../../lib/session-changes-query";
import { primarySessionFixture } from "../../mocks/fixtures";
import { useChangeSessionWorktree } from "../use-change-session-worktree";
import { useForkSessionToWorktree } from "../use-fork-session-to-worktree";
import { useSessionEnvironmentControl } from "../use-session-environment-control";
import {
  requestSessionReviewChanges,
  useSessionReviewPanelState,
} from "../use-session-review-panel-state";

// Suite: changing a session's worktree in place (file-explorer Greptile P1).
// Invariant: once the daemon moves the session to another checkout, the Review panel drops the
// old checkout's files and Changes and its focused change, instead of showing them as current.
// Owning layer: useChangeSessionWorktree. Boundary: `fetch`.
const WORKSPACE_ID = primarySessionFixture.workspace_id ?? "ws_primary";
const SESSION_ID = primarySessionFixture.id;

describe("useChangeSessionWorktree", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("Should drop the old checkout's files and Changes once the worktree changes", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => Response.json({ ...primarySessionFixture, worktree_id: "wt_new" }))
    );
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const root = { kind: "session" as const, workspaceId: WORKSPACE_ID, sessionId: SESSION_ID };
    const entriesKey = filesKeys.entries(root, "", true);
    const changesKey = sessionChangesKeys.summary({
      workspaceId: WORKSPACE_ID,
      sessionId: SESSION_ID,
    });
    queryClient.setQueryData(entriesKey, { from: "old-checkout" });
    queryClient.setQueryData(changesKey, { from: "old-checkout" });
    requestSessionReviewChanges(SESSION_ID, { path: "a.ts" });
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(
      () => ({
        change: useChangeSessionWorktree(WORKSPACE_ID, SESSION_ID),
        review: useSessionReviewPanelState(SESSION_ID),
      }),
      { wrapper }
    );
    const rootKey = result.current.review.rootKey;

    await act(() =>
      result.current.change.mutateAsync({ worktree: "wt_new", expectedWorktreeId: "wt_old" })
    );

    expect(queryClient.getQueryData(entriesKey)).toBeUndefined();
    expect(queryClient.getQueryData(changesKey)).toBeUndefined();
    expect(result.current.review.focusedChange).toBeNull();
    expect(result.current.review.rootKey).toBe(rootKey + 1);
  });

  // Regression test for AST-008: worktree bind/unbind/fork/create mutations inside a remote
  // session window must target `/c/{computerId}` and invalidate only that computer's query keys.
  it("AST-008: Should target the owning remote computer and scope cache updates for bind and root reset", async () => {
    const remoteComputerId = "cmp_remote_1";
    const requestedUrls: Array<{ method: string; url: string }> = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = input instanceof Request ? input.url : String(input);
        const method = init?.method ?? (input instanceof Request ? input.method : "GET");
        requestedUrls.push({ method, url });
        return Response.json({
          session: { ...primarySessionFixture, worktree_id: "wt_remote_new" },
        });
      })
    );

    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    const localRoot = {
      kind: "session" as const,
      workspaceId: WORKSPACE_ID,
      sessionId: SESSION_ID,
    };
    const remoteRoot = { ...localRoot, computerId: remoteComputerId };
    const localEntriesKey = filesKeys.entries(localRoot, "", true);
    const remoteEntriesKey = filesKeys.entries(remoteRoot, "", true);
    const localChangesKey = sessionChangesKeys.summary({
      workspaceId: WORKSPACE_ID,
      sessionId: SESSION_ID,
    });
    const remoteChangesKey = sessionChangesKeys.forComputer(remoteComputerId).summary({
      workspaceId: WORKSPACE_ID,
      sessionId: SESSION_ID,
      computerId: remoteComputerId,
    });
    queryClient.setQueryData(localEntriesKey, { from: "local-checkout" });
    queryClient.setQueryData(remoteEntriesKey, { from: "remote-old-checkout" });
    queryClient.setQueryData(localChangesKey, { from: "local-changes" });
    queryClient.setQueryData(remoteChangesKey, { from: "remote-old-changes" });

    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>
        <ComputerScopeContext value={remoteComputerId}>{children}</ComputerScopeContext>
      </QueryClientProvider>
    );

    const { result } = renderHook(() => useChangeSessionWorktree(WORKSPACE_ID, SESSION_ID), {
      wrapper,
    });

    await act(() =>
      result.current.mutateAsync({ worktree: "wt_remote_new", expectedWorktreeId: "wt_remote_old" })
    );

    expect(requestedUrls).toEqual([
      {
        method: "PUT",
        url: expect.stringContaining(
          `/c/${remoteComputerId}/api/workspaces/${WORKSPACE_ID}/sessions/${SESSION_ID}/worktree`
        ),
      },
    ]);
    // Remote root reads are dropped; local computer caches stay untouched.
    expect(queryClient.getQueryData(remoteEntriesKey)).toBeUndefined();
    expect(queryClient.getQueryData(remoteChangesKey)).toBeUndefined();
    expect(queryClient.getQueryData(localEntriesKey)).toEqual({ from: "local-checkout" });
    expect(queryClient.getQueryData(localChangesKey)).toEqual({ from: "local-changes" });

    const remoteSessionKeys = sessionKeys.forComputer(remoteComputerId);
    const remoteWorkspaceKeys = workspaceKeys.forComputer(remoteComputerId);
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: remoteSessionKeys.detail(WORKSPACE_ID, SESSION_ID),
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: remoteSessionKeys.byIdRoot(SESSION_ID),
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: remoteSessionKeys.commands(WORKSPACE_ID, SESSION_ID),
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: remoteWorkspaceKeys.worktrees(WORKSPACE_ID),
    });
    expect(invalidateSpy).not.toHaveBeenCalledWith({
      queryKey: sessionKeys.detail(WORKSPACE_ID, SESSION_ID),
    });

    // Clearing the worktree back to workspace root also targets `/c/{computerId}`.
    await act(() =>
      result.current.mutateAsync({ worktree: "", expectedWorktreeId: "wt_remote_new" })
    );
    expect(requestedUrls[1]).toEqual({
      method: "DELETE",
      url: expect.stringContaining(
        `/c/${remoteComputerId}/api/workspaces/${WORKSPACE_ID}/sessions/${SESSION_ID}/worktree?expected_worktree_id=wt_remote_new`
      ),
    });
  });

  // Regression test for AST-008: forking and creating worktrees from the session environment
  // control must route through `/c/{computerId}` and invalidate computer-scoped lists.
  it("AST-008: Should route fork and environment-control worktree creation to the owning remote computer", async () => {
    const remoteComputerId = "cmp_remote_2";
    const requestedUrls: Array<{ method: string; url: string }> = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = input instanceof Request ? input.url : String(input);
        const method = init?.method ?? (input instanceof Request ? input.method : "GET");
        requestedUrls.push({ method, url });
        if (url.includes("/api/profiles")) {
          return Response.json({ profiles: [], active_profile: "default" });
        }
        if (url.endsWith("/worktree-fork")) {
          return Response.json({
            session: { ...primarySessionFixture, id: "sess_forked", worktree_id: "wt_target" },
          });
        }
        return Response.json(
          {
            worktree: {
              id: "wt_created_remote",
              workspace_id: WORKSPACE_ID,
              name: "remote-wt",
              branch: "remote-wt",
              path: "/remote/wt",
              state: "pending",
              pending_phase: "branch",
            },
          },
          { status: 202 }
        );
      })
    );

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>
        <ComputerScopeContext value={remoteComputerId}>{children}</ComputerScopeContext>
      </QueryClientProvider>
    );

    const forkView = renderHook(() => useForkSessionToWorktree(WORKSPACE_ID, SESSION_ID), {
      wrapper,
    });
    await act(() =>
      forkView.result.current.mutateAsync({ confirmed: true, worktree: "wt_target" })
    );

    expect(requestedUrls[0]).toEqual({
      method: "POST",
      url: expect.stringContaining(
        `/c/${remoteComputerId}/api/workspaces/${WORKSPACE_ID}/sessions/${SESSION_ID}/worktree-fork`
      ),
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: sessionKeys.forComputer(remoteComputerId).lists(),
    });
    expect(invalidateSpy).not.toHaveBeenCalledWith({
      queryKey: sessionKeys.lists(),
    });

    const controlView = renderHook(
      () =>
        useSessionEnvironmentControl({
          computerId: remoteComputerId,
          sessionId: SESSION_ID,
          workspaceId: WORKSPACE_ID,
          currentWorktreeId: "",
        }),
      { wrapper }
    );

    act(() => {
      controlView.result.current.startNewWorktree();
    });

    await waitFor(() => {
      expect(
        requestedUrls.some(
          req =>
            req.method === "POST" &&
            req.url.includes(`/c/${remoteComputerId}/api/workspaces/${WORKSPACE_ID}/worktrees`)
        )
      ).toBe(true);
    });
  });
});
