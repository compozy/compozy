"use client";

/** The scroll-pinned stages of the story: the pile of terminals becoming one board, and the always-on beats. */
import { AgentAvatar, cn } from "@compozy/ui";
import { MotionConfig, m, useMotionValueEvent, useTransform, type MotionValue } from "motion/react";
import { useState } from "react";

import { ASSET } from "@/components/landing/assets";
import { useProblemStage } from "@/components/landing/hooks/use-problem-stage";
import { useReducedMotionSafe } from "@/components/landing/hooks/use-reduced-motion-safe";
import { useStage } from "@/components/landing/hooks/use-stage";
import { BEATS, FACES, PANES, PILE_LINE, type Pane } from "@/components/landing/story-data";
import { TerminalPane } from "@/components/landing/terminal-pane";
import { STAGE_FADE } from "@/components/site/layout";
import { Muted } from "@/components/site/primitives";
import { Camera, Plate, ProductWindow } from "@/components/site/primitives-client";
import { useScrollTransform } from "@/components/landing/hooks/use-scroll-transform";

function FlyingPane({
  pane,
  index,
  progress,
}: {
  pane: Pane;
  index: number;
  progress: MotionValue<number>;
}) {
  const start = 0.08 + index * 0.03;
  const x = useScrollTransform(progress, [start, 0.54], [`${pane.x}vw`, "0vw"]);
  const y = useScrollTransform(progress, [start, 0.54], [`${pane.y}vh`, "0vh"]);
  const rotate = useScrollTransform(progress, [start, 0.54], [pane.r, 0]);
  const scale = useScrollTransform(progress, [start, 0.54], [1, 0.42]);
  const opacity = useScrollTransform(progress, [0.46, 0.56], [1, 0]);
  return (
    <m.div
      style={{ x, y, rotate, scale, opacity }}
      className="absolute top-1/2 left-1/2 -mt-20 -ml-[clamp(170px,12.5vw,240px)] w-[clamp(340px,25vw,480px)]"
    >
      <TerminalPane pane={pane} />
    </m.div>
  );
}

export function ProblemStage() {
  const {
    ref,
    progress,
    tangleOpacity,
    tangleScale,
    orderOpacity,
    boardOpacity,
    boardScale,
    beforeOpacity,
    afterOpacity,
    lineOpacity,
    lineY,
  } = useProblemStage();
  return (
    <div ref={ref} className="relative h-[340vh]">
      <div className="sticky top-0 flex h-screen items-center justify-center overflow-hidden bg-[#0a0a0a]">
        <Plate
          src={ASSET.plateTangle}
          style={{ opacity: tangleOpacity, scale: tangleScale }}
          className={cn("inset-0 size-full", STAGE_FADE)}
        />
        <Plate
          src={ASSET.plateOrder}
          style={{ opacity: orderOpacity }}
          className={cn("inset-0 size-full", STAGE_FADE)}
        />
        <m.p
          style={{ opacity: beforeOpacity }}
          className="absolute top-[72px] left-1/2 z-10 m-0 -translate-x-1/2 rounded-pill border border-site-light-150 bg-site-dark-700 px-4 py-1.5 font-mono text-[13px] text-fg backdrop-blur-md"
        >
          Before
        </m.p>
        <m.p
          style={{ opacity: afterOpacity }}
          className="absolute top-[72px] left-1/2 z-10 m-0 -translate-x-1/2 rounded-pill border border-site-light-150 bg-site-dark-700 px-4 py-1.5 font-mono text-[13px] text-brand-strong backdrop-blur-md"
        >
          After
        </m.p>
        {PANES.map((pane, index) => (
          <FlyingPane key={pane.id} pane={pane} index={index} progress={progress} />
        ))}
        <m.div
          style={{ opacity: boardOpacity, scale: boardScale }}
          className="relative w-[min(1500px,84vw,calc(76vh*1.6))]"
        >
          <ProductWindow>
            <img
              src={ASSET.board}
              alt="One sessions board sorted by who owns the next step"
              draggable={false}
              className="block size-full object-cover"
            />
          </ProductWindow>
        </m.div>
        <m.p
          style={{ opacity: lineOpacity, y: lineY }}
          className="absolute inset-x-0 bottom-9 m-0 text-center font-display text-[34px] tracking-[-0.02em] text-fg [text-shadow:0_2px_30px_#000] md:text-[44px]"
        >
          {PILE_LINE}
        </m.p>
      </div>
    </div>
  );
}

