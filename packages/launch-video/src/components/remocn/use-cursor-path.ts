// Source: https://remocn.dev — copied from the registry; MIT.
"use client";

import { useCurrentFrame } from "remotion";
import type { CursorStyle } from "@/components/remocn/cursor";
import { clamp01, type EasingName, easings } from "@/lib/remocn-ui";

export interface CursorWaypoint {
  at: number;
  x: number;
  y: number;
  duration?: number;
  click?: boolean;
  press?: boolean;
  easing?: EasingName;
}

export const DEFAULT_DURATION = 24;

export const CLICK_FRAMES = 16;

export const PRESS_FRAMES = 8;

export interface CursorPathOptions {
  speed?: number;
  defaultDuration?: number;
}

function lerp(a: number, b: number, t: number): number {
  return a + (b - a) * t;
}

export function ripplePhase(framesSinceClick: number): {
  rippleOpacity: number;
  rippleScale: number;
} {
  if (framesSinceClick < 0 || framesSinceClick >= CLICK_FRAMES) {
    return { rippleOpacity: 0, rippleScale: 0 };
  }
  const p = clamp01(framesSinceClick / CLICK_FRAMES);
  return {
    rippleOpacity: 0.5 * (1 - p),
    rippleScale: 2.5 * easings.out(p),
  };
}

export function clickPress(framesSinceClick: number): number {
  if (framesSinceClick < 0 || framesSinceClick >= PRESS_FRAMES) return 1;
  const half = PRESS_FRAMES / 2;
  const p =
    framesSinceClick < half ? framesSinceClick / half : 1 - (framesSinceClick - half) / half;
  return 1 - clamp01(p);
}

export function useCursorPath(
  waypoints: CursorWaypoint[],
  opts: CursorPathOptions = {}
): CursorStyle {
  const raw = useCurrentFrame();
  return cursorPathAt(waypoints, raw, opts);
}

export function cursorPathAt(
  waypoints: CursorWaypoint[],
  raw: number,
  opts: CursorPathOptions = {}
): CursorStyle {
  const { speed = 1, defaultDuration = DEFAULT_DURATION } = opts;
  const effectiveFrame = raw * speed;

  if (waypoints.length === 0) {
    return { x: 0, y: 0, scale: 1, rippleOpacity: 0, rippleScale: 0 };
  }

  const first = waypoints[0]!;

  if (effectiveFrame <= first.at) {
    return {
      x: first.x,
      y: first.y,
      scale: 1,
      rippleOpacity: 0,
      rippleScale: 0,
      pressScale: 1,
    };
  }

  let toIndex = waypoints.length - 1;
  for (let i = 1; i < waypoints.length; i++) {
    if (waypoints[i]!.at > effectiveFrame) {
      toIndex = i;
      break;
    }
  }
  const pastLast = effectiveFrame >= waypoints[waypoints.length - 1]!.at;
  const to = pastLast ? waypoints[waypoints.length - 1]! : waypoints[toIndex]!;
  const from = pastLast ? waypoints[waypoints.length - 1]! : waypoints[toIndex - 1]!;

  const dur = to.duration ?? defaultDuration;
  const ease = easings[to.easing ?? "inOut"];
  const start = to.at - dur;
  const t = pastLast || dur <= 0 ? 1 : ease(clamp01((effectiveFrame - start) / dur));
  const x = lerp(from.x, to.x, t);
  const y = lerp(from.y, to.y, t);

  let lastClickAt = -Infinity;
  for (const wp of waypoints) {
    if (wp.click && wp.at <= effectiveFrame && wp.at > lastClickAt) lastClickAt = wp.at;
  }
  const sinceClick = lastClickAt === -Infinity ? -1 : effectiveFrame - lastClickAt;
  const ripple = ripplePhase(sinceClick);
  const clickDip = clickPress(sinceClick);

  const holdWp = pastLast ? waypoints[waypoints.length - 1]! : from;
  const heldPress = holdWp.press ? 0 : 1;

  const pressScale = Math.min(clickDip, heldPress);

  return {
    x,
    y,
    scale: 1,
    rippleOpacity: ripple.rippleOpacity,
    rippleScale: ripple.rippleScale,
    pressScale,
  };
}
