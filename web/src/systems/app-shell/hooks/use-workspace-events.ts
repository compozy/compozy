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

interface WorkflowQueryReference {
  workflowSlug: string;
  taskGroupId?: string;
}

const taskGroupReferencePattern = /^([^/]+)\/(TG-[0-9]{3})$/;

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

  const reference = parseWorkflowQueryReference(workflowSlug);
  const queryScope = reference.taskGroupId
    ? [workspaceId, reference.workflowSlug, reference.taskGroupId]
    : [workspaceId, reference.workflowSlug];

  // An initiative event stops before the task-group slot and invalidates every child.
  // A composite initiative/TG-NNN event includes that slot and invalidates only its child.
  void queryClient.invalidateQueries({
    queryKey: [...workflowKeys.workflows(), ...queryScope],
  });

  if (options.allArtifacts || shouldInvalidateSpec(options.paths)) {
    void queryClient.invalidateQueries({ queryKey: [...specKeys.all, ...queryScope] });
  }
  if (options.allArtifacts || shouldInvalidateMemory(options.paths)) {
    void queryClient.invalidateQueries({
      queryKey: [...memoryKeys.indexes(), ...queryScope],
    });
    void queryClient.invalidateQueries({ queryKey: [...memoryKeys.files(), ...queryScope] });
  }
  if (options.allArtifacts || shouldInvalidateReviews(options.paths)) {
    void queryClient.invalidateQueries({
      queryKey: [...reviewKeys.summaries(), ...queryScope],
    });
    void queryClient.invalidateQueries({ queryKey: [...reviewKeys.rounds(), ...queryScope] });
  }
}

function parseWorkflowQueryReference(reference: string): WorkflowQueryReference {
  const match = taskGroupReferencePattern.exec(reference);
  if (!match) {
    return { workflowSlug: reference };
  }
  const workflowSlug = match[1];
  const taskGroupId = match[2];
  if (!workflowSlug || !taskGroupId) {
    return { workflowSlug: reference };
  }
  return { workflowSlug, taskGroupId };
}

function shouldInvalidateSpec(paths: string[] | undefined): boolean {
  return (paths ?? []).some(path => {
    return (
      path === "_prd.md" ||
      path === "_techspec.md" ||
      path === "_task_groups.md" ||
      path.startsWith("adrs/")
    );
  });
}

function shouldInvalidateMemory(paths: string[] | undefined): boolean {
  return (paths ?? []).some(path => path.startsWith("memory/"));
}

function shouldInvalidateReviews(paths: string[] | undefined): boolean {
  return (paths ?? []).some(path => path.startsWith("reviews-"));
}
