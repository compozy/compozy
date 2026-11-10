"use client";

/**
 * The comparison's check mark. The HTML serves it in place; a table still below the fold at
 * hydration pops its checks in row by row when it arrives.
 */
import { Check } from "@compozy/ui/icons";
import { m } from "motion/react";

import { useArrival } from "@/components/site/hooks/use-arrival";

export function PopCheck({ index }: { index: number }) {
  const { ref, progress } = useArrival<HTMLSpanElement>({
    delay: 0.1 * index,
    type: "spring",
    stiffness: 320,
    damping: 14,
  });
  return (
    <m.span ref={ref} className="inline-block" style={{ scale: progress }}>
      <Check aria-label="Yes" className="size-5 text-white" />
    </m.span>
  );
}
