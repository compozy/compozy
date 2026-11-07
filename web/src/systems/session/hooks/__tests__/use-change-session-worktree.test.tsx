import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { fileViewerTarget, filesKeys } from "@/systems/files";

import { primarySessionFixture } from "../../mocks/fixtures";
import { sessionChangesKeys } from "../../lib/session-changes-query";
import { useChangeSessionWorktree } from "../use-change-session-worktree";
import {
  requestSessionReviewFile,
  useSessionReviewPanelState,
} from "../use-session-review-panel-state";

// Suite: changing a session's worktree in place (file-explorer Greptile P1).
// Invariant: once the daemon moves the session to another checkout, the Review panel drops the
// old checkout's files and Changes and its viewer target, instead of showing them as current.
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
    requestSessionReviewFile(SESSION_ID, fileViewerTarget("a.ts"));
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
    expect(result.current.review.target).toBeNull();
    expect(result.current.review.rootKey).toBe(rootKey + 1);
  });
});
