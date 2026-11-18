import { WorkerPoolManager } from "@pierre/diffs/worker";

import { PIERRE_CODE_THEMES } from "./pierre-theme";

/** Terminate the shared pool this long after its last consumer unmounts. */
export const CODE_HIGHLIGHT_POOL_IDLE_MS = 30_000;
const MIN_POOL_SIZE = 2;
const MAX_POOL_SIZE = 4;
// Small files highlight faster on the main thread than through a worker
// round trip, and render highlighted without a plain-text pass.
const MAIN_THREAD_MAX_CHARACTERS = 50_000;

export interface CodeHighlightPoolOptions {
  /**
   * Creates one highlight worker. Apps pass `createCodeHighlightWorker` from
   * `@compozy/ui/lib/code-highlight-worker`; without it, highlighting stays on the
   * main thread (fine for stories, tests, and small files).
   */
  workerFactory?: () => Worker;
}

interface SharedPool {
  consumers: number;
  idleTimer: ReturnType<typeof setTimeout> | undefined;
  pool: WorkerPoolManager;
}

let sharedPool: SharedPool | undefined;
const poolListeners = new Set<() => void>();

export function resolveCodeHighlightPoolSize(hardwareConcurrency: number | undefined): number {
  const cores = Math.max(1, hardwareConcurrency || MAX_POOL_SIZE);
  return Math.max(MIN_POOL_SIZE, Math.min(MAX_POOL_SIZE, Math.floor(cores / 2)));
}

export function subscribePoolChange(listener: () => void): () => void {
  poolListeners.add(listener);
  return () => {
    poolListeners.delete(listener);
  };
}

function emitPoolChange() {
  for (const listener of poolListeners) listener();
}

function armIdleTimer(entry: SharedPool, idleMs: number) {
  clearTimeout(entry.idleTimer);
  entry.idleTimer = setTimeout(() => {
    entry.idleTimer = undefined;
    if (entry.consumers > 0) return;
    entry.pool.terminate();
    if (sharedPool === entry) sharedPool = undefined;
    emitPoolChange();
  }, idleMs);
}

/**
 * The pool for the first render. Viewers bind their pool when they mount, so
 * it must exist before any provider subscribes; until one does, the idle timer
 * reclaims it (a render React threw away never leaks workers).
 */
export function ensureCodeHighlightPool(workerFactory: () => Worker): WorkerPoolManager {
  if (!sharedPool) {
    const pool = new WorkerPoolManager(
      {
        workerFactory,
        poolSize: resolveCodeHighlightPoolSize(globalThis.navigator?.hardwareConcurrency),
        mainThreadMaxCharacters: MAIN_THREAD_MAX_CHARACTERS,
        totalASTLRUCacheSize: 200,
      },
      {
        // One light/dark pair for the pool's lifetime; each viewer picks the
        // side through its own `themeType`, so theme switches never re-highlight.
        theme: PIERRE_CODE_THEMES,
        preferredHighlighter: "shiki-js",
        tokenizeMaxLineLength: 1_000,
      }
    );
    sharedPool = { consumers: 0, idleTimer: undefined, pool };
    armIdleTimer(sharedPool, CODE_HIGHLIGHT_POOL_IDLE_MS);
    void pool.initialize().catch(() => {
      // A failed init leaves the pool highlighting on the main thread.
    });
  }
  return sharedPool.pool;
}

export function acquirePool(workerFactory: () => Worker): SharedPool {
  ensureCodeHighlightPool(workerFactory);
  const entry = sharedPool as SharedPool;
  clearTimeout(entry.idleTimer);
  entry.idleTimer = undefined;
  entry.consumers += 1;
  return entry;
}

export function releasePool(entry: SharedPool, idleMs: number) {
  entry.consumers -= 1;
  if (entry.consumers > 0) return;
  armIdleTimer(entry, idleMs);
}

/** Test seam: current shared pool and its consumer count. */
export function inspectCodeHighlightPool(): { consumers: number; active: boolean } {
  return { consumers: sharedPool?.consumers ?? 0, active: sharedPool !== undefined };
}
