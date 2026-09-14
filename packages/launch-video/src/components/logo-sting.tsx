import { Easing, interpolate, useCurrentFrame, useVideoConfig } from "remotion";

import { LOCKUP, SymbolGlyph, Wordmark } from "@/brand/logo";
import { EASE_OUT, color } from "@/brand/tokens";

/**
 * The brand sting: the mark settles, then the wordmark wipes open beside it.
 *
 * logo-bumper.md calls for a single-mark `logo-sting`, which the remocn catalog does not have —
 * its `logo-enter` is a multi-chip partner cluster, and the text-based reveals (`tracking-in`,
 * `kinetic-center-build`) cannot be used because DESIGN.md §8 forbids setting the wordmark in a
 * font. So the mark and the official wordmark geometry are choreographed directly here.
 */
export function LogoSting({ height }: { height: number }) {
  const { fps } = useVideoConfig();
  const frame = (useCurrentFrame() * 30) / fps;
  const scale = height / LOCKUP.artboardHeight;

  return (
    <div style={{ display: "flex", alignItems: "flex-start", height }}>
      <div
        style={{
          opacity: interpolate(frame, [0, 14], [0, 1], {
            extrapolateLeft: "clamp",
            extrapolateRight: "clamp",
            easing: Easing.bezier(...EASE_OUT),
          }),
          scale: interpolate(frame, [0, 30], [0.88, 1], {
            extrapolateLeft: "clamp",
            extrapolateRight: "clamp",
            easing: Easing.bezier(...EASE_OUT),
            output: "perceptual-scale",
          }),
        }}
      >
        <SymbolGlyph size={LOCKUP.symbolSize * scale} />
      </div>
      <div
        style={{
          marginLeft: LOCKUP.gap * scale,
          marginTop: LOCKUP.wordmarkTop * scale,
          clipPath: `inset(0 ${interpolate(frame, [8, 40], [100, 0], {
            extrapolateLeft: "clamp",
            extrapolateRight: "clamp",
            easing: Easing.bezier(...EASE_OUT),
          })}% 0 0)`,
          translate: `${interpolate(frame, [8, 40], [-14, 0], {
            extrapolateLeft: "clamp",
            extrapolateRight: "clamp",
            easing: Easing.bezier(...EASE_OUT),
          })}px 0`,
        }}
      >
        <Wordmark height={LOCKUP.wordmarkHeight * scale} fill={color.fg} />
      </div>
    </div>
  );
}
