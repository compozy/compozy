import { useStore, useSelector } from "@xstate/store-react";
import { useEffect } from "react";
import { useQueryClient, type QueryClient } from "@tanstack/react-query";

import { createStreamEventSource } from "@/lib/ticketed-event-source";

import { workspaceKeys } from "../lib/query-keys";
import type { WorktreeCatalogEventPayload } from "../types";
import {
  worktreeCatalogStreamLogic,
  type WorktreeCatalogStreamStatus,
} from "./worktree-catalog-stream-store";

const WORKTREE_CATALOG_CHANGED_EVENT = "worktree_catalog_changed";

export interface WorktreeCatalogEventSource {
  addEventListener: (type: string, listener: EventListenerOrEventListenerObject) => void;
  removeEventListener: (type: string, listener: EventListenerOrEventListenerObject) => void;
  close: () => void;
}

export type WorktreeCatalogEventSourceFactory = (url: string) => WorktreeCatalogEventSource;

interface UseWorktreeCatalogStreamOptions {
  enabled?: boolean;
  eventSourceFactory?: WorktreeCatalogEventSourceFactory;
}

export type { WorktreeCatalogStreamStatus } from "./worktree-catalog-stream-store";

export function worktreeCatalogStreamURL(): string {
  return "/api/worktrees/catalog-stream";
}

function defaultEventSourceFactory(url: string): WorktreeCatalogEventSource {
  return createStreamEventSource(url);
}

function parseWorktreeCatalogEvent(event: Event): WorktreeCatalogEventPayload | undefined {
  if (!(event instanceof MessageEvent) || typeof event.data !== "string") return undefined;
  try {
    const payload = JSON.parse(event.data) as Partial<WorktreeCatalogEventPayload>;
    if (
      typeof payload.kind !== "string" ||
      typeof payload.workspace_id !== "string" ||
      typeof payload.worktree_id !== "string" ||
      payload.workspace_id.trim() === "" ||
      payload.worktree_id.trim() === ""
    ) {
      return undefined;
    }
    return payload as WorktreeCatalogEventPayload;
  } catch {
    return undefined;
  }
}

function openWorktreeCatalogStream(
  queryClient: QueryClient,
  workspaceIds: readonly string[],
  eventSourceFactory: WorktreeCatalogEventSourceFactory,
  onStatusChange: (status: Exclude<WorktreeCatalogStreamStatus, "disabled">) => void
): () => void {
  const authorizedWorkspaceIds = new Set(workspaceIds);
  // A reconnect may have missed frames, so every authorized workspace rereads.
  const reconcileWorkspaces: EventListener = () => {
    onStatusChange("live");
    for (const workspaceId of authorizedWorkspaceIds) {
      void queryClient.invalidateQueries({ queryKey: workspaceKeys.worktrees(workspaceId) });
    }
  };
  const handleStreamError: EventListener = () => onStatusChange("stale");
  const handleCatalogChange: EventListener = event => {
    const payload = parseWorktreeCatalogEvent(event);
    // Frames for workspaces this client does not hold are dropped before any
    // cache write, so a catalog frame can never leak across workspaces.
    if (!payload || !authorizedWorkspaceIds.has(payload.workspace_id)) return;
    void queryClient.invalidateQueries({
      queryKey: workspaceKeys.worktrees(payload.workspace_id),
    });
    void queryClient.invalidateQueries({
      queryKey: workspaceKeys.worktreeDetail(payload.workspace_id, payload.worktree_id),
      exact: true,
    });
  };

  const source = eventSourceFactory(worktreeCatalogStreamURL());
  try {
    source.addEventListener("open", reconcileWorkspaces);
    source.addEventListener("error", handleStreamError);
    source.addEventListener(WORKTREE_CATALOG_CHANGED_EVENT, handleCatalogChange);
  } catch (error) {
    source.removeEventListener("open", reconcileWorkspaces);
    source.removeEventListener("error", handleStreamError);
    source.removeEventListener(WORKTREE_CATALOG_CHANGED_EVENT, handleCatalogChange);
    source.close();
    throw error;
  }

  return () => {
    source.removeEventListener("open", reconcileWorkspaces);
    source.removeEventListener("error", handleStreamError);
    source.removeEventListener(WORKTREE_CATALOG_CHANGED_EVENT, handleCatalogChange);
    source.close();
  };
}

/**
 * Reconciles live worktree catalog frames into the query cache. It never writes
 * cache data — TanStack Query stays the only owner of the remote snapshot; this
 * effect only invalidates workspace-qualified keys.
 */
export function useWorktreeCatalogStream(
  workspaces: ReadonlyArray<{ id: string }>,
  { enabled = true, eventSourceFactory }: UseWorktreeCatalogStreamOptions = {}
) {
  const queryClient = useQueryClient();
  const workspaceIds = [
    ...new Set(
      workspaces.flatMap(workspace => {
        const workspaceId = workspace.id.trim();
        return workspaceId === "" ? [] : [workspaceId];
      })
    ),
  ];
  const workspaceKey = JSON.stringify(workspaceIds);
  const canConnect =
    enabled &&
    // Checked on the list, not the serialized key: an empty array still
    // serializes to a non-empty string.
    workspaceIds.length > 0 &&
    typeof window !== "undefined" &&
    (eventSourceFactory !== undefined || typeof EventSource !== "undefined");
  const store = useStore(worktreeCatalogStreamLogic);
  const status = useSelector(store, snapshot => snapshot.context.status);

  useEffect(() => {
    if (!canConnect) {
      store.trigger.connectionDisabled();
      return undefined;
    }

    store.trigger.connectionRequested({
      connect: onStatusChange =>
        openWorktreeCatalogStream(
          queryClient,
          JSON.parse(workspaceKey) as string[],
          eventSourceFactory ?? defaultEventSourceFactory,
          onStatusChange
        ),
    });
    return () => store.trigger.connectionDisabled();
  }, [canConnect, eventSourceFactory, queryClient, store, workspaceKey]);

  return status;
}
