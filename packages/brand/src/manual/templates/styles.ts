// Class strings and inline-style helpers shared by the kit templates (sized in cqi, coloured by
// @compozy/ui tokens). Components live in ./parts.
import type { CSSProperties } from "react";

/** The ink ground every dark template starts from. */
export const ink = "absolute inset-0 overflow-hidden bg-rail text-fg";
/** Display headline: Instrument Sans 400, tight tracking (Foundations → Type). */
export const display = "m-0 font-display font-normal tracking-[-0.03em]";
/** Small uppercase mono line. */
export const monoUp = "m-0 font-mono uppercase";

/**
 * A fade from a surface token to transparent, for text that sits over art.
 *   scrim("90deg", [[0, 1], [40, 0.85], [72, 0]]) → solid at 0%, 85% at 40%, clear at 72%
 */
export function scrim(
  angle: string,
  stops: readonly (readonly [at: number, alpha: number])[],
  surface = "--color-rail"
): CSSProperties {
  const parts = stops.map(
    ([at, alpha]) =>
      `color-mix(in oklab, var(${surface}) ${Math.round(alpha * 100)}%, transparent) ${at}%`
  );
  return { backgroundImage: `linear-gradient(${angle}, ${parts.join(", ")})` };
}

/** The brand gradient (45deg, sky → cyan → glint) as a fill. */
export const gradientFill: CSSProperties = { backgroundImage: "var(--gradient-brand)" };
/** The brand gradient clipped to text. */
export const gradientText: CSSProperties = {
  backgroundImage: "var(--gradient-brand)",
  WebkitBackgroundClip: "text",
  backgroundClip: "text",
  color: "transparent",
};

/** cqi helper for inline styles. */
export const cq = (n: number) => `${n}cqi`;
