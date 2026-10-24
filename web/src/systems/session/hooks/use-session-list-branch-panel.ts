import { useEffect, useState } from "react";

import type { SessionThreadSegment } from "../lib/session-thread-branch";

// The fold's `duration-base` plus a frame: past it the folded list is out of sight.
const FOLD_SETTLE_MS = 200;

export interface SessionListBranchPanel {
  /** The list mounted in the fold; it outlives `segment` until the fold settles. */
  shown: SessionThreadSegment | null;
  /** 1 slides the incoming list in from the right, -1 from the left, 0 not at all. */
  direction: number;
}

const CLOSED: SessionListBranchPanel = { shown: null, direction: 0 };

/** The list a new segment mounts and the way it slides in; `null` keeps the folding list. */
function enteringPanel(
  panel: SessionListBranchPanel,
  segment: SessionThreadSegment | null,
  reduced: boolean
): SessionListBranchPanel | null {
  if (segment === null) return panel.shown !== null && reduced ? CLOSED : null;
  if (segment === panel.shown) return null;
  if (panel.shown === null) return { shown: segment, direction: 0 };
  return { shown: segment, direction: segment === "subagents" ? 1 : -1 };
}

/** Keeps the folding list mounted until the fold settles, then releases it. */
export function useSessionListBranchPanel(
  segment: SessionThreadSegment | null,
  reduced: boolean
): SessionListBranchPanel {
  const [panel, setPanel] = useState<SessionListBranchPanel>({ shown: segment, direction: 0 });
  const entering = enteringPanel(panel, segment, reduced);
  if (entering !== null) setPanel(entering);
  const settling = segment === null && panel.shown !== null;
  useEffect(() => {
    if (!settling) return;
    const timer = setTimeout(() => setPanel(CLOSED), FOLD_SETTLE_MS);
    return () => clearTimeout(timer);
  }, [settling]);
  return entering ?? panel;
}
