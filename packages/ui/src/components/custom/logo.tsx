import type { SVGProps } from "react";

import { cn } from "../../lib/utils";
import { LOGO_SYMBOL_PNG_256 } from "./logo-symbol.generated";
import { LOGO_LOCKUP, LOGO_WORDMARK } from "./logo-wordmark.generated";

export type LogoVariant = "logo" | "symbol" | "lettering";

export interface LogoProps extends Omit<SVGProps<SVGSVGElement>, "children"> {
  variant?: LogoVariant;
  label?: string;
  decorative?: boolean;
}

const SYMBOL_SIZE = 256;

const LOGO_VIEWBOX: Record<LogoVariant, string> = {
  logo: LOGO_LOCKUP.viewBox,
  symbol: `0 0 ${SYMBOL_SIZE} ${SYMBOL_SIZE}`,
  lettering: LOGO_WORDMARK.viewBox,
};

const LOGO_SIZE_CLASS: Record<LogoVariant, string> = {
  logo: "h-8 w-auto",
  symbol: "size-8",
  lettering: "h-8 w-auto",
};

interface SymbolArtworkProps {
  x?: number;
  y?: number;
  size?: number;
}

// TODO(vector): replace with hand-drawn symbol vector
// The symbol ships as a raster until the vector exists (ADR-004); the gradient does not recolour.
function SymbolArtwork({ x = 0, y = 0, size = SYMBOL_SIZE }: SymbolArtworkProps) {
  return (
    <image
      href={LOGO_SYMBOL_PNG_256}
      x={x}
      y={y}
      width={size}
      height={size}
      preserveAspectRatio="xMidYMid meet"
    />
  );
}

// The wordmark follows the surrounding text colour, so the lockup reads on dark and light.
function LetteringArtwork() {
  return <path d={LOGO_WORDMARK.d} fill="currentColor" />;
}

function Logo({
  variant = "logo",
  label = "Compozy",
  decorative = false,
  className,
  role,
  "aria-label": ariaLabel,
  ...props
}: LogoProps) {
  return (
    <svg
      {...props}
      data-slot="logo"
      data-variant={variant}
      xmlns="http://www.w3.org/2000/svg"
      viewBox={LOGO_VIEWBOX[variant]}
      fill="none"
      preserveAspectRatio="xMidYMid meet"
      focusable="false"
      role={decorative ? undefined : (role ?? "img")}
      aria-label={decorative ? undefined : (ariaLabel ?? label)}
      aria-hidden={decorative ? true : undefined}
      className={cn("inline-block shrink-0", LOGO_SIZE_CLASS[variant], className)}
    >
      {variant === "logo" && (
        <>
          <SymbolArtwork
            x={LOGO_LOCKUP.symbol.x}
            y={LOGO_LOCKUP.symbol.y}
            size={LOGO_LOCKUP.symbol.size}
          />
          <g transform={`translate(${LOGO_LOCKUP.wordmark.x} ${LOGO_LOCKUP.wordmark.y})`}>
            <LetteringArtwork />
          </g>
        </>
      )}
      {variant === "symbol" && <SymbolArtwork />}
      {variant === "lettering" && <LetteringArtwork />}
    </svg>
  );
}

export { Logo };
