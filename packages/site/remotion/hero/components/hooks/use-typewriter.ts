import { useCurrentFrame } from "remotion";

/**
 * Slice-based typewriter. Returns the visible prefix of `text` at the
 * current frame relative to `startFrame`. Never animate per-character
 * opacity — string slicing only, per the remotion-best-practices skill.
 */
export function useTypewriter(
  text: string,
  startFrame: number,
  durationInFrames: number
): { visible: string; done: boolean; progress: number } {
  const frame = useCurrentFrame();
  const raw = (frame - startFrame) / Math.max(durationInFrames, 1);
  const progress = Math.max(0, Math.min(1, raw));
  const visibleChars = Math.floor(progress * text.length);
  return {
    visible: text.slice(0, visibleChars),
    done: progress >= 1,
    progress,
  };
}
