"use client";

import * as React from "react";

import { cn } from "../../lib/utils";
import {
  loadTerminalEngine,
  type TerminalDimensions,
  type TerminalEngineLoader,
} from "./terminal-engine";
import type { TerminalInstance } from "./terminal-registry";
import type { TerminalRendererKind } from "./terminal-renderer";
import { useTerminalInstance } from "./use-terminal-instance";

export type { TerminalDimensions, TerminalEngine, TerminalEngineLoader } from "./terminal-engine";
export type { TerminalRendererKind } from "./terminal-renderer";
export { destroyTerminalInstance, destroyTerminalInstances } from "./terminal-registry";

import { useTerminalPendingQueue, type PendingQueue } from "./use-terminal-pending-queue";

/**
 * A scrollback range, numbered the way people and the CLI count lines.
 *
 * The emulator counts rows from zero; every surface a person reads — the quote
 * block, `--lines A-B`, the journal — counts from one. Converting here, once,
 * keeps the two from drifting apart in every consumer.
 */
export interface TerminalSelectionRange {
  /** 1-based, inclusive. */
  startLine: number;
  /** 1-based, inclusive. */
  endLine: number;
  text: string;
}

/** The imperative surface a byte stream drives the view through. */
export interface TerminalViewHandle {
  /**
   * Writes bytes and resolves once the emulator has *parsed* them. Callers that
   * return flow-control credit must wait for this, not for the socket read.
   */
  write(data: string | Uint8Array): Promise<void>;
  /** The size this container could host. Advisory — never applied here. */
  proposeDimensions(): TerminalDimensions | null;
  /** Applies a size decided elsewhere. */
  applyDimensions(dimensions: TerminalDimensions): void;
  /** Clears the screen and every mode, for a replay from a clean slate. */
  reset(): void;
  focus(): void;
  getSelection(): string;
  getSelectionRange(): TerminalSelectionRange | null;
}

export interface TerminalViewProps extends Omit<React.ComponentProps<"div">, "children"> {
  /**
   * Stable identity of the buffer. Two mounts sharing an id share one emulator,
   * which is how a tab switch keeps its scrollback.
   */
  instanceId: string;
  /** Names the grid for assistive technology. */
  "aria-label": string;
  /** Suppresses local input. The authority over writing lives elsewhere. */
  readOnly?: boolean;
  screenReaderMode?: boolean;
  scrollbackLines?: number;
  /** Replaces the emulator, for tests and playback harnesses. */
  engineLoader?: TerminalEngineLoader;
  handleRef?: React.Ref<TerminalViewHandle>;
  onData?: (data: string) => void;
  onSelectionChange?: (selection: string) => void;
  onProposeDimensions?: (dimensions: TerminalDimensions) => void;
  onRendererChange?: (renderer: TerminalRendererKind) => void;
  /**
   * The emulator is attached, and everything queued before it has been replayed.
   *
   * Nothing written earlier is lost — calls made before the engine loads are
   * held in order and flushed here — so this is a "the screen is now live"
   * signal rather than a gate anyone has to wait behind.
   */
  onAttached?: () => void;
}

const DEFAULT_SCROLLBACK_LINES = 1000;

/**
 * A live byte-stream grid.
 *
 * The view owns rendering and lifetime; it owns no protocol. Bytes arrive
 * through the handle, size is proposed but never self-applied, and read-only is
 * a local courtesy rather than a claim about who may write.
 */
