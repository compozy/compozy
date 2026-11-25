import { useStore, useSelector } from "@xstate/store-react";
import { useEffect } from "react";
import { useQueryClient, type QueryClient } from "@tanstack/react-query";

import { useCurrentComputerId } from "@/hooks/use-current-computer-id";
import { computerApiPath } from "@/lib/api-client";
import type { ComputerId } from "@/lib/computer-scope";
import { createStreamEventSource } from "@/lib/ticketed-event-source";

import { workspaceKeys } from "../lib/query-keys";
import {
  createCoalescedReread,
  WORKTREE_REREAD_COALESCE_MS,
  WORKTREE_REREAD_MAX_WAIT_MS,
} from "../lib/worktree-reread";
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
  computerId?: ComputerId;
  eventSourceFactory?: WorktreeCatalogEventSourceFactory;
}

export type { WorktreeCatalogStreamStatus } from "./worktree-catalog-stream-store";

export function worktreeCatalogStreamURL(computerId?: ComputerId): string {
  return computerApiPath(computerId, "/api/worktrees/catalog-stream");
}

function defaultEventSourceFactory(url: string): WorktreeCatalogEventSource {
  return createStreamEventSource(url, { transport: "websocket" });
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
  onStatusChange: (status: Exclude<WorktreeCatalogStreamStatus, "disabled">) => void,
  computerId?: ComputerId
): () => void {
  const keys = workspaceKeys.forComputer(computerId);
  const authorizedWorkspaceIds = new Set(workspaceIds);
  const pendingWorkspaces = new Set<string>();
  const reread = createCoalescedReread(
    () => {
      for (const workspaceId of pendingWorkspaces) {
        // The list prefix includes its details, status and exit plan queries.
        void queryClient.invalidateQueries({ queryKey: keys.worktrees(workspaceId) });
      }
      pendingWorkspaces.clear();
      void queryClient.invalidateQueries({ queryKey: keys.worktreeAggregates() });
    },
    WORKTREE_REREAD_COALESCE_MS,
    WORKTREE_REREAD_MAX_WAIT_MS
  );
  const reconcileWorkspaces: EventListener = () => {
    onStatusChange("live");
    for (const workspaceId of authorizedWorkspaceIds) pendingWorkspaces.add(workspaceId);
    reread.request();
  };
  const handleStreamError: EventListener = () => {
    reread.flush();
    onStatusChange("stale");
  };
  const handleCatalogChange: EventListener = event => {
    const payload = parseWorktreeCatalogEvent(event);
    // Frames for workspaces this client does not hold are dropped before any
    // cache write, so a catalog frame can never leak across workspaces.
    if (!payload || !authorizedWorkspaceIds.has(payload.workspace_id)) return;
    if (payload.kind === "failed") {
      queryClient.setQueryData(
        keys.worktreeMaterializationFailure(payload.workspace_id, payload.worktree_id),
        payload.error || "Worktree creation failed and was rolled back."
      );
    }
    pendingWorkspaces.add(payload.workspace_id);
    reread.request();
  };

  const source = eventSourceFactory(worktreeCatalogStreamURL(computerId));
  try {
    source.addEventListener("open", reconcileWorkspaces);
    source.addEventListener("error", handleStreamError);
    source.addEventListener(WORKTREE_CATALOG_CHANGED_EVENT, handleCatalogChange);
  } catch (error) {
    reread.flush();
    source.removeEventListener("open", reconcileWorkspaces);
    source.removeEventListener("error", handleStreamError);
    source.removeEventListener(WORKTREE_CATALOG_CHANGED_EVENT, handleCatalogChange);
    source.close();
    throw error;
  }

  return () => {
    reread.flush();
    source.removeEventListener("open", reconcileWorkspaces);
    source.removeEventListener("error", handleStreamError);
    source.removeEventListener(WORKTREE_CATALOG_CHANGED_EVENT, handleCatalogChange);
    source.close();
  };
}

/**
 * Reconciles live worktree catalog frames into the query cache. TanStack Query
 * owns the remote snapshot: the stream invalidates workspace-qualified keys and
 * stores only the daemon's terminal materialization failure handoff.
 */
export function useWorktreeCatalogStream(
  workspaces: ReadonlyArray<{ id: string }>,
  { enabled = true, computerId, eventSourceFactory }: UseWorktreeCatalogStreamOptions = {}
) {
  const queryClient = useQueryClient();
  const scopedComputerId = useCurrentComputerId(computerId);
  const workspaceIds = [
    ...new Set(
      workspaces.flatMap(workspace => {
        const workspaceId = workspace.id.trim();
        return workspaceId === "" ? [] : [workspaceId];
      })
    ),
  ];
  const workspaceKey = JSON.stringify(workspaceIds.sort());
  const canConnect =
    enabled &&
    // Checked on the list, not the serialized key: an empty array still
    // serializes to a non-empty string.
    workspaceIds.length > 0 &&
    typeof window !== "undefined" &&
    (eventSourceFactory !== undefined || typeof WebSocket !== "undefined");
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
          onStatusChange,
          scopedComputerId
        ),
    });
    return () => store.trigger.connectionDisabled();
  }, [canConnect, eventSourceFactory, queryClient, scopedComputerId, store, workspaceKey]);

  return status;
}
