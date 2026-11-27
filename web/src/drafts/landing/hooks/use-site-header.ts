import { useMotionValueEvent, useReducedMotion, useScroll, useSpring } from "motion/react";
import { useEffect, useState } from "react";

export function useSiteHeader(scrolled: boolean) {
  const reduced = useReducedMotion();
  const { scrollY, scrollYProgress } = useScroll();
  const [compact, setCompact] = useState(
    () => scrolled || (typeof window !== "undefined" && window.scrollY > 80)
  );
  useMotionValueEvent(scrollY, "change", value => setCompact(scrolled || value > 80));
  useEffect(() => {
    const sync = () => setCompact(scrolled || window.scrollY > 80);
    sync();
    window.addEventListener("scroll", sync, { passive: true });
    return () => window.removeEventListener("scroll", sync);
  }, [scrolled]);
  const width = useSpring(scrollYProgress, { stiffness: 120, damping: 30 });
  const isCompact = scrolled || compact;
  return { reduced, width, isCompact };
}
