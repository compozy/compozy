"use client";

import Image from "next/image";
import { useEffect, useRef } from "react";
import { cn } from "@compozy/ui/lib/utils";
import { useReducedMotion } from "./primitives/use-reduced-motion";

// Pointer swing around the rest pose, in degrees. Narrow on purpose: the
// response should read as depth, not as a toy card.
const PITCH_RANGE_DEG = 4;
const YAW_RANGE_DEG = 6;

const PANEL_TRANSFORM = [
  `rotateX(calc(var(--hero-rx) + var(--tilt-y, 0) * ${PITCH_RANGE_DEG}deg))`,
  `rotateY(calc(var(--hero-ry) + var(--tilt-x, 0) * -${YAW_RANGE_DEG}deg))`,
].join(" ");

const TILT_TRANSITION = "transform var(--duration-slow) var(--ease-out)";

interface HeroVisualProps {
  className?: string;
}

/**
 * The deck-cover hero shot: the real OS shell capture pitched and yawed in
 * perspective. Pointer travel steers the pose through `--tilt-x`/`--tilt-y`
 * written straight to the DOM (no React state on the pointer path); touch
 * input and reduced motion keep the static rest pose.
 */
export function HeroVisual({ className }: HeroVisualProps) {
  const frameRef = useRef<HTMLDivElement>(null);
  const rafRef = useRef(0);
  const reduced = useReducedMotion();

  useEffect(() => () => cancelAnimationFrame(rafRef.current), []);

  const setTilt = (x: number, y: number) => {
    const frame = frameRef.current;
    if (!frame) return;
    cancelAnimationFrame(rafRef.current);
    rafRef.current = requestAnimationFrame(() => {
      frame.style.setProperty("--tilt-x", x.toFixed(3));
      frame.style.setProperty("--tilt-y", y.toFixed(3));
    });
  };

  const handlePointerMove = (event: React.PointerEvent<HTMLDivElement>) => {
    if (reduced || event.pointerType === "touch") return;
    const rect = event.currentTarget.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return;
    setTilt(
      ((event.clientX - rect.left) / rect.width) * 2 - 1,
      ((event.clientY - rect.top) / rect.height) * 2 - 1
    );
  };

  const handlePointerLeave = () => {
    if (reduced) return;
    setTilt(0, 0);
  };

  return (
    <div className={cn("relative", className)}>
      {/* Ambient accent bloom behind the window, echoing the capture's own gradient rim. */}
      <div
        aria-hidden="true"
        className="absolute -inset-[8%] rounded-window blur-3xl"
        style={{
          background:
            "radial-gradient(58% 55% at 24% 80%, var(--color-accent-dim), transparent 72%)",
        }}
      />
      <div
        ref={frameRef}
        onPointerMove={handlePointerMove}
        onPointerLeave={handlePointerLeave}
        className="relative [perspective:1400px]"
      >
        <div
          className="relative overflow-hidden rounded-window shadow-window ring-1 ring-line-strong will-change-transform [--hero-rx:6deg] [--hero-ry:-8deg] lg:[--hero-rx:9deg] lg:[--hero-ry:-16deg]"
          style={{ transform: PANEL_TRANSFORM, transition: TILT_TRANSITION }}
        >
          <Image
            src="/images/hero/os-shell-capture-v1.png"
            alt="CompozyOS workspace capture: the Loops window lists three workspace loops beside a Tasks board tracking launch work across Blocked, Queued, and Done."
            width={3124}
            height={2002}
            priority
            quality={90}
            sizes="(min-width: 1024px) 52rem, 100vw"
            className="h-auto w-full"
          />
          {/* Pointer-tracked sheen: a faint light streak that drifts with the pose. */}
          <div
            aria-hidden="true"
            className="pointer-events-none absolute inset-0 mix-blend-screen"
            style={{
              background:
                "linear-gradient(115deg, transparent 40%, var(--color-overlay-ghost-hover) 50%, transparent 62%)",
              transform:
                "translate3d(calc(var(--tilt-x, 0) * 8%), calc(var(--tilt-y, 0) * 8%), 0) scale(1.2)",
              transition: TILT_TRANSITION,
            }}
          />
        </div>
      </div>
    </div>
  );
}
