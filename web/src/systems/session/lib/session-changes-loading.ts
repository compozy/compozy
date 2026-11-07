import type { SessionChangePatch, SessionChangeTarget } from "../adapters/session-changes-api";
import { flattenTranscriptEntries, type SessionTranscriptData } from "./session-transcript-query";

/** The daemon's patch batch caps: 20 paths and 8 KiB of path bytes per request. */
export const PATCH_BATCH_MAX_PATHS = 20;
export const PATCH_BATCH_MAX_PATH_BYTES = 8192;

const encoder = new TextEncoder();

function targetBytes(target: SessionChangeTarget): number {
  return encoder.encode(target.path).length + encoder.encode(target.oldPath ?? "").length;
}

/** Splits targets into request-sized batches, preserving order. */
export function planPatchBatches(targets: readonly SessionChangeTarget[]): SessionChangeTarget[][] {
  const batches: SessionChangeTarget[][] = [];
  let current: SessionChangeTarget[] = [];
  let bytes = 0;
  for (const target of targets) {
    const size = targetBytes(target);
    if (
      current.length > 0 &&
      (current.length >= PATCH_BATCH_MAX_PATHS || bytes + size > PATCH_BATCH_MAX_PATH_BYTES)
    ) {
      batches.push(current);
      current = [];
      bytes = 0;
    }
    current.push(target);
    bytes += size;
  }
  if (current.length > 0) batches.push(current);
  return batches;
}

interface PendingPatch {
  target: SessionChangeTarget;
  resolve: (patch: SessionChangePatch) => void;
  reject: (error: unknown) => void;
}

/**
 * Coalesces the diff list's per-file loads into batched requests (US-009.AC-2): every load
 * asked for in the same tick shares one request per batch. A file missing from the response
 * rejects, so the viewer shows its error state instead of hanging.
 */
export function createPatchBatcher(
  fetchBatch: (targets: SessionChangeTarget[]) => Promise<readonly SessionChangePatch[]>
) {
  let pending: PendingPatch[] = [];
  let scheduled = false;

  const flush = () => {
    scheduled = false;
    const queued = pending;
    pending = [];
    for (const batch of planPatchBatches(queued.map(entry => entry.target))) {
      const entries = queued.filter(entry => batch.includes(entry.target));
      fetchBatch(batch).then(
        patches => {
          const byPath = new Map(patches.map(patch => [patch.path, patch]));
          for (const entry of entries) {
            const patch = byPath.get(entry.target.path);
            if (patch) entry.resolve(patch);
            else entry.reject(new Error(`no patch for ${entry.target.path}`));
          }
        },
        (error: unknown) => {
          for (const entry of entries) entry.reject(error);
        }
      );
    }
  };

  return (target: SessionChangeTarget): Promise<SessionChangePatch> =>
    new Promise((resolve, reject) => {
      pending.push({ target, resolve, reject });
      if (scheduled) return;
      scheduled = true;
      queueMicrotask(flush);
    });
}

/** Quiet time after the last tool call before Changes refetches. */
export const CHANGES_REFRESH_DEBOUNCE_MS = 750;
/** A steady stream of tool calls still refreshes within 2 s (US-010.AC-1). */
export const CHANGES_REFRESH_MAX_WAIT_MS = 2_000;

const SETTLED_TOOL_STATES = new Set(["output-available", "output-error"]);

/** Tool calls that finished in the loaded transcript; each new one may have written files. */
export function settledToolCount(data: SessionTranscriptData | undefined): number {
  let count = 0;
  for (const entry of flattenTranscriptEntries(data)) {
    for (const part of entry.message.parts) {
      const record = part as { type: string; state?: unknown };
      const tool = record.type === "dynamic-tool" || record.type.startsWith("tool-");
      if (tool && typeof record.state === "string" && SETTLED_TOOL_STATES.has(record.state)) {
        count += 1;
      }
    }
  }
  return count;
}

export interface DebouncedRefreshOptions {
  /** Quiet time after the last signal. */
  delayMs: number;
  /** A steady stream of signals still refreshes this often (US-010.AC-1: within 2 s). */
  maxWaitMs: number;
  onRefresh: () => void;
}

/**
 * Debounces session activity into summary refetches: a burst of tool calls refreshes once,
 * and a continuous burst never starves the refresh past `maxWaitMs`.
 */
export function createDebouncedRefresh({ delayMs, maxWaitMs, onRefresh }: DebouncedRefreshOptions) {
  let timer: ReturnType<typeof setTimeout> | null = null;
  let firstSignalAt: number | null = null;

  const fire = () => {
    timer = null;
    firstSignalAt = null;
    onRefresh();
  };

  return {
    signal() {
      const now = Date.now();
      firstSignalAt ??= now;
      if (timer !== null) clearTimeout(timer);
      const remaining = Math.max(0, firstSignalAt + maxWaitMs - now);
      timer = setTimeout(fire, Math.min(delayMs, remaining));
    },
    cancel() {
      if (timer !== null) clearTimeout(timer);
      timer = null;
      firstSignalAt = null;
    },
  };
}

/**
 * The revision fence's client half (US-010.AC-3). The first stale detail read refetches the
 * summary once (`refetching`); stale reads still in flight from the old summary are ignored.
 * Once the fresh summary lands (`verifying`), a clean patch returns to `clean`, and another
 * stale read stops at `failed` ("Couldn't refresh · Retry") instead of looping.
 */
export type StaleFenceState = "clean" | "refetching" | "verifying" | "failed";

export type StaleFenceEvent = "stale" | "summary" | "patch" | "retry";

export function nextStaleFence(state: StaleFenceState, event: StaleFenceEvent): StaleFenceState {
  switch (event) {
    case "stale":
      if (state === "clean") return "refetching";
      return state === "verifying" ? "failed" : state;
    case "summary":
      return state === "refetching" ? "verifying" : state;
    case "patch":
      return state === "verifying" ? "clean" : state;
    case "retry":
      return "refetching";
  }
}
