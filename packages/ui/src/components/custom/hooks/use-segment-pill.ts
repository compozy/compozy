import {
  animate,
  useMotionValue,
  useReducedMotionConfig,
  useTransform,
  type AnimationPlaybackControls,
} from "motion/react";
import * as React from "react";

import { MOTION_DURATION_SLOW } from "../../../lib/motion";

/** Marks a sibling element as one of the segments the pill can rest on. */
export const SEGMENT_PILL_TARGET = "data-segment-pill-target";

/** The pill's full travel; a synchronized transition (a sliding panel) reuses it. */
export const SEGMENT_PILL_DURATION = MOTION_DURATION_SLOW;

// The leading edge stretches toward the target first, the trailing edge follows
// and settles a hair past it before relaxing: the "rubber" travel.
const LEAD_SHARE = 0.6;
const TRAIL_DELAY_SHARE = 0.3;
const EASE_STRETCH = [0.23, 1, 0.32, 1] as const;
const EASE_SETTLE = [0.34, 1.3, 0.64, 1] as const;

interface Edges {
  left: number;
  right: number;
}

/**
 * Drives a segment pill: measures the selected target among the host's
 * `SEGMENT_PILL_TARGET` children and animates the plate's edges toward it —
 * the leading edge first, the trailing edge after — as motion values only.
 * Reduced motion jumps; a resized host re-seats the plate without travel.
 */
export function useSegmentPill(index: number | null, duration: number) {
  const ref = React.useRef<HTMLSpanElement>(null);
  const reduced = useReducedMotionConfig() ?? false;
  const left = useMotionValue(0);
  const right = useMotionValue(0);
  const cap = useMotionValue(0);
  const opacity = useMotionValue(0);
  const shown = React.useRef<Edges | null>(null);
  const running = React.useRef<AnimationPlaybackControls[]>([]);

  // The 1px body overlaps each cap by half a pixel so no seam shows mid-travel.
  const bodyX = useTransform(() => left.get() + cap.get() - 0.5);
  const bodyScale = useTransform(() => Math.max(0, right.get() - left.get() - 2 * cap.get() + 1));
  const rightCapX = useTransform(() => right.get() - cap.get());

  React.useLayoutEffect(() => {
    const host = ref.current?.parentElement;
    if (!host) return;
    const stop = () => {
      for (const controls of running.current) controls.stop();
      running.current = [];
    };
    const measure = (): Edges | null => {
      if (index === null) return null;
      const target = host.querySelectorAll<HTMLElement>(`:scope > [${SEGMENT_PILL_TARGET}]`)[index];
      if (!target) return null;
      cap.set(target.offsetHeight / 2);
      return { left: target.offsetLeft, right: target.offsetLeft + target.offsetWidth };
    };
    const jump = (edges: Edges) => {
      stop();
      left.jump(edges.left);
      right.jump(edges.right);
    };

    const next = measure();
    const from = shown.current;
    shown.current = next;
    if (next === null) {
      stop();
      running.current = [animate(opacity, 0, { duration: reduced ? 0 : duration / 2 })];
      return;
    }
    if (from === null || reduced) {
      jump(next);
      opacity.jump(1);
    } else {
      stop();
      const forward = next.left + next.right >= from.left + from.right;
      const [lead, leadTo, trail, trailTo] = forward
        ? [right, next.right, left, next.left]
        : [left, next.left, right, next.right];
      running.current = [
        animate(lead, leadTo, { duration: duration * LEAD_SHARE, ease: EASE_STRETCH }),
        animate(trail, trailTo, {
          duration: duration * (1 - TRAIL_DELAY_SHARE),
          delay: duration * TRAIL_DELAY_SHARE,
          ease: EASE_SETTLE,
        }),
        animate(opacity, 1, { duration: duration / 2 }),
      ];
    }

    // Segment labels change width (live counts): the plate follows without travel.
    const observer = new ResizeObserver(() => {
      const edges = measure();
      if (edges === null || running.current.some(controls => controls.state === "running")) return;
      shown.current = edges;
      jump(edges);
    });
    observer.observe(host);
    return () => observer.disconnect();
  }, [index, duration, reduced, left, right, cap, opacity]);

  React.useEffect(
    () => () => {
      for (const controls of running.current) controls.stop();
    },
    []
  );

  return { ref, opacity, left, cap, bodyX, bodyScale, rightCapX };
}
