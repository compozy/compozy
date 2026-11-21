import { useEffect, type RefObject } from "react";

import type { SessionLifecycleActionHandlers } from "./use-session-lifecycle-actions";
import { useSessionListPin } from "./use-session-pin-actions";

function currentRow(root: HTMLElement, sessionId: string): HTMLElement | null {
  for (const row of root.querySelectorAll<HTMLElement>('[data-slot="session-row-target"]')) {
    if (row.dataset.sessionId === sessionId) return row;
  }
  return null;
}

/**
 * Scrolls the current session's row into view once each time the current session changes —
 * a notification jump that lands in an existing Session window (#1927), a palette pick, a
 * sidebar click — only as far as needed (`block: "nearest"`), so a row already on screen
 * stays put. A row that is not rendered yet (a page, the Pinned section or a group still
 * loading) is revealed when it mounts; once revealed, refreshes of the same session never
 * scroll the list back after the operator moved it.
 */
export function useSessionListRevealCurrent(
  contentRef: RefObject<HTMLElement | null>,
  currentSessionId: string | undefined
): void {
  useEffect(() => {
    const root = contentRef.current;
    const sessionId = currentSessionId ?? "";
    if (root === null || sessionId === "") return;
    const reveal = () => {
      const row = currentRow(root, sessionId);
      if (row === null) return false;
      row.scrollIntoView({ block: "nearest" });
      return true;
    };
    if (reveal()) return;
    const observer = new MutationObserver(() => {
      if (reveal()) observer.disconnect();
    });
    observer.observe(root, { childList: true, subtree: true });
    return () => observer.disconnect();
  }, [contentRef, currentSessionId]);
}

/** The list's content root: its Pin to top actions and the current row's reveal share one ref. */
export function useSessionListContent(
  hostActions: SessionLifecycleActionHandlers,
  testIdPrefix: string,
  currentSessionId: string | undefined
) {
  const content = useSessionListPin(hostActions, testIdPrefix);
  useSessionListRevealCurrent(content.contentRef, currentSessionId);
  return content;
}
