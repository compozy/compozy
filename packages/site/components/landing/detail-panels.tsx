"use client";

/** Three supervision details as panels that open in turn; pointing at one takes over from the timer. */
import { cn } from "@compozy/ui";
import { Bookmark, GitPullRequest, Hand } from "@compozy/ui/icons";
import { m } from "motion/react";
import { useEffect, useState } from "react";

import { ASSET } from "@/components/landing/assets";
import type { Icon } from "@/components/site/primitives";
import { useReducedMotionSafe } from "@/components/site/hooks/use-reduced-motion-safe";

const DETAIL_PANELS: ReadonlyArray<{
  icon: Icon;
  src: string;
  art: string;
  alt: string;
  text: string;
}> = [
  {
    icon: Hand,
    src: ASSET.cropApproval,
    art: ASSET.ucGate,
    alt: "A tool call waiting for Don't allow, Always allow or Allow once",
    text: "Approve a tool call in one click, from the session or the bell.",
  },
  {
    icon: Bookmark,
    src: ASSET.cropLater,
    art: ASSET.ucBrief,
    alt: "The Move to Later dialog with a reason and a reminder date",
    text: "Park a session for later, with a reason and a date.",
  },
  {
    icon: GitPullRequest,
    src: ASSET.cropReview,
    art: ASSET.plateBranch,
    alt: "The In review column with each session's pull request",
    text: "A pull request moves its session to In review on its own.",
  },
];
const DETAIL_STEP_MS = 5200;

export function DetailPanels() {
  const reduced = useReducedMotionSafe();
  const [active, setActive] = useState(0);
  const [held, setHeld] = useState(false);
  useEffect(() => {
    if (reduced || held) return;
    const timer = window.setInterval(
      () => setActive(current => (current + 1) % DETAIL_PANELS.length),
      DETAIL_STEP_MS
    );
    return () => window.clearInterval(timer);
  }, [reduced, held]);
  return (
    <div
      className="flex flex-col gap-4 lg:h-[640px] lg:flex-row"
      onPointerLeave={() => setHeld(false)}
    >
      {DETAIL_PANELS.map((panel, index) => {
        const on = index === active;
        return (
          <button
            key={panel.text}
            type="button"
            aria-pressed={on}
            onPointerEnter={() => {
              setActive(index);
              setHeld(true);
            }}
            onFocus={() => {
              setActive(index);
              setHeld(true);
            }}
            onClick={() => setActive(index)}
            className={cn(
              "group relative h-[520px] min-w-0 cursor-pointer lg:basis-0 overflow-hidden rounded-[26px] border bg-black text-left transition-[flex-grow,border-color] duration-700 ease-[cubic-bezier(0.16,1,0.3,1)] focus-visible:outline-none focus-visible:shadow-focus-ring lg:h-auto",
              on ? "border-site-light-250 lg:grow-[2.7]" : "border-site-light-100 lg:grow"
            )}
          >
            <img
              src={panel.art}
              alt=""
              aria-hidden
              draggable={false}
              className={cn(
                "absolute inset-0 size-full object-cover transition-opacity duration-700",
                on ? "opacity-70" : "opacity-40"
              )}
            />
            <div
              aria-hidden
              className="absolute inset-0 bg-[linear-gradient(180deg,rgba(10,10,10,0.1)_40%,rgba(10,10,10,0.94)_84%)]"
            />
            <div className="absolute inset-x-6 top-6 bottom-[150px] flex items-center justify-center">
              <img
                src={panel.src}
                alt={panel.alt}
                draggable={false}
                className={cn(
                  "max-h-full max-w-full rounded-[14px] border border-site-light-150 object-contain shadow-[0_30px_80px_-10px_#000] transition-[opacity,transform] duration-700 ease-[cubic-bezier(0.16,1,0.3,1)]",
                  on ? "scale-100 opacity-100" : "scale-[0.92] opacity-30"
                )}
              />
            </div>
            <div className="absolute inset-x-7 bottom-7">
              <panel.icon
                className={cn("size-6 transition-colors", on ? "text-brand-strong" : "text-fg/60")}
                aria-hidden
              />
              <p className="m-0 mt-3 max-w-[520px] font-display text-[21px]/[1.2] tracking-[-0.015em] text-fg md:text-[25px]/[1.18]">
                {panel.text}
              </p>
            </div>
            {on && !held && !reduced ? (
              <m.span
                key={active}
                aria-hidden
                className="absolute inset-x-0 bottom-0 h-[2px] origin-left bg-[linear-gradient(90deg,#1f87fc,#1df0d7)]"
                initial={{ scaleX: 0 }}
                animate={{ scaleX: 1 }}
                transition={{ duration: DETAIL_STEP_MS / 1000, ease: "linear" }}
              />
            ) : null}
          </button>
        );
      })}
    </div>
  );
}
