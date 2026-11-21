import { useEffect, useRef, type RefObject } from "react";

import type { SessionPayload } from "../types";

/**
 * Scrolls the current session's row into view once each time the current session changes —
 * a notification jump that lands in an existing Session window (#1927), a palette pick, a
 * sidebar click — only as far as needed (`block: "nearest"`), so a row already on screen
 * stays put. A row that is not rendered yet is revealed when the rows it arrives with render;
 * later refreshes of the same session never scroll the list back after the operator moved it.
 */
export function useSessionListRevealCurrent(
  contentRef: RefObject<HTMLElement | null>,
  currentSessionId: string | undefined,
  sessions: readonly SessionPayload[]
): void {
  const revealedRef = useRef<string | null>(null);
  useEffect(() => {
    const id = currentSessionId ?? "";
    if (id === "") {
      revealedRef.current = null;
      return;
    }
    if (revealedRef.current === id) return;
    const row = contentRef.current?.querySelector<HTMLElement>('[aria-current="true"]');
    if (!row) return;
    revealedRef.current = id;
    row.scrollIntoView({ block: "nearest" });
  }, [contentRef, currentSessionId, sessions]);
}
