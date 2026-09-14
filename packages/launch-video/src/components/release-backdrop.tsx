import { AbsoluteFill, useVideoConfig } from "remotion";

import { color } from "@/brand/tokens";

/**
 * The scene canvas: --wallpaper-ember over --wallpaper-grid, straight from tokens.css.
 *
 * Deliberately static. logo-bumper choreography ends on a dead-still hold so an editor can trim
 * cleanly, and DESIGN.md §6 rules out motion that is not doing a job — a drifting backdrop would
 * break both. Radii scale with composition width so the same plate reads across 16:9, 1:1 and 9:16.
 */
export function ReleaseBackdrop() {
  const { width } = useVideoConfig();
  const emberSize = `${Math.round(width * 0.57)}px ${Math.round(width * 0.36)}px`;
  const tealSize = `${Math.round(width * 0.52)}px ${Math.round(width * 0.33)}px`;

  return (
    <AbsoluteFill
      style={{
        backgroundColor: color.canvas,
        backgroundImage: [
          `radial-gradient(${emberSize} at 12% 108%, ${color.accentTintStrong}, transparent 68%)`,
          `radial-gradient(${tealSize} at 92% -10%, rgba(34, 85, 85, 0.38), transparent 70%)`,
          `radial-gradient(1.5px 1.5px at 50% 50%, ${color.gridDot} 40%, transparent 41%)`,
          `linear-gradient(180deg, ${color.canvasSoft}, ${color.rail} 62%)`,
        ].join(", "),
        backgroundSize: "auto, auto, 26px 26px, auto",
      }}
    />
  );
}
