"use client";

import { WorkerPoolContext } from "@pierre/diffs/react";
import type * as React from "react";

import type { CodeHighlightPoolOptions } from "./code-highlight-pool-store";
import { useCodeHighlightPool } from "./hooks/use-code-highlight-pool";

export interface CodeHighlightPoolProviderProps extends CodeHighlightPoolOptions {
  children?: React.ReactNode;
}

/**
 * Shares one highlight worker pool (2–4 workers) between every `CodeViewer`
 * and `DiffViewer` below it. The pool is ref-counted across providers and
 * terminates 30 s after the last one unmounts.
 */
function CodeHighlightPoolProvider({ children, workerFactory }: CodeHighlightPoolProviderProps) {
  const pool = useCodeHighlightPool(workerFactory);
  return <WorkerPoolContext value={pool}>{children}</WorkerPoolContext>;
}

export { CodeHighlightPoolProvider };
