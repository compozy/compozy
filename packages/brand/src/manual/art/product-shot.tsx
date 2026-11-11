// Frame around one captured product screen (ADR-005): the pixels come from the running web/ app
// against the demo seed (export:product-shots); the manual only adds perspective, crop and light.
//   perspective: the screen tilts back toward its focal point and fades into the band, lit from
//                behind by the brand volt (covers, backgrounds).
//   flat:        a straight crop, for inline figures that must read at a glance.
import { cn } from "@compozy/ui/utils";

import { productShotSrc, type ShotTheme } from "./product-pins";
import type { ProductShotName } from "./visuals";

interface ProductShotProps {
  name: ProductShotName;
  theme: ShotTheme;
  /** focal point 0..1: where the screen sits (perspective) or which part stays in the crop (flat) */
  x: number;
  y: number;
  mode: "perspective" | "flat";
  alt?: string;
}

const pct = (n: number) => `${Math.round(n * 100)}%`;

export function ProductShot({ name, theme, x, y, mode, alt = "" }: ProductShotProps) {
  const src = productShotSrc(name, theme);
  if (mode === "flat") {
    return (
      <img
        data-product-shot={name}
        src={src}
        alt={alt}
        decoding="async"
        loading="lazy"
        className="absolute inset-0 size-full object-cover"
        style={{ objectPosition: `${pct(x)} ${pct(y)}` }}
      />
    );
  }
  return (
    <div data-product-shot={name} className="absolute inset-0 overflow-hidden">
      {/* light: a soft volt bloom behind the screen */}
      <div
        aria-hidden
        className="absolute size-[90%] -translate-1/2 rounded-full bg-[radial-gradient(closest-side,color-mix(in_oklab,var(--color-brand-volt)_26%,transparent),transparent)] blur-2xl"
        style={{ left: pct(x), top: pct(y) }}
      />
      <img
        src={src}
        alt={alt}
        decoding="async"
        loading="lazy"
        className={cn(
          "absolute w-[min(86%,150cqh)] max-w-none rounded-[1.2cqw] border border-(--b-line,var(--color-line))",
          "shadow-[0_4cqw_8cqw_-2cqw_color-mix(in_oklab,var(--color-rail)_80%,transparent)]",
          "[mask-image:linear-gradient(180deg,black_55%,transparent_98%)]"
        )}
        style={{
          left: pct(x),
          top: pct(y),
          transform:
            "translate(-50%, -50%) perspective(140cqw) rotateX(16deg) rotateY(-14deg) rotateZ(3deg)",
        }}
      />
      {/* ground: the band surface swallows the far edges so text can sit beside the screen */}
      <div
        aria-hidden
        className="absolute inset-0 bg-[linear-gradient(0deg,var(--b-surface,var(--color-rail))_0%,transparent_30%)]"
      />
    </div>
  );
}
