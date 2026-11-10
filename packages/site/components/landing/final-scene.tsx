"use client";

/** The closing section's backdrop: the plate settles as the page ends, with dust on desktop. */
import { cn } from "@compozy/ui";
import { useScroll } from "motion/react";
import { useRef, type ReactNode } from "react";

import { ASSET } from "@/components/landing/assets";
import { Dust } from "@/components/landing/fx/dust";
import { useIsDesktop } from "@/components/site/hooks/use-is-desktop";
import { HAIRLINE } from "@/components/site/layout";
import { Plate } from "@/components/site/primitives-client";
import { useReducedMotionSafe } from "@/components/landing/hooks/use-reduced-motion-safe";
import { useScrollTransform } from "@/components/landing/hooks/use-scroll-transform";

export function FinalScene({ children }: { children: ReactNode }) {
  const ref = useRef<HTMLElement>(null);
  const reduced = useReducedMotionSafe();
  const desktop = useIsDesktop();
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start end", "end end"] });
  const scale = useScrollTransform(scrollYProgress, [0, 1], reduced ? [1, 1] : [1.18, 1]);
  return (
    <section
      ref={ref}
      id="final"
      className={cn("relative overflow-hidden border-t py-36 md:py-60", HAIRLINE)}
    >
      <Plate
        src={ASSET.plateFinal}
        style={{ scale, transformOrigin: "100% 100%" }}
        className="inset-0 size-full [mask-image:linear-gradient(180deg,transparent,black_18%,black_80%,transparent)]"
      />
      {desktop && !reduced ? <Dust count={120} className="absolute inset-0 size-full" /> : null}
      {children}
    </section>
  );
}
