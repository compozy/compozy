import { useRef } from "react";

/**
 * The one synchronous in-flight admission a lifecycle host shares across its batch families
 * (lifecycle, Later and selection menu). A ref, not state: two batches invoked in the same
 * turn must see each other before React renders. Check `held()` and `hold()` in one turn.
 */
export interface SessionBatchAdmission {
  held: () => boolean;
  hold: () => void;
  release: () => void;
}

export function useSessionBatchAdmission(): SessionBatchAdmission {
  const running = useRef(false);
  return {
    held: () => running.current,
    hold: () => {
      running.current = true;
    },
    release: () => {
      running.current = false;
    },
  };
}
