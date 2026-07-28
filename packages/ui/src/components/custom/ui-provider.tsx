import { LazyMotion, MotionConfig, domAnimation } from "motion/react";
import type { ReactNode } from "react";

import { MOTION_DURATION_BASE, MOTION_EASE_OUT } from "../../lib/motion";

export interface UIProviderProps {
  children: ReactNode;
  reducedMotion?: "user" | "always" | "never";
  skipAnimations?: boolean;
}

export function UIProvider({ children, reducedMotion = "user", skipAnimations }: UIProviderProps) {
  return (
    <LazyMotion features={domAnimation}>
      <MotionConfig
        reducedMotion={reducedMotion}
        skipAnimations={skipAnimations}
        transition={{ duration: MOTION_DURATION_BASE, ease: MOTION_EASE_OUT }}
      >
        {children}
      </MotionConfig>
    </LazyMotion>
  );
}
