"use client";

import * as React from "react";

import { TerminalWriteAbandonedError } from "./terminal-write-abandoned";

/** One call made before the emulator existed, waiting its turn. */
export type PendingOperation =
  | {
      kind: "write";
      data: string | Uint8Array;
      resolve: () => void;
      reject: (cause: unknown) => void;
    }
  | { kind: "reset" }
  | { kind: "resize"; cols: number; rows: number };

/** Pending work, tagged with the buffer it was meant for. */
export interface PendingQueue {
  key: string;
  operations: PendingOperation[];
  /** Keeps live calls behind the startup operations already being replayed. */
  draining: boolean;
  /**
   * Writes handed to the emulator but not yet parsed.
   *
   * A write leaves `operations` the moment it starts, so without this a view
   * that goes away mid-parse would have nothing left to answer and its caller
   * would wait forever.
   */
  inFlight: Set<(cause: unknown) => void>;
}

function createQueue(key: string): PendingQueue {
  return { key, operations: [], draining: false, inFlight: new Set() };
}

/**
 * Holds what was written before the emulator existed.
 *
 * The engine is fetched on first attach, so a stream that starts painting
 * immediately would write into nothing. Those calls are held in arrival order
 * and flushed the moment the emulator exists.
 *
 * The queue belongs to exactly one buffer. When the view is pointed at a
 * different terminal — or unmounts — before those calls are drawn, they are
 * abandoned and every waiting caller told so, rather than left pending forever
 * or, worse, resolved as if the bytes had been drawn.
 */
export function useTerminalPendingQueue(instanceId: string): PendingQueue {
  const [queue, setQueue] = React.useState<PendingQueue>(() => createQueue(instanceId));
  const active = queue.key === instanceId ? queue : createQueue(instanceId);
  if (active !== queue) setQueue(active);
  // StrictMode runs an effect's cleanup and then sets it up again on the same
  // mount. Abandoning the queue in that cleanup would discard output that is
  // still perfectly valid, so the claim is held in a ref both passes can see:
  // cleanup releases it, setup re-takes it, and the decision — deferred by a
  // microtask — sees that it was never really given up.
  const claim = React.useRef<PendingQueue | null>(null);
  React.useEffect(() => {
    claim.current = active;
    return () => {
      if (claim.current === active) claim.current = null;
      void Promise.resolve().then(() => {
        if (claim.current === active) return;
        abandonPendingOperations(active);
      });
    };
  }, [active]);
  return active;
}

/**
 * Tells everything still waiting that its buffer went away.
 *
 * A write that will never be parsed still has a caller awaiting it — the
 * terminal client waits on exactly these promises before returning flow-control
 * credit and advancing its resume point — so they are rejected, not resolved.
 * Nothing was drawn, and nothing may be recorded as drawn. Writes already
 * handed to a disposed emulator are answered the same way: their parse callback
 * is never going to fire.
 */
export function abandonPendingOperations(
  queue: PendingQueue,
  cause: unknown = new TerminalWriteAbandonedError()
): void {
  for (const operation of queue.operations) {
    if (operation.kind === "write") operation.reject(cause);
  }
  queue.operations.length = 0;
  for (const reject of queue.inFlight) {
    reject(cause);
  }
  queue.inFlight.clear();
}
