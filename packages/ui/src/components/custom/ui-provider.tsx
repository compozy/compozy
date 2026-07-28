import { LazyMotion, MotionConfig, domAnimation } from "motion/react";
import type { ReactNode } from "react";

import { MOTION_DURATION_BASE, MOTION_EASE_OUT } from "../../lib/motion";

export interface UIProviderProps {
  children: ReactNode;
  reducedMotion?: "user" | "always" | "never";
}

export function UIProvider({ children, reducedMotion = "user" }: UIProviderProps) {
  return (
    <LazyMotion features={domAnimation}>
      <MotionConfig
        reducedMotion={reducedMotion}
        transition={{ duration: MOTION_DURATION_BASE, ease: MOTION_EASE_OUT }}
      >
        {children}
      </MotionConfig>
    </LazyMotion>
  );
}
