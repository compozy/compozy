import { useScroll } from "motion/react";
import { useRef } from "react";

/** Holds its content on screen while `length` of page scrolls past, and hands over the progress. */
export function useStage() {
  const ref = useRef<HTMLDivElement>(null);
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start start", "end end"] });
  return { ref, progress: scrollYProgress };
}
