/** Quiet window after the last frame before the worktree scope rereads once. */
export const WORKTREE_REREAD_COALESCE_MS = 150;
/** Longest a reread waits while frames keep landing inside the quiet window. */
export const WORKTREE_REREAD_MAX_WAIT_MS = 1000;

interface CoalescedReread {
  request: () => void;
  /** Pays a reread still waiting for its quiet window, so no frame is lost with the connection. */
  flush: () => void;
}

/**
 * Collapses a burst of frames into one scope reread. The first connection
 * replays the retained history (hundreds of frames in under a second) and every
 * lifecycle frame asks for the same four reads, so rereading per frame repeats
 * the same requests once per frame and cancels its own in-flight fetches on
 * each repeat. Each frame restarts the quiet window; the ceiling is armed by
 * the first frame only, so a steady trickle of frames cannot defer the reread
 * past it.
 */
export function createCoalescedReread(
  reread: () => void,
  delayMs: number,
  maxWaitMs: number
): CoalescedReread {
  let quiet: ReturnType<typeof setTimeout> | undefined;
  let ceiling: ReturnType<typeof setTimeout> | undefined;
  const fire = () => {
    if (quiet !== undefined) clearTimeout(quiet);
    if (ceiling !== undefined) clearTimeout(ceiling);
    quiet = undefined;
    ceiling = undefined;
    reread();
  };
  return {
    request: () => {
      if (quiet !== undefined) clearTimeout(quiet);
      quiet = setTimeout(fire, delayMs);
      if (ceiling === undefined) ceiling = setTimeout(fire, maxWaitMs);
    },
    flush: () => {
      if (quiet === undefined && ceiling === undefined) return;
      fire();
    },
  };
}
