import { Button, Tabs, TabsContent, TabsList, TabsTrigger, cn } from "@compozy/ui";
import { ChevronLeft, ChevronRight } from "@compozy/ui/icons";
import { m, useReducedMotion } from "motion/react";
import { useEffect, useRef, useState } from "react";

import terrain from "./assets/plate-terrain.jpg";
import { SpotlightCard } from "./fx/spotlight-card";
import { DETAILS, FEATURES } from "./content";
import { Reveal, ScrollWords } from "./motion";
import { CONTAINER, HAIRLINE } from "./layout";
import { Camera, LandingEyebrow, H2, H3, Muted, Plate, ProductWindow } from "./primitives";

const ADVANCE_MS = 6500;
const CARD_STEP = 340;

export function BuiltIn() {
  const reduced = useReducedMotion();
  const [active, setActive] = useState(FEATURES[0].tab);
  const [interacted, setInteracted] = useState(false);
  const rail = useRef<HTMLDivElement>(null);
  const auto = !interacted && !reduced;

  useEffect(() => {
    if (!auto) return;
    const timer = window.setInterval(() => {
      setActive(
        current =>
          FEATURES[(FEATURES.findIndex(feature => feature.tab === current) + 1) % FEATURES.length]
            .tab
      );
    }, ADVANCE_MS);
    return () => window.clearInterval(timer);
  }, [auto]);

  const scroll = (direction: 1 | -1) =>
    rail.current?.scrollBy({ left: direction * CARD_STEP, behavior: "smooth" });

  return (
    <section
      id="features"
      className={cn("relative overflow-x-clip border-t bg-[#070707] py-24 md:py-44", HAIRLINE)}
    >
      <Plate
        src={terrain}
        className="inset-x-0 top-[420px] h-[900px] w-full opacity-80 [mask-image:linear-gradient(180deg,transparent,black_30%,black_70%,transparent)]"
      />
      <div className={cn(CONTAINER, "relative")}>
        <div className="text-center">
          <Reveal>
            <LandingEyebrow>Built in</LandingEyebrow>
          </Reveal>
          <H2 size="lg">
            <ScrollWords text="Comes with what you would otherwise build." />
          </H2>
        </div>
        <Tabs
          value={active}
          onValueChange={value => {
            setActive(String(value));
            setInteracted(true);
          }}
          className="mt-14 gap-0 md:mt-20"
        >
          <div className="min-h-[108px]">
            {FEATURES.map(feature => (
              <TabsContent
                key={feature.tab}
                value={feature.tab}
                className="mx-auto max-w-[720px] text-center"
              >
                <H3 className="md:text-[36px]/[1.1]">{feature.title}</H3>
                <Muted className="mt-3 text-[18px]/[1.5]">{feature.body}</Muted>
              </TabsContent>
            ))}
          </div>
          <ProductWindow className="mt-8">
            {FEATURES.map(feature => (
              <Camera
                key={feature.tab}
                src={feature.shot}
                alt={feature.alt}
                active={feature.tab === active}
              />
            ))}
          </ProductWindow>
          <div className="relative z-10 -mt-8 flex justify-center">
            <TabsList className="max-w-full flex-wrap justify-center gap-1 rounded-[20px] border border-[color:var(--light-150)] bg-[#121212]/85 p-1.5 shadow-[0_24px_70px_-10px_#000] backdrop-blur-2xl group-data-horizontal/tabs:h-auto">
              {FEATURES.map(feature => {
                const on = feature.tab === active;
                return (
                  <TabsTrigger
                    key={feature.tab}
                    value={feature.tab}
                    className={cn(
                      "relative h-11 gap-2 overflow-hidden rounded-[14px] px-4 text-[14px] after:hidden",
                      on && "bg-[color:var(--light-100)] text-fg"
                    )}
                  >
                    <span className="inline-flex items-center gap-2">
                      <feature.icon
                        className={cn("size-4 shrink-0", on && "text-brand-strong")}
                        aria-hidden
                      />
                      {feature.tab}
                    </span>
                    {on && auto ? (
                      <m.span
                        key={active}
                        aria-hidden
                        className="absolute inset-x-0 bottom-0 h-[2px] origin-left bg-brand"
                        initial={{ scaleX: 0 }}
                        animate={{ scaleX: 1 }}
                        transition={{ duration: ADVANCE_MS / 1000, ease: "linear" }}
                      />
                    ) : null}
                  </TabsTrigger>
                );
              })}
            </TabsList>
          </div>
        </Tabs>
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
            className="flex h-[260px] w-[320px] shrink-0 snap-start flex-col justify-between rounded-[20px] border border-[color:var(--light-100)] p-7"
          >
            <span className="grid size-12 place-items-center rounded-[14px] border border-[color:var(--light-120)] bg-[linear-gradient(45deg,rgba(31,135,252,0.28),rgba(29,240,215,0.14))]">
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
    </section>
  );
}
