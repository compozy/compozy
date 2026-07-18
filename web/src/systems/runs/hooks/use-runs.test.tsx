import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { dashboardKeys } from "@/systems/dashboard";
import { workflowKeys } from "@/systems/workflows/lib/query-keys";
import { createTestQueryClient, installFetchStub, matchPath, withQuery } from "@/test/utils";

import { runKeys } from "../lib/query-keys";
import { dispatchedRunFixture } from "../mocks/fixtures";
import { useStartWorkflowRun } from "./use-runs";

describe("useStartWorkflowRun", () => {
  it("Should invalidate the workflow inventory, run list, and dashboard after a successful start", async () => {
    const { workspace_id: workspaceId, workflow_slug: slug, run_id: runId } = dispatchedRunFixture;
    if (!workspaceId || !slug || !runId) {
      throw new Error("dispatchedRunFixture must define workspace_id, workflow_slug, and run_id");
    }
    const stub = installFetchStub([
      {
        status: 200,
        body: { run: dispatchedRunFixture },
        matcher: matchPath(`/api/tasks/${slug}/runs`, "POST"),
      },
    ]);
    const queryClient = createTestQueryClient();
    // Spy passes through to the real implementation, so the cache is genuinely
    // invalidated while we can still assert exactly which scopes were refreshed.
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    try {
      const { result } = renderHook(() => useStartWorkflowRun(), {
        wrapper: withQuery(queryClient),
      });

      let startedRunId: string | undefined;
      await act(async () => {
        const run = await result.current.mutateAsync({
          workspaceId,
          slug,
          body: { presentation_mode: "detach" },
        });
        startedRunId = run.run_id;
      });

      // The real start path ran end to end through the stubbed network boundary.
      expect(startedRunId).toBe(runId);
      // Regression guard for issue_004: the inventory list gates the Start
      // button (active_runs, readiness badge, can_start_run) and must be
      // invalidated so the button does not re-enable against a stale, pre-run
      // projection that lets the user dispatch a duplicate run.
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: workflowKeys.list(workspaceId) });
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: runKeys.lists() });
      expect(invalidateSpy).toHaveBeenCalledWith({
        queryKey: dashboardKeys.byWorkspace(workspaceId),
      });
    } finally {
      stub.restore();
    }
  });
});
