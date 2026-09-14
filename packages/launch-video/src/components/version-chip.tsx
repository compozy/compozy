import { Easing, interpolate, useCurrentFrame, useVideoConfig } from "remotion";

import { FONT_MONO } from "@/brand/fonts";
import { color, radius, tracking, weight } from "@/brand/tokens";

/**
 * The release version, as a mono chip on a hairline.
 *
 * remocn's `micro-scale-fade` owns the right entrance for a quiet supporting label, but it renders
 * a bare centered span with sans type and its own tight tracking — it cannot carry chip chrome or
 * the +0.06em mono rail tracking the metadata role uses. The motion curve below is its curve
 * (18f, 0.96 → 1, cubic-bezier(0.32, 0.72, 0, 1)) so the pacing still matches the rest of the scene.
 */
/** Laid-out height of the chip: 0.5em of padding either side of a 1em line, plus the hairline. */
export function versionChipHeight(fontSize: number) {
  return fontSize * 2 + 2;
}

export function VersionChip({ version, fontSize }: { version: string; fontSize: number }) {
  const { fps } = useVideoConfig();
  const frame = (useCurrentFrame() * 30) / fps;
  const easing = Easing.bezier(0.32, 0.72, 0, 1);

  return (
    <div
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: fontSize * 0.62,
        paddingInline: fontSize * 0.84,
        paddingBlock: fontSize * 0.5,
        borderRadius: radius.chip,
        border: `1px solid ${color.line}`,
        backgroundColor: "rgba(255, 255, 255, 0.03)",
        opacity: interpolate(frame, [0, 18], [0, 1], {
          extrapolateLeft: "clamp",
          extrapolateRight: "clamp",
          easing,
        }),
        scale: interpolate(frame, [0, 18], [0.96, 1], {
          extrapolateLeft: "clamp",
          extrapolateRight: "clamp",
          easing,
          output: "perceptual-scale",
        }),
      }}
    >
      <span
        style={{
          width: fontSize * 0.34,
          height: fontSize * 0.34,
          borderRadius: "50%",
          backgroundColor: color.accent,
        }}
      />
      <span
        style={{
          fontFamily: FONT_MONO,
          fontSize,
          fontWeight: weight.medium,
          letterSpacing: tracking.wide,
          color: color.fg,
          lineHeight: 1,
        }}
      >
        {version}
      </span>
    </div>
  );
}
