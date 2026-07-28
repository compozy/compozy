"use client";

import * as React from "react";

import { AnimatePresence, m, type HTMLMotionProps } from "motion/react";

import { useIsInView, type UseIsInViewOptions } from "../../hooks/use-is-in-view";
import { getStrictContext } from "../../lib/context";
import { Slot, type WithAsChild } from "../../primitives/slot";

type Side = "top" | "bottom" | "left" | "right";
type Align = "start" | "center" | "end";

interface ParticlesContextType {
  animate: boolean;
  isInView: boolean;
}

const [ParticlesProvider, useParticles] =
  getStrictContext<ParticlesContextType>("ParticlesContext");

export type ParticlesProps = WithAsChild<
  Omit<HTMLMotionProps<"div">, "children"> & {
    animate?: boolean;
    children: React.ReactNode;
  } & UseIsInViewOptions
>;

export function Particles({
  ref,
  animate = true,
  asChild = false,
  inView = false,
  inViewMargin = "0px",
  inViewOnce = true,
  children,
  style,
  ...props
}: ParticlesProps) {
  const { ref: localRef, isInView } = useIsInView(ref as React.Ref<HTMLDivElement>, {
    inView,
    inViewOnce,
    inViewMargin,
  });

  const Component = asChild ? Slot : m.div;

  return (
    <ParticlesProvider value={{ animate, isInView }}>
      <Component ref={localRef} style={{ position: "relative", ...style }} {...props}>
        {children}
      </Component>
    </ParticlesProvider>
  );
}

export interface ParticlesEffectProps extends Omit<HTMLMotionProps<"div">, "children"> {
  side?: Side;
  align?: Align;
  count?: number;
  radius?: number;
  spread?: number;
  duration?: number;
  holdDelay?: number;
  sideOffset?: number;
  alignOffset?: number;
  delay?: number;
}

export function ParticlesEffect({
  side = "top",
  align = "center",
  count = 6,
  radius = 30,
  spread = 360,
  duration = 0.8,
  holdDelay = 0.05,
  sideOffset = 0,
  alignOffset = 0,
  delay = 0,
  transition,
  style,
  ...props
}: ParticlesEffectProps) {
  const { animate, isInView } = useParticles();

  const isVertical = side === "top" || side === "bottom";
  const alignPct = align === "start" ? "0%" : align === "end" ? "100%" : "50%";

  const top = isVertical
    ? side === "top"
      ? `calc(0% - ${sideOffset}px)`
      : `calc(100% + ${sideOffset}px)`
    : `calc(${alignPct} + ${alignOffset}px)`;

  const left = isVertical
    ? `calc(${alignPct} + ${alignOffset}px)`
    : side === "left"
      ? `calc(0% - ${sideOffset}px)`
      : `calc(100% + ${sideOffset}px)`;

  const containerStyle = {
    position: "absolute",
    top,
    left,
    transform: "translate(-50%, -50%)",
  } satisfies NonNullable<HTMLMotionProps<"div">["style"]>;

  const angleStep = (spread * (Math.PI / 180)) / Math.max(1, count - 1);

  return (
    <AnimatePresence>
      {animate &&
        isInView &&
        [...Array(count)].map((_, i) => {
          const angle = i * angleStep;
          const x = Math.cos(angle) * radius;
          const y = Math.sin(angle) * radius;

          return (
            <m.div
              key={i}
              style={style ? { ...containerStyle, ...style } : containerStyle}
              initial={{ scale: 0.95, opacity: 0 }}
              animate={{
                x: `${x}px`,
                y: `${y}px`,
                scale: [0.95, 1, 0.95],
                opacity: [0, 1, 0],
              }}
              transition={{
                duration,
                delay: delay + i * holdDelay,
                ease: "easeOut",
                ...transition,
              }}
              {...props}
            />
          );
        })}
    </AnimatePresence>
  );
}

export { useParticles };
