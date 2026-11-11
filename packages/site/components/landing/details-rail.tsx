"use client";

/** "And the details": a snapped rail of small built-in features with previous and next controls. */
import { Button, cn } from "@compozy/ui";
import { ChevronLeft, ChevronRight } from "@compozy/ui/icons";
import { useRef } from "react";

import { DETAILS } from "@/components/landing/content";
import { SpotlightCard } from "@/components/landing/fx/spotlight-card";
import { CONTAINER } from "@/components/site/layout";
import { Muted } from "@/components/site/primitives";

const CARD_STEP = 340;

export function DetailsRail() {
  const rail = useRef<HTMLDivElement>(null);
  const scroll = (direction: 1 | -1) =>
    rail.current?.scrollBy({ left: direction * CARD_STEP, behavior: "smooth" });
  return (
    <>
      <div className={cn(CONTAINER, "relative")}>
        <div className="mt-24 flex items-center justify-between md:mt-32">
          <p className="m-0 font-display text-[28px] tracking-[-0.02em] text-fg md:text-[36px]">
            And the details
          </p>
          <div className="flex gap-1.5">
            <Button
              aria-label="Previous details"
              size="icon-lg"
              variant="outline"
              onClick={() => scroll(-1)}
            >
              <ChevronLeft />
            </Button>
            <Button
              aria-label="Next details"
              size="icon-lg"
              variant="outline"
              onClick={() => scroll(1)}
            >
              <ChevronRight />
            </Button>
          </div>
        </div>
      </div>
      <div
        ref={rail}
        className="relative mt-8 flex snap-x snap-mandatory gap-4 overflow-x-auto scroll-pl-[max(24px,calc((100vw-1200px)/2))] px-[max(24px,calc((100vw-1200px)/2))] pb-2 [scrollbar-width:none]"
      >
        {DETAILS.map(([Glyph, title, body]) => (
          <SpotlightCard
            key={title}
            className="flex h-[260px] w-[320px] shrink-0 snap-start flex-col justify-between rounded-[20px] border border-site-light-100 p-7"
          >
            <span className="grid size-12 place-items-center rounded-[14px] border border-site-light-120 bg-[linear-gradient(45deg,rgba(31,135,252,0.28),rgba(29,240,215,0.14))]">
              <Glyph className="size-[22px] text-fg" aria-hidden />
            </span>
            <div>
              <h3 className="m-0 font-display text-[23px] font-normal tracking-[-0.015em] text-fg">
                {title}
              </h3>
              <Muted className="mt-2">{body}</Muted>
            </div>
          </SpotlightCard>
        ))}
      </div>
    </>
  );
}
