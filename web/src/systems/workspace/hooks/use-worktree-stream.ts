import { useSelector, useStore } from "@xstate/store-react";
import { useEffect } from "react";
import { useQueryClient, type QueryClient } from "@tanstack/react-query";

import { createStreamEventSource } from "@/lib/ticketed-event-source";

import {
  parseWorktreeExitEvent,
  WORKTREE_EXIT_EVENTS,
  WORKTREE_EXIT_TERMINAL_EVENTS,
  WORKTREE_LIFECYCLE_EVENTS,
  type WorktreeExitEventName,
  type WorktreeExitEventPayload,
  type WorktreeStreamEventName,
} from "../lib/worktree-events";
import { invalidateWorktreeExitScope } from "./use-worktree-exit";
import { worktreeStreamLogic, type WorktreeStreamStatus } from "./worktree-stream-store";

export interface WorktreeEventSource {
  addEventListener: (type: string, listener: EventListenerOrEventListenerObject) => void;
  removeEventListener: (type: string, listener: EventListenerOrEventListenerObject) => void;
  close: () => void;
}

export type WorktreeEventSourceFactory = (url: string) => WorktreeEventSource;

export type WorktreeExitEventHandler = (
  name: WorktreeExitEventName,
  payload: WorktreeExitEventPayload
) => void;

interface UseWorktreeStreamOptions {
  enabled?: boolean;
  onExitEvent?: WorktreeExitEventHandler;
  eventSourceFactory?: WorktreeEventSourceFactory;
}

export type { WorktreeStreamStatus } from "./worktree-stream-store";

/** Quiet window after the last frame before the worktree scope rereads once. */
export const WORKTREE_REREAD_COALESCE_MS = 150;
/** Longest a reread waits while frames keep landing inside the quiet window. */
export const WORKTREE_REREAD_MAX_WAIT_MS = 1000;

export function worktreeStreamURL(workspaceID: string, worktreeID: string): string {
  return `/api/workspaces/${encodeURIComponent(workspaceID)}/worktrees/${encodeURIComponent(worktreeID)}/stream`;
}

function defaultEventSourceFactory(url: string): WorktreeEventSource {
  return createStreamEventSource(url, { resumeWithLastEventId: true });
}

interface CoalescedReread {
  request: () => void;
  /** Pays a reread still waiting for its quiet window, so no frame is lost with the connection. */
  flush: () => void;
}

/**
 * Collapses a burst of frames into one scope reread. The first connection
 * replays the retained history (hundreds of frames in under a second) and every
 * lifecycle frame asks for the same four reads, so rereading per frame repeats
 * the same requests once per frame and cancels its own in-flight fetches on
 * each repeat. Each frame restarts the quiet window; the ceiling is armed by
 * the first frame only, so a steady trickle of frames cannot defer the reread
 * past it.
 */
function createCoalescedReread(
  reread: () => void,
  delayMs: number,
  maxWaitMs: number
): CoalescedReread {
  let quiet: ReturnType<typeof setTimeout> | undefined;
  let ceiling: ReturnType<typeof setTimeout> | undefined;
  const fire = () => {
    if (quiet !== undefined) clearTimeout(quiet);
    if (ceiling !== undefined) clearTimeout(ceiling);
    quiet = undefined;
    ceiling = undefined;
    reread();
  };
  return {
    request: () => {
      if (quiet !== undefined) clearTimeout(quiet);
      quiet = setTimeout(fire, delayMs);
      if (ceiling === undefined) ceiling = setTimeout(fire, maxWaitMs);
    },
    flush: () => {
      if (quiet === undefined && ceiling === undefined) return;
      fire();
    },
  };
}

interface WorktreeStreamBinding {
  queryClient: QueryClient;
  workspaceID: string;
  worktreeID: string;
  onExitEvent?: WorktreeExitEventHandler;
}

interface WorktreeListenerSet {
  listeners: Array<[string, EventListener]>;
  /** Pays a reread still waiting for its quiet window before the listeners go. */
  dispose: () => void;
}

/**
 * Builds one named listener per stream event. Every entry is registered and
 * removed by the same reference, so the disposer takes down exactly what it
 * added and never leaves a frame handler attached to a closed source.
 */
