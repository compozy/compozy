"use client";

import { useStage } from "@/components/landing/hooks/use-stage";
import { useScrollTransform } from "@/components/landing/hooks/use-scroll-transform";

/** The pile of terminals fades out as the single board takes its place. */
export function useProblemStage() {
  const { ref, progress } = useStage();
  const tangleOpacity = useScrollTransform(progress, [0.3, 0.6], [0.62, 0]);
  const tangleScale = useScrollTransform(progress, [0, 0.6], [1, 1.22]);
  const orderOpacity = useScrollTransform(progress, [0.45, 0.7], [0, 0.8]);
  const boardOpacity = useScrollTransform(progress, [0.48, 0.66], [0, 1]);
  const boardScale = useScrollTransform(progress, [0.48, 0.8], [0.74, 1]);
  const beforeOpacity = useScrollTransform(progress, [0.3, 0.46], [1, 0]);
  const afterOpacity = useScrollTransform(progress, [0.56, 0.68], [0, 1]);
  const lineOpacity = useScrollTransform(progress, [0.78, 0.9], [0, 1]);
  const lineY = useScrollTransform(progress, [0.78, 0.9], [24, 0]);
  return {
    ref,
    progress,
    tangleOpacity,
    tangleScale,
    orderOpacity,
    boardOpacity,
    boardScale,
    beforeOpacity,
    afterOpacity,
    lineOpacity,
    lineY,
  };
}
