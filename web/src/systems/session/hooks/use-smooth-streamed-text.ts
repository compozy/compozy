import { useEffect, useRef, useState } from "react";

import {
  createSmoothRevealState,
  isAppendOnlyUpdate,
  type SmoothRevealState,
  stepSmoothReveal,
} from "../lib/session-smooth-reveal";

/**
 * Smooth reveal of streamed prose (ADR-008). While `animate` holds, the
 * returned text trails the real text at a cadence that tracks arrival and
 * snaps to the full text the moment the stream settles or the text is
 * rewritten. When `animate` is false (stream done, reduced motion, the
 * client-local toggle off, no animation frames available) the real text is
 * returned as-is — presentation only, never a delay of finished content.
 */
export function useSmoothStreamedText(text: string, animate: boolean): string {
  const frames = typeof requestAnimationFrame === "function";
  const active = animate && frames;
  const [revealed, setRevealed] = useState(text);
  const targetRef = useRef(text);
  const stateRef = useRef<SmoothRevealState>(createSmoothRevealState(text.length));
  const emittedRef = useRef(text.length);
  const frameRef = useRef<number | null>(null);
  const tickRef = useRef<() => void>(() => undefined);

  useEffect(() => {
    tickRef.current = () => {
      frameRef.current = null;
      const target = targetRef.current;
      const step = stepSmoothReveal(
        stateRef.current,
        performance.now(),
        target.length,
        emittedRef.current
      );
      if (step.emitCount !== null) {
        emittedRef.current = step.emitCount;
        setRevealed(step.emitCount >= target.length ? target : target.slice(0, step.emitCount));
      }
      if (!step.done) {
        frameRef.current = requestAnimationFrame(() => tickRef.current());
      }
    };
  });

  useEffect(() => {
    const previous = targetRef.current;
    targetRef.current = text;
    if (!active || !isAppendOnlyUpdate(previous, text)) {
      // Snap: stream end, reduced motion, toggle off, or a rewrite.
      if (frameRef.current !== null) cancelAnimationFrame(frameRef.current);
      frameRef.current = null;
      stateRef.current = createSmoothRevealState(text.length);
      emittedRef.current = text.length;
      setRevealed(text);
      return;
    }
    if (text.length > stateRef.current.shown && frameRef.current === null) {
      frameRef.current = requestAnimationFrame(() => tickRef.current());
    }
  }, [active, text]);

  useEffect(() => {
    return () => {
      if (frameRef.current !== null) cancelAnimationFrame(frameRef.current);
      frameRef.current = null;
    };
  }, []);

  return active ? revealed : text;
}
