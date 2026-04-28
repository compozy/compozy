import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { createTestQueryClient, withQuery } from "@/test/utils";
import { dashboardKeys } from "@/systems/dashboard";
import { memoryKeys } from "@/systems/memory";
import { reviewKeys } from "@/systems/reviews";
import { runKeys } from "@/systems/runs";
import { specKeys } from "@/systems/spec";
import { workflowKeys } from "@/systems/workflows";

import {
  invalidateWorkspaceEvent,
  invalidateWorkspaceScope,
  useWorkspaceEvents,
} from "./use-workspace-events";
import type {
  OpenWorkspaceEventStreamOptions,
  WorkspaceEventController,
  WorkspaceEventHandler,
  WorkspaceEventPayload,
  WorkspaceEventStreamFactory,
} from "../lib/workspace-events";

interface FakeWorkspaceEventController extends WorkspaceEventController {
  emit: WorkspaceEventHandler;
  closed: boolean;
  options: OpenWorkspaceEventStreamOptions;
}

function createWorkspaceStreamHarness() {
  const controllers: FakeWorkspaceEventController[] = [];
  const factory: WorkspaceEventStreamFactory = (options, handler) => {
    const controller: FakeWorkspaceEventController = {
      emit: handler,
      closed: false,
      options,
      close() {
        controller.closed = true;
      },
    };
    controllers.push(controller);
    return controller;
  };
  return { factory, controllers };
}

function workspaceEvent(overrides: Partial<WorkspaceEventPayload> = {}): WorkspaceEventPayload {
  return {
    seq: 1,
    ts: "2026-04-28T12:00:00Z",
    workspace_id: "workspace-1",
    workflow_slug: "demo",
    run_id: "run-1",
    mode: "task",
    status: "running",
    kind: "run.status_changed",
    ...overrides,
  };
}

describe("useWorkspaceEvents", () => {
  it("Should open one workspace stream and invalidate run queries from workspace events", async () => {
    const queryClient = createTestQueryClient();
    const harness = createWorkspaceStreamHarness();
    const dashboardKey = dashboardKeys.byWorkspace("workspace-1");
    const runListKey = runKeys.list({ workspaceId: "workspace-1", status: "active" });
    const runSummaryKey = runKeys.run("run-1");
    const runSnapshotKey = runKeys.snapshot("run-1");
    for (const key of [dashboardKey, runListKey, runSummaryKey, runSnapshotKey]) {
      queryClient.setQueryData(key, { ok: true });
    }

    const { unmount } = renderHook(
      () => useWorkspaceEvents({ workspaceId: "workspace-1", factory: harness.factory }),
      { wrapper: withQuery(queryClient) }
    );

    expect(harness.controllers).toHaveLength(1);
    expect(harness.controllers[0]!.options.workspaceId).toBe("workspace-1");

    act(() => {
      harness.controllers[0]!.emit({ type: "event", eventId: "1", payload: workspaceEvent() });
    });

    await waitFor(() => {
      expect(queryClient.getQueryState(dashboardKey)?.isInvalidated).toBe(true);
      expect(queryClient.getQueryState(runListKey)?.isInvalidated).toBe(true);
      expect(queryClient.getQueryState(runSummaryKey)?.isInvalidated).toBe(true);
      expect(queryClient.getQueryState(runSnapshotKey)?.isInvalidated).toBe(true);
    });

    unmount();
    expect(harness.controllers[0]!.closed).toBe(true);
  });

  it("Should invalidate workflow artifact families from artifact events", async () => {
    const queryClient = createTestQueryClient();
    const dashboardKey = dashboardKeys.byWorkspace("workspace-1");
    const workflowListKey = workflowKeys.list("workspace-1");
    const boardKey = workflowKeys.board("workspace-1", "demo");
    const tasksKey = workflowKeys.tasks("workspace-1", "demo");
    const specKey = specKeys.workflow("workspace-1", "demo");
    const memoryIndexKey = memoryKeys.index("workspace-1", "demo");
    const reviewSummaryKey = reviewKeys.summary("workspace-1", "demo");
    const reviewIssuesKey = reviewKeys.issues("workspace-1", "demo", 1);
    for (const key of [
      dashboardKey,
      workflowListKey,
      boardKey,
      tasksKey,
      specKey,
      memoryIndexKey,
      reviewSummaryKey,
      reviewIssuesKey,
    ]) {
      queryClient.setQueryData(key, { ok: true });
    }

    invalidateWorkspaceEvent(
      queryClient,
      "workspace-1",
      workspaceEvent({
        kind: "artifact.changed",
        paths: ["_techspec.md", "memory/MEMORY.md", "reviews-001/issue_001.md"],
      })
    );

    await waitFor(() => {
      expect(queryClient.getQueryState(dashboardKey)?.isInvalidated).toBe(true);
      expect(queryClient.getQueryState(workflowListKey)?.isInvalidated).toBe(true);
      expect(queryClient.getQueryState(boardKey)?.isInvalidated).toBe(true);
      expect(queryClient.getQueryState(tasksKey)?.isInvalidated).toBe(true);
      expect(queryClient.getQueryState(specKey)?.isInvalidated).toBe(true);
      expect(queryClient.getQueryState(memoryIndexKey)?.isInvalidated).toBe(true);
      expect(queryClient.getQueryState(reviewSummaryKey)?.isInvalidated).toBe(true);
      expect(queryClient.getQueryState(reviewIssuesKey)?.isInvalidated).toBe(true);
    });
  });

  it("Should invalidate the full workspace scope after overflow", async () => {
    const queryClient = createTestQueryClient();
    const dashboardKey = dashboardKeys.byWorkspace("workspace-1");
    const workflowListKey = workflowKeys.list("workspace-1");
    const runListKey = runKeys.list({ workspaceId: "workspace-1" });
    for (const key of [dashboardKey, workflowListKey, runListKey]) {
      queryClient.setQueryData(key, { ok: true });
    }

    invalidateWorkspaceScope(queryClient, "workspace-1");

    await waitFor(() => {
      expect(queryClient.getQueryState(dashboardKey)?.isInvalidated).toBe(true);
      expect(queryClient.getQueryState(workflowListKey)?.isInvalidated).toBe(true);
      expect(queryClient.getQueryState(runListKey)?.isInvalidated).toBe(true);
    });
  });
});