function buildWorktreeListeners(
  binding: WorktreeStreamBinding,
  onStatusChange: (status: Exclude<WorktreeStreamStatus, "disabled">) => void
): WorktreeListenerSet {
  const { queryClient, workspaceID, worktreeID, onExitEvent } = binding;
  const reread = createCoalescedReread(
    () => {
      void invalidateWorktreeExitScope(queryClient, workspaceID, worktreeID);
    },
    WORKTREE_REREAD_COALESCE_MS,
    WORKTREE_REREAD_MAX_WAIT_MS
  );
  const rereadScope = reread.request;
  // A reconnect may have missed frames, so the whole worktree scope rereads.
  const handleOpen: EventListener = () => {
    onStatusChange("live");
    rereadScope();
  };
  const handleError: EventListener = () => onStatusChange("stale");

  const listeners: Array<[string, EventListener]> = [
    ["open", handleOpen],
    ["error", handleError],
  ];
  for (const name of WORKTREE_LIFECYCLE_EVENTS) {
    listeners.push([name, rereadScope]);
  }
  for (const name of WORKTREE_EXIT_EVENTS) {
    listeners.push([
      name,
      event => {
        if (!(event instanceof MessageEvent) || typeof event.data !== "string") return;
        const payload = parseWorktreeExitEvent(event.data);
        if (!payload) return;
        onExitEvent?.(name, payload);
        if (WORKTREE_EXIT_TERMINAL_EVENTS.includes(name)) rereadScope();
      },
    ]);
  }
  return { listeners, dispose: reread.flush };
}

function openWorktreeStream(
  binding: WorktreeStreamBinding,
  eventSourceFactory: WorktreeEventSourceFactory,
  onStatusChange: (status: Exclude<WorktreeStreamStatus, "disabled">) => void
): () => void {
  const { listeners, dispose } = buildWorktreeListeners(binding, onStatusChange);
  const source = eventSourceFactory(worktreeStreamURL(binding.workspaceID, binding.worktreeID));
  const detach = () => {
    dispose();
    for (const [name, listener] of listeners) source.removeEventListener(name, listener);
  };
  try {
    for (const [name, listener] of listeners) source.addEventListener(name, listener);
  } catch (error) {
    detach();
    source.close();
    throw error;
  }

  return () => {
    detach();
    source.close();
  };
}

const WORKTREE_STREAM_EVENT_NAMES: readonly WorktreeStreamEventName[] = [
  ...WORKTREE_LIFECYCLE_EVENTS,
  ...WORKTREE_EXIT_EVENTS,
];

export { WORKTREE_STREAM_EVENT_NAMES };

/**
 * Subscribes to one worktree's durable event stream while its context is open.
 * Frames only ever invalidate worktree-qualified keys — the hook never writes
 * cache data, so TanStack Query stays the single owner of the remote snapshot.
 */
export function useWorktreeStream(
  workspaceID: string,
  worktreeID: string,
  { enabled = true, onExitEvent, eventSourceFactory }: UseWorktreeStreamOptions = {}
) {
  const queryClient = useQueryClient();
  const scopedWorkspaceID = workspaceID.trim();
  const scopedWorktreeID = worktreeID.trim();
  const canConnect =
    enabled &&
    scopedWorkspaceID !== "" &&
    scopedWorktreeID !== "" &&
    typeof window !== "undefined" &&
    (eventSourceFactory !== undefined || typeof EventSource !== "undefined");
  const store = useStore(worktreeStreamLogic);
  const status = useSelector(store, snapshot => snapshot.context.status);

  useEffect(() => {
    if (!canConnect) {
      store.trigger.connectionDisabled();
      return undefined;
    }

    store.trigger.connectionRequested({
      connect: onStatusChange =>
        openWorktreeStream(
          {
            queryClient,
            workspaceID: scopedWorkspaceID,
            worktreeID: scopedWorktreeID,
            onExitEvent,
          },
          eventSourceFactory ?? defaultEventSourceFactory,
          onStatusChange
        ),
    });
    return () => store.trigger.connectionDisabled();
  }, [
    canConnect,
    eventSourceFactory,
    onExitEvent,
    queryClient,
    scopedWorkspaceID,
    scopedWorktreeID,
    store,
  ]);

  return status;
}
