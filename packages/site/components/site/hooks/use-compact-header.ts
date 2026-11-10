"use client";

import { useMotionValueEvent, useScroll, useSpring } from "motion/react";
import { useState } from "react";

/** The header compacts after 80px of scroll; `progress` drives the reading-progress hairline. */
export function useCompactHeader() {
  const { scrollY, scrollYProgress } = useScroll();
  const [compact, setCompact] = useState(false);
  useMotionValueEvent(scrollY, "change", value => setCompact(value > 80));
  const progress = useSpring(scrollYProgress, { stiffness: 120, damping: 30 });
  return { compact, progress };
}
