import { useTransform } from "motion/react";

import { useStage } from "./use-stage";

/** The pile of terminals fades out as the single board takes its place. */
export function useProblemStage() {
  const { ref, progress } = useStage();
  const tangleOpacity = useTransform(progress, [0.3, 0.6], [0.62, 0]);
  const tangleScale = useTransform(progress, [0, 0.6], [1, 1.22]);
  const orderOpacity = useTransform(progress, [0.45, 0.7], [0, 0.8]);
  const boardOpacity = useTransform(progress, [0.48, 0.66], [0, 1]);
  const boardScale = useTransform(progress, [0.48, 0.8], [0.74, 1]);
  const beforeOpacity = useTransform(progress, [0.3, 0.46], [1, 0]);
  const afterOpacity = useTransform(progress, [0.56, 0.68], [0, 1]);
  const lineOpacity = useTransform(progress, [0.78, 0.9], [0, 1]);
  const lineY = useTransform(progress, [0.78, 0.9], [24, 0]);
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
