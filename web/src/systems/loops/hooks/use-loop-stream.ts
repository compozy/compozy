import { useEffect, useEffectEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { LOOP_RUN_EVENT_KINDS, LOOP_RUN_LIFECYCLE_EVENT_KINDS } from "@/generated/loop-enums";

import { buildLoopStreamUrl } from "../adapters/loops-api";
import { isTerminalLoopStatus } from "../lib/loop-formatters";
import { loopsKeys } from "../lib/query-keys";
import type { LoopRunEventFrame, LoopRunEventKind } from "../types";

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
  onEvent?: (payload: LoopRunEventFrame) => void;
  onError?: (error: unknown) => void;
}

// AGH Loop SSE emits named events via `event: <kind>` from the run-events writer
// (internal/daemon). EventSource routes named SSE frames to listeners registered
// with addEventListener("<kind>", ...); they never reach onmessage, which only
// handles unnamed `message` frames. Keep this list aligned with the enumerated
// LoopRunEventKind contract (techspec §observability, L-017 named-listener rule):
// an unenumerated kind silently never renders.
const LOOP_STREAM_EVENT_TYPES = LOOP_RUN_EVENT_KINDS satisfies readonly LoopRunEventKind[];

// Lifecycle kinds mutate durable run state, so each one invalidates the run detail +
// runs list (daemon truth wins). A status transition also invalidates the catalog's
// `last_run`/aggregate projection. The high-frequency display frames `token_tick` and
// `channel_msg` are applied locally via `onEvent` (the run-page meter/timeline store,
// task 20) and never invalidate — otherwise every tick would refetch the workspace
// runs list.
const LOOP_LIFECYCLE_EVENT_KINDS = new Set<LoopRunEventKind>(
  LOOP_RUN_LIFECYCLE_EVENT_KINDS satisfies readonly LoopRunEventKind[]
);

function isLifecycleKind(kind: string): boolean {
  return LOOP_LIFECYCLE_EVENT_KINDS.has(kind as LoopRunEventKind);
}

function isTerminalStatusFrame(kind: string, frame: LoopRunEventFrame): boolean {
  if (kind !== "status_changed" || typeof frame.payload !== "object" || frame.payload === null) {
    return false;
  }
  const status = (frame.payload as Record<string, unknown>).status;
  return typeof status === "string" && isTerminalLoopStatus(status);
}

function defaultEventSourceFactory(url: string): LoopStreamEventSource {
  return new EventSource(url);
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

/**
 * Subscribes to a Loop run's SSE event stream, mirroring `useTaskStream`: named
 * listeners for every enumerated kind, `onEvent` on each frame, `afterSequence`
 * resume, and query invalidation on the lifecycle kinds (`token_tick`/`channel_msg`
 * are display-only, applied via `onEvent`, never invalidating). `onEvent`/`onError`
 * are stabilized through Effect Events so an inline callback never tears down
 * and reopens the EventSource; only the workspace, run, resume seed, or factory
 * identity reopen the stream.
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
  const trimmedWorkspace = workspaceId.trim();
  const trimmedRun = runId.trim();

  const notifyEvent = useEffectEvent((payload: LoopRunEventFrame) => {
    onEvent?.(payload);
  });
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

    const handleFrame = (event: MessageEvent) => {
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
      }
      notifyEvent(payload);
      if (isTerminalStatusFrame(kind, payload)) {
        source.close();
      }
    };

    const handleError = (event: Event) => {
      notifyError(event, "Loop stream failed");
    };

    return attachLoopStreamSource(source, handleFrame, handleError);
  }, [enabled, trimmedWorkspace, trimmedRun, afterSequence, customEventSourceFactory, queryClient]);
}

export type { LoopStreamEventSource, LoopStreamEventSourceFactory, UseLoopStreamOptions };