export function TerminalView({
  instanceId,
  readOnly = false,
  screenReaderMode = false,
  scrollbackLines = DEFAULT_SCROLLBACK_LINES,
  engineLoader = loadTerminalEngine,
  handleRef,
  onData,
  onSelectionChange,
  onProposeDimensions,
  onRendererChange,
  onAttached,
  className,
  ...props
}: TerminalViewProps) {
  const [renderer, setRenderer] = React.useState<TerminalRendererKind | null>(null);
  const pending = useTerminalPendingQueue(instanceId);
  const { containerRef, instanceRef } = useTerminalInstance({
    instanceKey: instanceId,
    readOnly,
    screenReaderMode,
    scrollbackLines,
    engineLoader,
    onData: data => onData?.(data),
    onSelectionChange: selection => onSelectionChange?.(selection),
    onProposeDimensions: dimensions => onProposeDimensions?.(dimensions),
    onRendererChange: kind => {
      setRenderer(kind);
      onRendererChange?.(kind);
    },
    onAttached: instance => {
      // Order is preserved across the boundary: a size applied before the first
      // byte was queued first, so it is applied first here too. `onAttached`
      // fires only once every queued parse has actually completed — it means
      // "the screen is live", and it would not be true a moment earlier.
      void flushPendingOperations(instance, pending).then(
        () => onAttached?.(),
        // The view was replaced while its queue was draining; nothing here is
        // an error, and every caller has already been answered.
        () => undefined
      );
    },
  });

  React.useImperativeHandle(handleRef, () => createHandle(instanceRef, pending), [
    instanceRef,
    pending,
  ]);

  return (
    <div
      className={cn("relative min-h-0 min-w-0 flex-1 overflow-hidden", className)}
      data-readonly={readOnly ? "true" : undefined}
      data-renderer={renderer ?? undefined}
      data-slot="terminal-view"
      ref={containerRef}
      role="log"
      {...props}
    />
  );
}

/**
 * Writes bytes and resolves on the parse — or rejects if the view goes first.
 *
 * Every write goes through here, queued or live, so that exactly one thing is
 * true of all of them: a caller is answered once, and a parse callback that
 * arrives after the view was abandoned changes nothing. Without the record, a
 * write still being parsed at unmount would wait on a callback the disposed
 * emulator will never make.
 */
function trackedWrite(
  instance: TerminalInstance,
  queue: PendingQueue,
  data: string | Uint8Array
): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    let settled = false;
    const cancel = (cause: unknown) => {
      if (settled) return;
      settled = true;
      reject(cause);
    };
    queue.inFlight.add(cancel);
    instance.terminal.write(data, () => {
      if (settled) return;
      settled = true;
      queue.inFlight.delete(cancel);
      resolve();
    });
  });
}

async function flushPendingOperations(
  instance: TerminalInstance,
  queue: PendingQueue
): Promise<void> {
  while (queue.operations.length > 0) {
    const next = queue.operations.shift();
    if (!next) return;
    if (next.kind === "reset") {
      instance.terminal.reset();
      continue;
    }
    if (next.kind === "resize") {
      instance.terminal.resize(next.cols, next.rows);
      continue;
    }
    // Awaited one at a time: the caller's promise resolves on the parse, and a
    // later reset in this same queue must not run before an earlier write has
    // been drawn. It is tracked as in-flight while it waits, because it has
    // already left the queue and a view that closes now must still answer it.
    await trackedWrite(instance, queue, next.data).then(next.resolve, next.reject);
  }
}

function createHandle(
  instanceRef: React.RefObject<TerminalInstance | null>,
  pending: PendingQueue
): TerminalViewHandle {
  return {
    write: data => {
      const instance = instanceRef.current;
      if (instance) return trackedWrite(instance, pending, data);
      return new Promise<void>((resolve, reject) => {
        pending.operations.push({ kind: "write", data, resolve, reject });
      });
    },
    proposeDimensions: () => instanceRef.current?.fit.proposeDimensions() ?? null,
    applyDimensions: ({ cols, rows }) => {
      const instance = instanceRef.current;
      if (!instance) {
        pending.operations.push({ kind: "resize", cols, rows });
        return;
      }
      instance.terminal.resize(cols, rows);
    },
    reset: () => {
      const instance = instanceRef.current;
      if (!instance) {
        pending.operations.push({ kind: "reset" });
        return;
      }
      instance.terminal.reset();
    },
    focus: () => instanceRef.current?.terminal.focus(),
    getSelection: () => instanceRef.current?.terminal.getSelection() ?? "",
    getSelectionRange: () => {
      const instance = instanceRef.current;
      if (!instance) return null;
      const position = instance.terminal.getSelectionPosition();
      if (!position) return null;
      return {
        startLine: position.start.y + 1,
        endLine: position.end.y + 1,
        text: instance.terminal.getSelection(),
      };
    },
  };
}
