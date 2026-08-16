import { useEffect } from "react";

import { applyTitleBadge, resetTitleBadge } from "../lib/document-title";

/**
 * Mirrors the cross-workspace needs-you count into the document title.
 *
 * The title is an external system, so writing it is exactly the escape hatch
 * `useEffect` exists for. It updates while the tab is backgrounded — no focus,
 * no permission prompt, no setup — and clears to the plain title at zero.
 */
export function useDocumentTitleBadge(count: number): void {
  useEffect(() => {
    applyTitleBadge(count);
    return resetTitleBadge;
  }, [count]);
}
