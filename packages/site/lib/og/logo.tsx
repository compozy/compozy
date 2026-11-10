// Mirrors packages/ui/src/components/custom/logo.tsx (vector lettering + raster symbol, ADR-004).
// The data lives in a site-local generated module because next/og (Satori) cannot resolve
// cross-package imports at build time; `bun run logos` in packages/brand refreshes it.
// Satori does not load `<image>` inside `<svg>`, so the symbol renders as an HTML `<img>` and the
// lockup composes it with the vector wordmark using the generated lockup geometry.
import { LOGO_LOCKUP, LOGO_SYMBOL_PNG, LOGO_WORDMARK } from "./logo-assets.generated";

function viewBoxSize(viewBox: string) {
  const [, , width = 1, height = 1] = viewBox.split(" ").map(Number);
  return { width, height };
}

const LOCKUP_BOX = viewBoxSize(LOGO_LOCKUP.viewBox);
const WORDMARK_BOX = viewBoxSize(LOGO_WORDMARK.viewBox);

interface SymbolGlyphProps {
  size: number;
}

// TODO(vector): replace with hand-drawn symbol vector
export function SymbolGlyph({ size }: SymbolGlyphProps) {
  return <img src={LOGO_SYMBOL_PNG} width={size} height={size} alt="" />;
}

interface LogoLockupProps {
  height: number;
  letteringFill?: string;
}

export function LogoLockup({ height, letteringFill = "#ececec" }: LogoLockupProps) {
  const scale = height / LOCKUP_BOX.height;
  const { symbol, wordmark } = LOGO_LOCKUP;
  const wordmarkHeight = WORDMARK_BOX.height * scale;
  const wordmarkWidth = WORDMARK_BOX.width * scale;
  return (
    <div
      style={{
        display: "flex",
        position: "relative",
        width: `${Math.round(LOCKUP_BOX.width * scale)}px`,
        height: `${height}px`,
      }}
    >
      <div
        style={{
          display: "flex",
          position: "absolute",
          left: `${symbol.x * scale}px`,
          top: `${symbol.y * scale}px`,
        }}
      >
        <SymbolGlyph size={symbol.size * scale} />
      </div>
      <svg
        width={wordmarkWidth}
        height={wordmarkHeight}
        viewBox={LOGO_WORDMARK.viewBox}
        aria-hidden
        focusable="false"
        style={{
          position: "absolute",
          left: `${wordmark.x * scale}px`,
          top: `${wordmark.y * scale}px`,
        }}
      >
        <path d={LOGO_WORDMARK.d} fill={letteringFill} />
      </svg>
    </div>
  );
}
