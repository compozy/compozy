"use client";

import { transform, useTransform, type MotionValue } from "motion/react";

/**
 * `useTransform` over scroll progress, computed on the main thread. Motion otherwise hands a ranged
 * transform to a native view timeline, whose range honours the page's `scroll-padding-top` and
 * starts the effect early (the hero copy loads faded). The JS-measured progress has no such shift.
 */
export function useScrollTransform<T>(
  progress: MotionValue<number>,
  input: number[],
  output: T[]
): MotionValue<T> {
  const map = transform(input, output);
  return useTransform(progress, value => map(value));
}
