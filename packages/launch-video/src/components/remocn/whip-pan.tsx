// Source: https://remocn.dev — copied from the registry; MIT.
"use client";

import type {
  TransitionPresentation,
  TransitionPresentationComponentProps,
} from "@remotion/transitions";
import type React from "react";
import { AbsoluteFill, Easing, interpolate } from "remotion";

const clampOpts = {
  extrapolateLeft: "clamp" as const,
  extrapolateRight: "clamp" as const,
};

export type WhipPanProps = {
  direction?: "left" | "right" | "up" | "down";
  blur?: number;
};

const WhipPanPresentation: React.FC<TransitionPresentationComponentProps<WhipPanProps>> = ({
  children,
  presentationProgress,
  presentationDirection,
  passedProps,
}) => {
  const { direction = "left", blur = 24 } = passedProps;
  const entering = presentationDirection === "entering";
  const p = presentationProgress;

  const travel = interpolate(p, [0, 1], [0, 1], {
    ...clampOpts,
    easing: Easing.bezier(0.7, 0, 0.2, 1),
  });
  const velocity = Math.sin(Math.PI * travel);

  const axis: "x" | "y" = direction === "left" || direction === "right" ? "x" : "y";
  const sign = direction === "left" || direction === "up" ? -1 : 1;

  const offset = entering ? (travel - 1) * 110 * sign : travel * 110 * sign;
  const stretch = 1 + velocity * 0.12;

  const translate = axis === "x" ? `${offset}%` : `0 ${offset}%`;
  const smear = axis === "x" ? `${stretch} 1` : `1 ${stretch}`;

  const style: React.CSSProperties = {
    translate,
    scale: smear,
    filter: `blur(${velocity * blur}px)`,
  };

  return <AbsoluteFill style={style}>{children}</AbsoluteFill>;
};

export function whipPan(props: WhipPanProps = {}): TransitionPresentation<WhipPanProps> {
  return {
    component: WhipPanPresentation,
    props,
  };
}
