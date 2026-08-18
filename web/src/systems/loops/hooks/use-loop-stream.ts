import { useEffect, useEffectEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useStore } from "@xstate/store-react";

import { LOOP_RUN_EVENT_KINDS, LOOP_RUN_LIFECYCLE_EVENT_KINDS } from "@/generated/loop-enums";
import { createStreamEventSource } from "@/lib/ticketed-event-source";

import { buildLoopStreamUrl } from "../adapters/loops-api";
import { isLoopRequestEventKind } from "../lib/loop-graph-events";
import { loopsKeys } from "../lib/query-keys";
import type { LoopRunEventFrame, LoopRunEventKind } from "../types";
import {
  loopStreamLifecycleLogic,
  type LoopStreamSubscription,
} from "./loop-stream-lifecycle-store";

interface LoopStreamEventSource {
  addEventListener: (type: string, listener: EventListenerOrEventListenerObject) => void;
  removeEventListener?: (type: string, listener: EventListenerOrEventListenerObject) => void;
  close: () => void;
  onmessage: ((event: MessageEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
}

type LoopStreamEventSourceFactory = (url: string) => LoopStreamEventSource;

interface UseLoopStreamOptions {
  enabled?: boolean;
  afterSequence?: number;
  eventSourceFactory?: LoopStreamEventSourceFactory;
  onEvent?: (payload: LoopRunEventFrame, subscription: LoopStreamSubscription) => void;
  onError?: (error: unknown) => void;
}

const LOOP_STREAM_EVENT_TYPES = LOOP_RUN_EVENT_KINDS satisfies readonly LoopRunEventKind[];

const LOOP_LIFECYCLE_EVENT_KINDS = new Set<LoopRunEventKind>(
  LOOP_RUN_LIFECYCLE_EVENT_KINDS satisfies readonly LoopRunEventKind[]
);

function isLifecycleKind(kind: string): boolean {
  return LOOP_LIFECYCLE_EVENT_KINDS.has(kind as LoopRunEventKind);
}

function isRequestKind(kind: string): boolean {
  return isLoopRequestEventKind(kind);
}

function defaultEventSourceFactory(url: string): LoopStreamEventSource {
  return createStreamEventSource(url);
}

function attachLoopStreamSource(
  source: LoopStreamEventSource,
  handleFrame: (event: MessageEvent) => void,
  handleError: (event: Event) => void
): () => void {
  source.onmessage = handleFrame;
  source.onerror = handleError;
  const namedListener = handleFrame as EventListener;
  for (const type of LOOP_STREAM_EVENT_TYPES) {
    source.addEventListener(type, namedListener);
  }
  return () => {
    if (source.removeEventListener) {
      for (const type of LOOP_STREAM_EVENT_TYPES) {
        source.removeEventListener(type, namedListener);
      }
    }
    source.onmessage = null;
    source.onerror = null;
    source.close();
  };
}

type QueryClient = ReturnType<typeof useQueryClient>;

function invalidateLoopRunQueries(
  queryClient: QueryClient,
  workspaceId: string,
  runId: string,
  refreshCatalog: boolean
) {
  void queryClient.invalidateQueries({ queryKey: loopsKeys.runDetail(workspaceId, runId) });
  void queryClient.invalidateQueries({ queryKey: loopsKeys.runsByWorkspace(workspaceId) });
  if (refreshCatalog) {
    void queryClient.invalidateQueries({ queryKey: loopsKeys.catalogByWorkspace(workspaceId) });
  }
}

function invalidateLoopRequestQueries(
  queryClient: QueryClient,
  workspaceId: string,
  runId: string
) {
  void queryClient.invalidateQueries({ queryKey: loopsKeys.runDetail(workspaceId, runId) });
  void queryClient.invalidateQueries({ queryKey: loopsKeys.nodeInventoryByWorkspace(workspaceId) });
  void queryClient.invalidateQueries({ queryKey: loopsKeys.requestsByWorkspace(workspaceId) });
}

/**
 * Subscribes to a Loop run's SSE event stream, mirroring `useTaskStream`: named
 * listeners for every enumerated kind, `onEvent` on each frame, `afterSequence`
 * resume, and query invalidation on the lifecycle kinds (`token_tick`/`channel_msg`
 * are display-only, applied via `onEvent`, never invalidating). `onEvent`/`onError`
 * are stabilized through Effect Events so an inline callback never tears down
 * and reopens the EventSource; only the workspace, run, resume seed, or factory
 * identity reopen the stream. The mounted hook owns the source until cleanup;
 * terminal effects can append frames after the run status becomes terminal.
 */
export function useLoopStream(
  workspaceId: string,
  runId: string,
  {
    enabled = true,
    afterSequence,
    eventSourceFactory: customEventSourceFactory,
    onEvent,
    onError,
  }: UseLoopStreamOptions = {}
) {
  const queryClient = useQueryClient();
  const lifecycleStore = useStore(loopStreamLifecycleLogic);
  const trimmedWorkspace = workspaceId.trim();
  const trimmedRun = runId.trim();

  const notifyEvent = useEffectEvent(
    (payload: LoopRunEventFrame, subscription: LoopStreamSubscription) => {
      onEvent?.(payload, subscription);
    }
  );
  const notifyError = useEffectEvent((error: unknown, fallback: string) => {
    if (onError) {
      onError(error);
      return;
    }
    console.error(fallback, error);
  });

  useEffect(() => {
    if (
      !enabled ||
      trimmedWorkspace === "" ||
      trimmedRun === "" ||
      typeof window === "undefined" ||
      (!customEventSourceFactory && typeof EventSource === "undefined")
    ) {
      return undefined;
    }

    const url = buildLoopStreamUrl(trimmedWorkspace, trimmedRun, {
      after_sequence: afterSequence === undefined ? undefined : String(afterSequence),
    });
    const source = (customEventSourceFactory ?? defaultEventSourceFactory)(url);
    const subscription = {
      generation: lifecycleStore.getSnapshot().context.generation + 1,
      runId: trimmedRun,
      workspaceId: trimmedWorkspace,
    };
    lifecycleStore.trigger.subscriptionOpened({ subscription });

    const handleFrame = (event: MessageEvent) => {
      if (lifecycleStore.getSnapshot().context.activeSubscription !== subscription) {
        return;
      }
      if (typeof event.data !== "string") {
        return;
      }
      // Only genuine parse failures reach onError; a throw inside the consumer's
      // onEvent sink must propagate, not be misreported as a stream-parse error.
      let payload: LoopRunEventFrame;
      try {
        payload = JSON.parse(event.data) as LoopRunEventFrame;
      } catch (error) {
        notifyError(error, "Failed to parse loop stream payload");
        return;
      }
      // Named frames carry the kind as event.type; the defensive onmessage frame
      // ("message") falls back to the parsed payload kind.
      const kind = event.type !== "message" ? event.type : (payload.kind ?? "");
      if (isLifecycleKind(kind)) {
        invalidateLoopRunQueries(
          queryClient,
          trimmedWorkspace,
          trimmedRun,
          kind === "status_changed"
        );
      } else if (isRequestKind(kind)) {
        invalidateLoopRequestQueries(queryClient, trimmedWorkspace, trimmedRun);
      }
      notifyEvent(payload, subscription);
    };

    const handleError = (event: Event) => {
      notifyError(event, "Loop stream failed");
    };

    const detach = attachLoopStreamSource(source, handleFrame, handleError);
    return () => {
      lifecycleStore.trigger.subscriptionClosed({ subscription });
      detach();
    };
  }, [
    afterSequence,
    customEventSourceFactory,
    enabled,
    lifecycleStore,
    queryClient,
    trimmedRun,
    trimmedWorkspace,
  ]);
}

export type { LoopStreamEventSource, LoopStreamEventSourceFactory, UseLoopStreamOptions };
export type { LoopStreamSubscription } from "./loop-stream-lifecycle-store";
