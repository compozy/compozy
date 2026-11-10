"use client";

/** The built-in features as tabs over one product window; they advance on their own until someone picks one. */
import { Tabs, TabsContent, TabsList, TabsTrigger, cn } from "@compozy/ui";
import { m } from "motion/react";
import { useEffect, useState } from "react";

import { FEATURES } from "@/components/landing/content";
import { H3, Muted } from "@/components/site/primitives";
import { Camera, ProductWindow } from "@/components/site/primitives-client";
import { useReducedMotionSafe } from "@/components/site/hooks/use-reduced-motion-safe";

const ADVANCE_MS = 6500;

export function FeatureTabs() {
  const reduced = useReducedMotionSafe();
  const [active, setActive] = useState(FEATURES[0].tab);
  const [interacted, setInteracted] = useState(false);
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

  return (
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
        <TabsList className="max-w-full flex-wrap justify-center gap-1 rounded-[20px] border border-site-light-150 bg-[#121212]/85 p-1.5 shadow-[0_24px_70px_-10px_#000] backdrop-blur-2xl group-data-horizontal/tabs:h-auto">
          {FEATURES.map(feature => {
            const on = feature.tab === active;
            return (
              <TabsTrigger
                key={feature.tab}
                value={feature.tab}
                className={cn(
                  "relative h-11 gap-2 overflow-hidden rounded-[14px] px-4 text-[14px] after:hidden",
                  on && "bg-site-light-100 text-fg"
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
  );
}
