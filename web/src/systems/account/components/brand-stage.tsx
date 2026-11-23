import type { ReactNode } from "react";

import { cn, Logo } from "@compozy/ui";

// Brand Atmosphere visuals (brand manual, `packages/brand`), the same family the site's art plates
// come from. The shipped cloud pages and app serve their own self-hosted copies of these files.
import coneLand from "../../../../../packages/brand/public/brand/visuals/cone-1280.jpg";
import conePortrait from "../../../../../packages/brand/public/brand/visuals/cone-portrait-1280.jpg";
import convergeLand from "../../../../../packages/brand/public/brand/visuals/converge-1280.jpg";
import convergePortrait from "../../../../../packages/brand/public/brand/visuals/converge-portrait-1280.jpg";
import swarmLand from "../../../../../packages/brand/public/brand/visuals/swarm-1280.jpg";
import swarmPortrait from "../../../../../packages/brand/public/brand/visuals/swarm-portrait-1280.jpg";

export type BrandStageVisual = "swarm" | "cone" | "converge";

const VISUALS: Record<
  BrandStageVisual,
  {
    land: string;
    portrait: string;
    landPosition: string;
    portraitPosition: string;
    /** Mirror the art so its light source sits away from the wordmark. */
    mirror?: boolean;
  }
> = {
  swarm: {
    land: swarmLand,
    portrait: swarmPortrait,
    landPosition: "70% 55%",
    portraitPosition: "60% 30%",
  },
  cone: {
    land: coneLand,
    portrait: conePortrait,
    landPosition: "20% 20%",
    portraitPosition: "0% 0%",
    mirror: true,
  },
  converge: {
    land: convergeLand,
    portrait: convergePortrait,
    landPosition: "60% 50%",
    portraitPosition: "40% 35%",
  },
};

export interface BrandStageProps {
  children: ReactNode;
  visual: BrandStageVisual;
  /** One display line on the art panel (wide screens). Never the page's own title. */
  statement?: ReactNode;
  dataSlot?: string;
  className?: string;
  contentClassName?: string;
  [dataAttribute: `data-${string}`]: string | undefined;
}

/**
 * Split stage for Compozy's standalone pages (cloud sign-in and billing, the app's sign-in gate):
 * the page's own column on the left and, on the right, an ink art panel carrying a brand Atmosphere
 * visual, the wordmark and one statement (operator, 2026-11-23: content left, image right). The
 * panel stays ink in both themes, because Atmosphere only sits on dark grounds, and no text is
 * laid over the bright part of the visual. Below `lg` the panel becomes a short band above the
 * content.
 */
export function BrandStage({
  children,
  visual,
  statement,
  dataSlot = "brand-stage",
  className,
  contentClassName,
  ...dataAttributes
}: BrandStageProps) {
  const art = VISUALS[visual];

  return (
    <main
      className={cn(
        "flex min-h-0 flex-1 flex-col bg-canvas text-fg lg:flex-row-reverse",
        className
      )}
      data-slot={dataSlot}
      data-visual={visual}
      {...dataAttributes}
    >
      <aside
        aria-hidden="true"
        className="dark relative m-2 flex h-36 shrink-0 flex-col justify-between overflow-hidden rounded-xl bg-black p-5 shadow-[0_0_0_1px_rgba(255,255,255,0.06)] sm:m-3 sm:h-44 lg:h-auto lg:w-[42%] lg:max-w-[640px] lg:p-10"
        data-slot="brand-stage-panel"
      >
        <img
          alt=""
          className={cn(
            "pointer-events-none absolute inset-0 size-full object-cover select-none lg:hidden",
            art.mirror && "-scale-x-100"
          )}
          draggable={false}
          src={art.land}
          style={{ objectPosition: art.landPosition }}
        />
        <img
          alt=""
          className={cn(
            "pointer-events-none absolute inset-0 hidden size-full object-cover select-none lg:block",
            art.mirror && "-scale-x-100"
          )}
          draggable={false}
          src={art.portrait}
          style={{ objectPosition: art.portraitPosition }}
        />
        <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(180deg,rgba(0,0,0,0.55)_0%,transparent_28%,transparent_52%,rgba(0,0,0,0.92)_86%)]" />

        <Logo className="relative h-6 self-center text-white lg:h-7 lg:self-start" variant="logo" />

        {statement ? (
          <p className="relative hidden max-w-[16ch] text-[2.375rem] leading-[1.08] font-medium tracking-display text-balance text-white lg:block">
            {statement}
          </p>
        ) : null}
      </aside>

      <div
        className={cn(
          "relative flex min-h-0 flex-1 flex-col items-center overflow-y-auto px-4 py-8 sm:px-8 sm:py-10",
          contentClassName
        )}
        data-slot="brand-stage-content"
      >
        {/* `my-auto` centers when there is room and never clips the top when the column scrolls. */}
        <div className="my-auto flex w-full flex-col items-center">{children}</div>
      </div>
    </main>
  );
}
