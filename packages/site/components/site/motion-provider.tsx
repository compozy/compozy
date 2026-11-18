"use client";

import { LazyMotion, domMax } from "motion/react";
import type { ReactNode } from "react";

/** Loads the motion features the marketing `m.*` components use. */
export function SiteMotionProvider({ children }: { children: ReactNode }) {
  return <LazyMotion features={domMax}>{children}</LazyMotion>;
}
