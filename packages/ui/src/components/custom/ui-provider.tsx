import { LucideProvider } from "lucide-react";
import { LazyMotion, MotionConfig, domAnimation } from "motion/react";
import type { ReactNode } from "react";

import { ICON_STROKE_WIDTH } from "../../lib/icon-stroke";
import { MOTION_DURATION_BASE, MOTION_EASE_OUT } from "../../lib/motion";

export interface UIProviderProps {
  children: ReactNode;
  reducedMotion?: "user" | "always" | "never";
  skipAnimations?: boolean;
}

/**
 * App-wide runtime defaults: motion config and the single lucide stroke width
 * (every lucide icon under the provider draws at `ICON_STROKE_WIDTH` unless the
 * call site overrides it).
 */
export function UIProvider({ children, reducedMotion = "user", skipAnimations }: UIProviderProps) {
  return (
    <LucideProvider strokeWidth={ICON_STROKE_WIDTH}>
      <LazyMotion features={domAnimation}>
        <MotionConfig
          reducedMotion={reducedMotion}
          skipAnimations={skipAnimations}
          transition={{ duration: MOTION_DURATION_BASE, ease: MOTION_EASE_OUT }}
        >
          {children}
        </MotionConfig>
      </LazyMotion>
    </LucideProvider>
  );
}
