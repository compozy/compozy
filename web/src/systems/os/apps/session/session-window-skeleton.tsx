import type { ReactNode } from "react";

import { SessionWindowSkeletonShape } from "./session-window-skeleton-shape";

/** The pane alone: the window's live rail already sits beside it. */
function SessionPaneOnly({ children }: { sidebar: ReactNode; children: ReactNode }) {
  return children;
}

/**
 * The session pane while its session resolves: a transcript over a composer card, in the frame
 * the loaded pane fills. `SessionWindowLoadingFrame` sets it beside the window's live rail, so a
 * row pick never trades the rail for placeholder rows (#1923). The frame shown before the
 * window's code has loaded is `SessionWindowFallback`, rail placeholder included.
 */
export function SessionWindowSkeleton() {
  return <SessionWindowSkeletonShape Layout={SessionPaneOnly} sidebar={null} />;
}
