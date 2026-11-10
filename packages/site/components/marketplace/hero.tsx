"use client";

/** Search is the first thing to do here: the page opens on it, among logos people already know. */
import { cn } from "@compozy/ui";
import { Search } from "@compozy/ui/icons";
import { m, useScroll } from "motion/react";

import { ASSET } from "@/components/landing/assets";
import { EntryLogo } from "@/components/marketplace/entry-card";
import { useReducedMotionSafe } from "@/components/site/hooks/use-reduced-motion-safe";
import { useScrollTransform } from "@/components/site/hooks/use-scroll-transform";
import { CONTAINER } from "@/components/site/layout";
import { Marquee } from "@/components/site/motion";
import { H1, LandingEyebrow, Lead, Plate } from "@/components/site/primitives";
import { fieldTiles, type Tile } from "@/components/marketplace/hero-field";
import type { MarketplaceCard } from "@/lib/marketplace-cards";

/** Half of the centered column, plus air: the logos live only in the gutters beside it. */
const COLUMN_HALF = "calc(50% + 380px)";

function FloatingLogo({ tile, index }: { tile: Tile; index: number }) {
  const [left, top, depth, size] = tile.slot;
  const reduced = useReducedMotionSafe();
  const { scrollY } = useScroll();
  const y = useScrollTransform(scrollY, [0, 700], reduced ? [0, 0] : [0, -170 * depth]);
  return (
    <div className="absolute" style={{ left: `${left}%`, top, transform: `translateX(-${left}%)` }}>
      <m.div style={{ y, opacity: 0.45 + depth * 0.55 }}>
        <m.div
          initial={{ opacity: 0, scale: 0.7, y: 24 }}
          animate={
            reduced ? { opacity: 1, scale: 1, y: 0 } : { opacity: 1, scale: 1, y: [0, -9, 0] }
          }
          transition={
            reduced
              ? { duration: 0 }
              : {
                  opacity: { duration: 0.7, delay: 0.15 + index * 0.05 },
                  scale: { duration: 0.9, delay: 0.15 + index * 0.05, ease: [0.16, 1, 0.3, 1] },
                  y: {
                    duration: 6 + (index % 4),
                    delay: index * 0.2,
                    repeat: Infinity,
                    ease: "easeInOut",
                  },
                }
          }
        >
          <EntryLogo
            entry={tile.entry}
            size={size}
            className="shadow-[inset_0_0_0_1px_rgba(255,255,255,0.12),0_24px_50px_-14px_rgba(0,0,0,0.9)]"
          />
        </m.div>
      </m.div>
    </div>
  );
}

export function MarketplaceHero({
  entries,
  categoryCount,
  query,
  onQuery,
}: {
  entries: readonly MarketplaceCard[];
  categoryCount: number;
  query: string;
  onQuery: (value: string) => void;
}) {
  const { left, right } = fieldTiles(entries);
  return (
    <section className="relative overflow-hidden pt-[150px] pb-20 md:min-h-[720px] md:pt-[190px]">
      <Plate
        src={ASSET.plateWatch}
        className="inset-x-0 top-[420px] bottom-0 h-[calc(100%-420px)] w-full object-[50%_40%] opacity-75 [mask-image:linear-gradient(180deg,transparent,black_26%,black_56%,transparent_100%)]"
      />
      <div
        aria-hidden
        className="absolute inset-x-0 top-0 mx-auto hidden h-full max-w-[1760px] xl:block"
      >
        <div className="absolute inset-y-0 left-6" style={{ right: COLUMN_HALF }}>
          {left.map((tile, index) => (
            <FloatingLogo key={tile.entry.id} tile={tile} index={index} />
          ))}
        </div>
        <div className="absolute inset-y-0 right-6" style={{ left: COLUMN_HALF }}>
          {right.map((tile, index) => (
            <FloatingLogo key={tile.entry.id} tile={tile} index={left.length + index} />
          ))}
        </div>
      </div>
      <div className={cn(CONTAINER, "relative z-10 flex flex-col items-center text-center")}>
        <LandingEyebrow>Marketplace</LandingEyebrow>
        <div className="max-w-[820px]">
          <H1>Install what your agents need</H1>
        </div>
        <Lead className="max-w-[620px]">
          Plugins, extensions, and MCP servers for the tools your team already uses. Find one here,
          add it from the app.
        </Lead>
        <label className="mt-10 flex h-[60px] w-full max-w-[620px] items-center gap-3.5 rounded-[18px] border border-site-light-150 bg-[#121212]/80 pr-5 pl-5 shadow-[0_30px_80px_-30px_rgba(31,135,252,0.55)] backdrop-blur-xl transition-[border-color] focus-within:border-site-light-350">
          <Search aria-hidden className="size-[18px] shrink-0 text-subtle" />
          <span className="sr-only">Search the catalog</span>
          <input
            value={query}
            onChange={event => onQuery(event.target.value)}
            placeholder={`Search ${entries.length} plugins and extensions`}
            className="h-full min-w-0 flex-1 bg-transparent text-[17px] text-fg outline-none placeholder:text-subtle"
          />
        </label>
        <p className="mt-5 mb-0 text-[14px] text-subtle">
          {entries.length} in the catalog, across {categoryCount} categories
        </p>
      </div>
      <Marquee seconds={46} className="relative z-10 mt-12 xl:hidden">
        {[...left, ...right].map(tile => (
          <EntryLogo key={tile.entry.id} entry={tile.entry} size="lg" />
        ))}
      </Marquee>
    </section>
  );
}
