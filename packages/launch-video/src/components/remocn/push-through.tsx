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

export type PushThroughProps = {
  zoom?: number;
  blur?: number;
};

const PushThroughPresentation: React.FC<TransitionPresentationComponentProps<PushThroughProps>> = ({
  children,
  presentationProgress,
  presentationDirection,
  passedProps,
}) => {
  const { zoom = 2.4, blur = 14 } = passedProps;
  const entering = presentationDirection === "entering";
  const p = presentationProgress;

  if (!entering) {
    const exitStyle: React.CSSProperties = {
      opacity: interpolate(p, [0.4, 0.62], [1, 0], {
        ...clampOpts,
        easing: Easing.bezier(0.42, 0, 0.58, 1),
      }),
      scale: `${interpolate(p, [0, 0.65], [1, zoom], {
        ...clampOpts,
        easing: Easing.in(Easing.cubic),
      })}`,
      filter: `blur(${interpolate(p, [0.15, 0.6], [0, blur], clampOpts)}px)`,
    };
    return <AbsoluteFill style={exitStyle}>{children}</AbsoluteFill>;
  }

  const childStyle: React.CSSProperties = {
    opacity: interpolate(p, [0.3, 0.5], [0, 1], clampOpts),
    scale: `${interpolate(p, [0.3, 0.88, 1], [0.68, 1.02, 1], {
      ...clampOpts,
      easing: Easing.out(Easing.cubic),
    })}`,
    filter: `blur(${interpolate(p, [0.3, 0.75], [blur * 0.7, 0], clampOpts)}px)`,
  };

  return <AbsoluteFill style={childStyle}>{children}</AbsoluteFill>;
};

export function pushThrough(
  props: PushThroughProps = {}
): TransitionPresentation<PushThroughProps> {
  return {
    component: PushThroughPresentation,
    props,
  };
}
