"use client";

/** The hero's moving parts: the parallax plate, the dust, the lifting copy and the tilted product window. */
import { cn } from "@compozy/ui";
import { m, type MotionStyle } from "motion/react";
import { useEffect, useState, type ReactNode } from "react";

import { ASSET } from "@/components/landing/assets";
import { Dust } from "@/components/landing/fx/dust";
import { useHeroMotion } from "@/components/landing/hooks/use-hero-motion";
import { CONTAINER, type Focus } from "@/components/site/layout";
import { Camera, Plate, ProductWindow } from "@/components/site/primitives-client";
import { useReducedMotionSafe } from "@/components/site/hooks/use-reduced-motion-safe";

const WHOLE_FRAME: Focus = { x: 0, y: 0, w: 1 };
const DRIFT: Focus = { x: 0.012, y: 0.012, w: 0.976 };
const CLIP: ReadonlyArray<{ src: string; alt: string; from: Focus; to: Focus }> = [
  {
    src: ASSET.heroAway,
    alt: "An always-on agent's main session with its While you were away card",
    from: WHOLE_FRAME,
    to: DRIFT,
  },
  {
    src: ASSET.clarify,
    alt: "The sessions board with a session open beside it, asking a person to choose",
    from: WHOLE_FRAME,
    to: DRIFT,
  },
  {
    src: ASSET.board,
    alt: "The sessions board, sorted by who owns the next step",
    from: WHOLE_FRAME,
    to: DRIFT,
  },
];
const STEP_MS = 5600;
/** The hero window spans the page up to 1560px. */
const HERO_SIZES = "(min-width: 1560px) 1560px, 100vw";
const POSTER = CLIP.length - 1;

/** Stands in for the recorded hero clip: real frames, filmed in the order the clip will follow. */
function HeroClip() {
  const reduced = useReducedMotionSafe();
  const [frame, setFrame] = useState(0);
  const [settled, setSettled] = useState(false);
  useEffect(() => {
    if (reduced) return;
    const settle = window.setTimeout(() => setSettled(true), 500);
    const timer = window.setInterval(() => {
      setSettled(false);
      setFrame(current => (current + 1) % CLIP.length);
      window.setTimeout(() => setSettled(true), 500);
    }, STEP_MS);
    return () => {
      window.clearTimeout(settle);
      window.clearInterval(timer);
    };
  }, [reduced]);
  const shown = reduced ? POSTER : frame;
  return (
    <>
      {CLIP.map((item, index) => (
        <Camera
          key={item.src}
          src={item.src}
          alt={item.alt}
          active={index === shown}
          focus={index === shown && (settled || reduced) ? item.to : item.from}
          duration={3.4}
          sizes={HERO_SIZES}
          load={index === 0 ? "priority" : "lazy"}
        />
      ))}
      <div aria-hidden className="absolute inset-x-5 bottom-4 z-10 flex gap-1.5">
        {CLIP.map((item, index) => (
          <span
            key={item.src}
            className="h-[3px] flex-1 overflow-hidden rounded-pill bg-site-light-150"
          >
            {index === frame && !reduced ? (
              <m.span
                key={frame}
                className="block h-full origin-left bg-brand"
                initial={{ scaleX: 0 }}
                animate={{ scaleX: 1 }}
                transition={{ duration: STEP_MS / 1000, ease: "linear" }}
              />
            ) : null}
          </span>
        ))}
      </div>
    </>
  );
}

export function HeroScene({ children }: { children: ReactNode }) {
  const { ref, reduced, desktop, flat, plateY, copyY, copyOpacity } = useHeroMotion();
  return (
    <section ref={ref} id="hero" className="relative overflow-x-clip pb-20 md:pb-32">
      <Plate
        src={ASSET.plateHero}
        style={{ y: plateY }}
        load="eager"
        className="inset-x-0 top-0 h-[1120px] w-full [mask-image:linear-gradient(180deg,black_62%,transparent)]"
      />
      {desktop && !reduced ? (
        <Dust
          count={170}
          className="absolute inset-x-0 top-0 h-[1000px] w-full [mask-image:linear-gradient(180deg,black_60%,transparent)]"
        />
      ) : null}
      <m.div
        style={{ y: copyY, opacity: copyOpacity }}
        className={cn(CONTAINER, "relative z-10 pt-36 text-center md:pt-44")}
      >
        {children}
      </m.div>
      <div className="relative z-20 mx-auto mt-14 w-full max-w-[1560px] px-4 [perspective:1500px] md:mt-20 md:px-8">
        <ProductWindow
          className="site-hero-tilt"
          style={{ "--hero-flat": flat, transformOrigin: "50% 0%" } as MotionStyle}
        >
          <HeroClip />
        </ProductWindow>
      </div>
    </section>
  );
}
