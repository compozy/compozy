import type { ReactNode } from "react";

import { SessionWindowSkeletonShape } from "./session-window-skeleton-shape";

/** The pane alone: the window's live rail already frames it. */
function SessionPaneOnly({ children }: { sidebar: ReactNode; children: ReactNode }) {
  return children;
}

/**
 * The session pane while its session resolves: a transcript over a composer card, in the frame
 * the loaded pane fills. It mounts inside the window's live rail (`SessionWindowRail`), which
 * stays put across a session switch (#1923), so only the pane stands in. The frame shown before
 * the window's code has loaded is `SessionWindowFallback`, rail placeholder included.
 */
export function SessionWindowSkeleton() {
  return <SessionWindowSkeletonShape Layout={SessionPaneOnly} sidebar={null} />;
}
