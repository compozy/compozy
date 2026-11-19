import { useEffect, useRef, useState } from "react";

export type CopyState = "idle" | "copied" | "failed";

const COPY_FEEDBACK_MS = 1600;

/**
 * A clipboard write that reports its outcome for a short window. The state
 * drives the icon and label swap and the polite live-region text, so a copy is
 * confirmed to sighted and screen-reader users alike.
 */
export function useCopyFeedback(): { state: CopyState; copy: (value: string) => Promise<void> } {
  const [state, setState] = useState<CopyState>("idle");
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    },
    []
  );

  const copy = async (value: string) => {
    let next: CopyState = "copied";
    try {
      await navigator.clipboard.writeText(value);
    } catch {
      next = "failed";
    }
    if (timerRef.current) clearTimeout(timerRef.current);
    setState(next);
    timerRef.current = setTimeout(() => setState("idle"), COPY_FEEDBACK_MS);
  };

  return { state, copy };
}
