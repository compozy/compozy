"use client";

import type { WorkerPoolManager } from "@pierre/diffs/worker";
import * as React from "react";

import {
  acquirePool,
  CODE_HIGHLIGHT_POOL_IDLE_MS,
  ensureCodeHighlightPool,
  releasePool,
  subscribePoolChange,
} from "../code-highlight-pool-store";

/** Acquires the shared pool for the component's lifetime; `undefined` without a factory. */
export function useCodeHighlightPool(
  workerFactory: (() => Worker) | undefined
): WorkerPoolManager | undefined {
  const factoryRef = React.useRef(workerFactory);
  const enabled = workerFactory !== undefined;
  const subscribe = React.useCallback(
    (onChange: () => void) => {
      const unsubscribe = subscribePoolChange(onChange);
      const factory = factoryRef.current;
      const entry = enabled && factory ? acquirePool(factory) : undefined;
      if (entry) onChange();
      return () => {
        unsubscribe();
        if (entry) releasePool(entry, CODE_HIGHLIGHT_POOL_IDLE_MS);
      };
    },
    [enabled]
  );
  return React.useSyncExternalStore(
    subscribe,
    () => (workerFactory ? ensureCodeHighlightPool(workerFactory) : undefined),
    () => undefined
  );
}
