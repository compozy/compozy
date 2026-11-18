"use client";

/** The numbered section map's rail: the brand beam fills as the map scrolls past. */
import { cn } from "@compozy/ui";
import { m, useScroll } from "motion/react";
import { useRef, type ReactNode } from "react";

import { BEAM } from "@/components/docs/docs-classes";
import { useReducedMotionSafe } from "@/components/site/hooks/use-reduced-motion-safe";

export function SectionMapRail({ children }: { children: ReactNode }) {
  const rail = useRef<HTMLOListElement>(null);
  const reduced = useReducedMotionSafe();
  const { scrollYProgress } = useScroll({ target: rail, offset: ["start 70%", "end 55%"] });
  return (
    <ol ref={rail} className="docs-wide docs-steps not-prose relative my-10 list-none! p-0 pl-0!">
      <span aria-hidden className="absolute top-2 bottom-2 left-[13px] w-px bg-site-light-120">
        <m.span
          style={{ scaleY: reduced ? 1 : scrollYProgress }}
          className={cn("absolute inset-y-0 -left-px w-0.5 origin-top", BEAM)}
        />
      </span>
      {children}
    </ol>
  );
}
