import { useEffect, useState, type ReactNode } from "react";
import { AnimatePresence, m, useReducedMotionConfig, type Variants } from "motion/react";

import { SEGMENT_PILL_DURATION, cn } from "@compozy/ui";

import type { SessionThreadSegment } from "../../lib/session-thread-branch";

// The token ease-out; the slide shares the segment pill's duration so both land together.
const SLIDE_TRANSITION = { duration: SEGMENT_PILL_DURATION, ease: [0.22, 1, 0.36, 1] as const };
const INSTANT = { duration: 0 };
// The fold's `duration-base` plus a frame: past it the folded list is out of sight.
const FOLD_SETTLE_MS = 200;

/** `direction` 1 reveals the list to the right (subagents), -1 the one to the left. */
const SLIDE: Variants = {
  enter: (direction: number) => ({ x: `${direction * 100}%` }),
  center: { x: "0%" },
  exit: (direction: number) => ({ x: `${direction * -100}%` }),
};

export interface SessionListBranchProps {
  /** The list on show, or `null` while the branch is folded. */
  segment: SessionThreadSegment | null;
  render: (segment: SessionThreadSegment) => ReactNode;
}

/**
 * The fold under a session row holding one list at a time. Folding animates
 * the grid row; switching lists slides the outgoing one out (transform only)
 * while the incoming one takes the flow. A folded list unmounts once the fold
 * settles, so a closed branch neither renders rows nor keeps reads alive.
 * Reduced motion switches and folds instantly.
 */
export function SessionListBranch({ segment, render }: SessionListBranchProps) {
  const reduced = useReducedMotionConfig() ?? false;
  const [shown, setShown] = useState(segment);
  const [direction, setDirection] = useState(0);
  if (segment !== null && segment !== shown) {
    setDirection(shown === null ? 0 : segment === "subagents" ? 1 : -1);
    setShown(segment);
  } else if (segment === null && shown !== null && reduced) {
    setDirection(0);
    setShown(null);
  }
  const open = segment !== null;
  const settling = segment === null && shown !== null;
  useEffect(() => {
    if (!settling) return;
    const timer = setTimeout(() => {
      setDirection(0);
      setShown(null);
    }, FOLD_SETTLE_MS);
    return () => clearTimeout(timer);
  }, [settling]);

  return (
    <div
      inert={!open}
      data-slot="session-list-branch"
      data-segment={segment ?? undefined}
      className={cn(
        "grid transition-[grid-template-rows] duration-base ease-out motion-reduce:transition-none",
        open ? "grid-rows-[1fr]" : "grid-rows-[0fr]"
      )}
    >
      <div className="relative min-h-0 overflow-hidden">
        <AnimatePresence initial={false} custom={direction} mode="popLayout">
          {shown !== null ? (
            <m.div
              key={shown}
              custom={direction}
              variants={SLIDE}
              initial="enter"
              animate="center"
              exit="exit"
              transition={reduced ? INSTANT : SLIDE_TRANSITION}
            >
              {render(shown)}
            </m.div>
          ) : null}
        </AnimatePresence>
      </div>
    </div>
  );
}
