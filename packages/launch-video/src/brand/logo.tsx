/**
 * Official CompozyOS logo geometry.
 *
 * Mirrors packages/ui/src/components/custom/logo.tsx: vector lettering plus the interim raster
 * symbol (ADR-004). DESIGN.md §8: the wordmark is always official SVG geometry and never a
 * `<text>` node or font fallback. The data lives in a package-local generated module because the
 * Remotion bundle does not pull in @compozy/ui; `bun run logos` in packages/brand refreshes it.
 *
 * The mark and the wordmark are exported separately so a scene can animate them independently;
 * LOCKUP carries the official artboard math that keeps them locked up correctly.
 */
import { LOGO_LOCKUP, LOGO_SYMBOL_PNG, LOGO_WORDMARK } from "./logo-assets.generated";

const [, , artboardWidth = 0, artboardHeight = 0] = LOGO_LOCKUP.viewBox.split(" ").map(Number);
const [, , wordmarkWidth = 0, wordmarkHeight = 0] = LOGO_WORDMARK.viewBox.split(" ").map(Number);

/** Official lockup artboard math, used to place the mark and wordmark apart. */
export const LOCKUP = {
  artboardWidth,
  artboardHeight,
  symbolSize: LOGO_LOCKUP.symbol.size,
  /** Gap between the mark's right edge and the wordmark's left edge, in artboard units. */
  gap: LOGO_LOCKUP.wordmark.x - LOGO_LOCKUP.symbol.x - LOGO_LOCKUP.symbol.size,
  wordmarkWidth,
  wordmarkHeight,
  /** Wordmark offset from the lockup top, in artboard units. */
  wordmarkTop: LOGO_LOCKUP.wordmark.y - LOGO_LOCKUP.symbol.y,
} as const;

interface SymbolGlyphProps {
  size: number;
}

// TODO(vector): replace with hand-drawn symbol vector
export function SymbolGlyph({ size }: SymbolGlyphProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 256 256" aria-hidden focusable="false">
      <image href={LOGO_SYMBOL_PNG} x={0} y={0} width={256} height={256} />
    </svg>
  );
}

interface WordmarkProps {
  height: number;
  fill: string;
}

export function Wordmark({ height, fill }: WordmarkProps) {
  const width = Math.round((height * LOCKUP.wordmarkWidth) / LOCKUP.wordmarkHeight);
  return (
    <svg
      width={width}
      height={height}
      viewBox={LOGO_WORDMARK.viewBox}
      aria-hidden
      focusable="false"
    >
      <path d={LOGO_WORDMARK.d} fill={fill} />
    </svg>
  );
}
