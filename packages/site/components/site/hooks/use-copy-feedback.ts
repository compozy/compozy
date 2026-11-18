"use client";

import { useEffect, useRef, useState } from "react";

export type CopyState = "idle" | "copied" | "failed";

/** Matches `CopyIconButton` in `@compozy/ui`: the result shows this long, then the control resets. */
export const COPY_FEEDBACK_MS = 1500;

/**
 * Copy-to-clipboard state for a labelled copy control (`CopyIconButton` covers the icon-only
 * ones): reports success or failure, then returns to idle. A newer click supersedes an older one.
 */
export function useCopyFeedback(value: string) {
  const [state, setState] = useState<CopyState>("idle");
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const request = useRef(0);
  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    []
  );
  async function copy() {
    request.current += 1;
    const id = request.current;
    if (timer.current) clearTimeout(timer.current);
    let copied = false;
    try {
      await navigator.clipboard.writeText(value);
      copied = true;
    } catch {
      copied = false;
    }
    if (id !== request.current) return;
    setState(copied ? "copied" : "failed");
    timer.current = setTimeout(() => setState("idle"), COPY_FEEDBACK_MS);
  }
  return { state, copy };
}
