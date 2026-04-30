import { useEffect } from "react";

import { useQueryClient, type QueryClient } from "@tanstack/react-query";

import { dashboardKeys } from "@/systems/dashboard";
import { memoryKeys } from "@/systems/memory";
import { reviewKeys } from "@/systems/reviews";
import { runKeys } from "@/systems/runs";
import { specKeys } from "@/systems/spec";
import { workflowKeys } from "@/systems/workflows";

import {
  defaultWorkspaceEventStreamFactory,
  type WorkspaceEventPayload,
  type WorkspaceEventStreamFactory,
} from "../lib/workspace-events";

export interface UseWorkspaceEventsOptions {
  workspaceId: string | null;
  enabled?: boolean;
  factory?: WorkspaceEventStreamFactory;
  baseUrl?: string;
}

export function useWorkspaceEvents(options: UseWorkspaceEventsOptions): void {
  const {
    workspaceId,
    enabled = true,
    factory = defaultWorkspaceEventStreamFactory,
    baseUrl,
  } = options;
  const queryClient = useQueryClient();

  useEffect(() => {
    if (!enabled || !workspaceId) {
      return;
    }

    const controller = factory({ workspaceId, baseUrl }, signal => {
      switch (signal.type) {
        case "event":
          invalidateWorkspaceEvent(queryClient, workspaceId, signal.payload);
          return;
        case "overflow":
          invalidateWorkspaceScope(queryClient, workspaceId);
          return;
        default:
          return;
      }
    });

    return () => controller.close();
  }, [baseUrl, enabled, factory, queryClient, workspaceId]);
}

export function invalidateWorkspaceEvent(
  queryClient: QueryClient,
  workspaceId: string,
  event: WorkspaceEventPayload
): void {
  if (event.workspace_id !== workspaceId) {
    return;
  }

  switch (event.kind) {
    case "run.created":
    case "run.status_changed":
    case "run.terminal":
      invalidateRunQueries(queryClient, workspaceId, event.run_id);
      return;
    case "workflow.sync_completed":
      invalidateWorkflowQueries(queryClient, workspaceId, event.workflow_slug, {
        allArtifacts: true,
      });
      return;
    case "artifact.changed":
      invalidateWorkflowQueries(queryClient, workspaceId, event.workflow_slug, {
        paths: event.paths ?? [],
      });
      return;
  }
}

export function invalidateWorkspaceScope(queryClient: QueryClient, workspaceId: string): void {
  invalidateRunQueries(queryClient, workspaceId, null);
  invalidateWorkflowQueries(queryClient, workspaceId, null, { allArtifacts: true });
}

function invalidateRunQueries(
  queryClient: QueryClient,
  workspaceId: string,
  runId?: string | null
) {
  void queryClient.invalidateQueries({ queryKey: dashboardKeys.byWorkspace(workspaceId) });
  void queryClient.invalidateQueries({ queryKey: runKeys.lists() });
  if (runId) {
    void queryClient.invalidateQueries({ queryKey: runKeys.run(runId) });
    void queryClient.invalidateQueries({ queryKey: runKeys.snapshot(runId) });
  }
}

interface WorkflowInvalidationOptions {
  allArtifacts?: boolean;
  paths?: string[];
}

function invalidateWorkflowQueries(
  queryClient: QueryClient,
  workspaceId: string,
  workflowSlug: string | null | undefined,
  options: WorkflowInvalidationOptions
): void {
  void queryClient.invalidateQueries({ queryKey: dashboardKeys.byWorkspace(workspaceId) });
  void queryClient.invalidateQueries({ queryKey: workflowKeys.list(workspaceId) });

  if (!workflowSlug) {
    void queryClient.invalidateQueries({ queryKey: workflowKeys.workflows() });
    void queryClient.invalidateQueries({ queryKey: reviewKeys.all });
    void queryClient.invalidateQueries({ queryKey: specKeys.all });
    void queryClient.invalidateQueries({ queryKey: memoryKeys.all });
    return;
  }

  void queryClient.invalidateQueries({ queryKey: workflowKeys.board(workspaceId, workflowSlug) });
  void queryClient.invalidateQueries({ queryKey: workflowKeys.tasks(workspaceId, workflowSlug) });

  if (options.allArtifacts || shouldInvalidateSpec(options.paths)) {
    void queryClient.invalidateQueries({ queryKey: specKeys.workflow(workspaceId, workflowSlug) });
  }
  if (options.allArtifacts || shouldInvalidateMemory(options.paths)) {
    void queryClient.invalidateQueries({ queryKey: memoryKeys.index(workspaceId, workflowSlug) });
    void queryClient.invalidateQueries({ queryKey: memoryKeys.files() });
  }
  if (options.allArtifacts || shouldInvalidateReviews(options.paths)) {
    void queryClient.invalidateQueries({ queryKey: reviewKeys.summary(workspaceId, workflowSlug) });
    void queryClient.invalidateQueries({ queryKey: reviewKeys.rounds() });
  }
}

function shouldInvalidateSpec(paths: string[] | undefined): boolean {
  return (paths ?? []).some(path => {
    return path === "_prd.md" || path === "_techspec.md" || path.startsWith("adrs/");
  });
}

function shouldInvalidateMemory(paths: string[] | undefined): boolean {
  return (paths ?? []).some(path => path.startsWith("memory/"));
}

function shouldInvalidateReviews(paths: string[] | undefined): boolean {
  return (paths ?? []).some(path => path.startsWith("reviews-"));
}
