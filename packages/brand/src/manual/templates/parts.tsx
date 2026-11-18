// Building blocks shared by the kit templates. Every template is sized ONLY in cqi (1cqi = 1% of
// the template width) inside a `container-type: inline-size` box, so the same markup renders at
// native size on /kit/$id and scaled inside a chapter <Frame>. Colours are @compozy/ui tokens.
// Class strings and inline-style helpers live in ./styles.
import { Logo } from "@compozy/ui";
import { cn } from "@compozy/ui/utils";
import type { CSSProperties, ReactNode } from "react";

import { productShotSrc, type ShotTheme } from "../art/product-pins";
import type { ProductShotName } from "../art/visuals";

/** Absolute overlay carrying a scrim. */
export function Scrim({ style, className }: { style: CSSProperties; className?: string }) {
  return <div aria-hidden className={cn("absolute inset-0", className)} style={style} />;
}

type MarkVariant = "dark" | "light" | "symbol" | "mono-white";

/**
 * The Compozy lockup inside a template, `size` cqi tall. dark/light/symbol compose the
 * @compozy/ui <Logo> (vector lettering in currentColor + the symbol raster); mono-white is the
 * one-colour vector file for pieces where the gradient cannot print.
 */
export function Mark({
  size = 2.6,
  variant = "dark",
  className,
}: {
  size?: number;
  variant?: MarkVariant;
  className?: string;
}) {
  const style: CSSProperties = { height: `${size}cqi`, width: "auto" };
  if (variant === "mono-white") {
    return (
      <img
        src="/brand/svg/compozy-lockup-mono-white.svg"
        alt="Compozy"
        className={cn("block", className)}
        style={style}
      />
    );
  }
  return (
    <Logo
      variant={variant === "symbol" ? "symbol" : "logo"}
      className={cn(
        "block",
        variant === "light" ? "text-rail" : "text-fg-strong",
        variant === "symbol" && "aspect-square",
        className
      )}
      style={style}
    />
  );
}

/** The full-size gradient symbol (618 px master) for avatars and large prints. */
export function SymbolArt({ width, className }: { width: number; className?: string }) {
  return (
    <img
      src="/brand/png/compozy-symbol-618.png"
      alt="Compozy"
      className={cn("block h-auto", className)}
      style={{ width: `${width}cqi` }}
    />
  );
}

interface ShotProps {
  name: ProductShotName;
  theme?: ShotTheme;
  className?: string;
  style?: CSSProperties;
}

/**
 * A captured product screen (ADR-005) as a flat window: the pixels come from the running web/
 * app against the demo seed; the template only places, frames and lights it. Width in cqi,
 * height from the capture's own ratio; place it with left/right/top/bottom in `style`.
 */
export function Shot({ name, theme = "dark", w, className, style }: ShotProps & { w: number }) {
  return (
    <img
      data-product-shot={name}
      src={productShotSrc(name, theme)}
      alt=""
      className={cn(
        "absolute block h-auto max-w-none border border-line-strong",
        "shadow-[0_2cqi_6cqi_color-mix(in_oklab,var(--color-black)_55%,transparent)]",
        className
      )}
      style={{ width: `${w}cqi`, borderRadius: `${Math.max(0.5, w / 60)}cqi`, ...style }}
    />
  );
}

/**
 * A framed panel (w × h cqi, in flow) holding one product capture. `fit="contain"` shows the
 * whole screen centred; `fit="cover"` crops it around the focal point (x, y in 0..1).
 */
export function ShotPanel({
  name,
  theme = "dark",
  w,
  h,
  pad = 1.5,
  radius = 1.6,
  fit = "contain",
  x = 0.5,
  y = 0.5,
  className,
}: ShotProps & {
  w: number;
  h: number;
  pad?: number;
  radius?: number;
  fit?: "contain" | "cover";
  x?: number;
  y?: number;
}) {
  return (
    <div
      className={cn("relative overflow-hidden border border-line-strong bg-canvas", className)}
      style={{ width: `${w}cqi`, height: `${h}cqi`, borderRadius: `${radius}cqi` }}
    >
      <img
        data-product-shot={name}
        src={productShotSrc(name, theme)}
        alt=""
        className="absolute block max-w-none"
        style={{
          inset: fit === "contain" ? `${pad}cqi` : 0,
          width: fit === "contain" ? `calc(100% - ${2 * pad}cqi)` : "100%",
          height: fit === "contain" ? `calc(100% - ${2 * pad}cqi)` : "100%",
          objectFit: fit,
          objectPosition: `${x * 100}% ${y * 100}%`,
          borderRadius: fit === "contain" ? `${radius / 2}cqi` : undefined,
        }}
      />
    </div>
  );
}

/** Volt button inside an ad, sized in cqi. The only volt element in the piece. */
export function Cta({
  label,
  size = 2.6,
  field = "cta",
}: {
  label: string;
  size?: number;
  field?: string;
}) {
  return (
    <span
      data-field={field}
      className="inline-flex items-center whitespace-nowrap bg-brand-volt font-sans font-medium text-fg-strong"
      style={{
        fontSize: `${size}cqi`,
        padding: `${size * 0.62}cqi ${size * 1.05}cqi`,
        borderRadius: `${size * 0.42}cqi`,
        lineHeight: 1,
      }}
    >
      {label}
    </span>
  );
}

/**
 * Scales px-sized product components (agent faces) with the template: the child renders at
 * `px` CSS pixels inside an SVG viewBox, and the SVG is `w` cqi wide.
 */
export function Scaled({ px, w, children }: { px: number; w: number; children: ReactNode }) {
  return (
    <svg
      viewBox={`0 0 ${px} ${px}`}
      className="block overflow-visible"
      style={{ width: `${w}cqi`, height: `${w}cqi` }}
      aria-hidden
    >
      <foreignObject x={0} y={0} width={px} height={px}>
        <div className="grid size-full place-items-center">{children}</div>
      </foreignObject>
    </svg>
  );
}