/** The avatars read reduced motion through MotionConfig, so it follows the hydration-safe preference. */
export function Faces() {
  const reduced = useReducedMotionSafe();
  return (
    <MotionConfig reducedMotion={reduced ? "always" : "never"}>
      <div className="flex flex-wrap gap-2.5">
        {FACES.map((seed, index) => (
          <m.span
            key={seed}
            initial={{ opacity: 0, y: 14, scale: 0.7 }}
            whileInView={{ opacity: 1, y: 0, scale: 1 }}
            viewport={{ once: true }}
            transition={{ delay: index * 0.04, type: "spring", stiffness: 260, damping: 18 }}
          >
            <AgentAvatar
              appearance={{ avatar: "generated", seed }}
              label={seed}
              size={52}
              state={index % 5 === 4 ? "needs-you" : "working"}
              surfaceClassName="ring-[#0a0a0a]"
            />
          </m.span>
        ))}
      </div>
    </MotionConfig>
  );
}

export function AlwaysOnStage() {
  const { ref, progress } = useStage();
  const [active, setActive] = useState(0);
  useMotionValueEvent(progress, "change", value =>
    setActive(Math.min(BEATS.length - 1, Math.floor(value * BEATS.length)))
  );
  const bar = useTransform(progress, value => (value * BEATS.length) % 1);
  return (
    <div ref={ref} className="relative h-[420vh]">
      <div className="sticky top-0 flex h-screen items-center overflow-hidden bg-[#070707]">
        <Plate src={ASSET.plateWatch} className={cn("inset-0 size-full opacity-70", STAGE_FADE)} />
        <div className="relative mx-auto grid w-full max-w-[1720px] grid-cols-[340px_minmax(0,1fr)] items-center gap-12 px-10">
          <ol className="m-0 list-none space-y-1 p-0">
            {BEATS.map((beat, index) => {
              const on = index === active;
              return (
                <li
                  key={beat.label}
                  className={cn(
                    "relative overflow-hidden rounded-[16px] border px-5 py-4 transition-colors duration-500",
                    on
                      ? "border-site-light-150 bg-site-light-060 backdrop-blur-xl"
                      : "border-transparent"
                  )}
                >
                  <p
                    className={cn(
                      "m-0 flex items-center gap-2.5 text-[18px] transition-colors",
                      on ? "text-fg" : "text-fg/40"
                    )}
                  >
                    <beat.icon
                      className={cn("size-[18px]", on ? "text-brand-strong" : "text-fg/40")}
                      aria-hidden
                    />
                    {beat.label}
                  </p>
                  <div
                    className={cn(
                      "grid transition-[grid-template-rows,opacity] duration-500",
                      on ? "grid-rows-[1fr] opacity-100" : "grid-rows-[0fr] opacity-0"
                    )}
                  >
                    <div className="overflow-hidden">
                      <Muted className="pt-2 text-fg/75">{beat.body}</Muted>
                    </div>
                  </div>
                  {on ? (
                    <m.span
                      aria-hidden
                      style={{ scaleX: bar }}
                      className="absolute inset-x-0 bottom-0 h-[2px] origin-left bg-[linear-gradient(90deg,#1f87fc,#1df0d7)]"
                    />
                  ) : null}
                </li>
              );
            })}
          </ol>
          <div className="ml-auto w-full max-w-[calc(86vh*1.6)]">
            <ProductWindow>
              {BEATS.map((beat, index) => (
                <Camera
                  key={beat.label}
                  src={beat.src}
                  alt={beat.alt}
                  active={index === active}
                  focus={beat.focus}
                />
              ))}
            </ProductWindow>
          </div>
        </div>
      </div>
    </div>
  );
}
