import { useReducedMotionConfig } from "motion/react";

import { MOTION_DURATION_SLOW, MOTION_EASE_OUT } from "../../lib/motion";

/**
 * Portaled popup/overlay nodes are not direct AnimatePresence children, so enter
 * `initial` values can stick forever. Skip enter motion (`initial={false}`) and
 * only animate exit; honor reduced-motion by zeroing exit duration.
 */
export function useDialogMotionTransition() {
  const reduced = useReducedMotionConfig();
  return { duration: reduced ? 0 : MOTION_DURATION_SLOW, ease: MOTION_EASE_OUT };
}
