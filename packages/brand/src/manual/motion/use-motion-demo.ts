// When a chapter 09 demo plays: once when its stage is 60% in view, again on replay. With
// prefers-reduced-motion it never plays; the stage keeps its final frame and the card says so.
import { useCallback, useEffect, useRef, useState } from "react";

import { playDemo, resetDemo, type DemoId } from "./demos";

export const REDUCED_MOTION_QUERY = "(prefers-reduced-motion: reduce)";

export const prefersReducedMotion = () =>
  typeof window !== "undefined" && window.matchMedia(REDUCED_MOTION_QUERY).matches;

export function useMotionDemo<T extends HTMLElement = HTMLDivElement>(id: DemoId) {
  const stageRef = useRef<T>(null);
  const [reducedShown, setReducedShown] = useState(false);

  const play = useCallback(() => {
    const root = stageRef.current;
    if (!root) return;
    if (prefersReducedMotion()) {
      resetDemo(root);
      setReducedShown(true);
      return;
    }
    setReducedShown(false);
    playDemo(id, root);
  }, [id]);

  useEffect(() => {
    const root = stageRef.current;
    if (!root) return;
    const io = new IntersectionObserver(
      entries => {
        if (!entries.some(e => e.isIntersecting)) return;
        io.disconnect();
        play();
      },
      { threshold: 0.6 }
    );
    io.observe(root);
    return () => {
      io.disconnect();
      // stops pending timers and frames of the current run and drops its animations
      resetDemo(root);
    };
  }, [play]);

  return { stageRef, play, reducedShown };
}

/** Plays an endless WAAPI loop on mount unless reduced motion is on; cancels it on unmount. */
export function useLoopAnimation<T extends HTMLElement>(
  keyframes: Keyframe[],
  options: KeyframeAnimationOptions
) {
  const ref = useRef<T>(null);
  const kf = JSON.stringify(keyframes);
  const opts = JSON.stringify(options);
  useEffect(() => {
    const el = ref.current;
    if (!el || prefersReducedMotion()) return;
    const a = el.animate(
      JSON.parse(kf) as Keyframe[],
      JSON.parse(opts) as KeyframeAnimationOptions
    );
    return () => a.cancel();
  }, [kf, opts]);
  return ref;
}
