"use client";

import { useEffect, useReducer, useRef, type ReactNode } from "react";
import { cn } from "@agh/ui/lib/utils";
import { useReducedMotion } from "@/components/landing/primitives/use-reduced-motion";

interface AnimatedDiagramProps {
  children: (ctx: { active: boolean; reducedMotion: boolean }) => ReactNode;
  /** IntersectionObserver threshold at which the diagram is considered visible. */
  threshold?: number;
  className?: string;
  /** Optional a11y label for the diagram wrapper. */
  ariaLabel?: string;
}

/**
 * Wraps a diagram that should start animating when it scrolls into view and
 * short-circuit when the user prefers reduced motion. The render prop receives
 * both signals so the diagram can choose its own playback policy.
 */
export function AnimatedDiagram({
  children,
  threshold = 0.2,
  className,
  ariaLabel,
}: AnimatedDiagramProps) {
  const reducedMotion = useReducedMotion();
  const ref = useRef<HTMLDivElement | null>(null);
  const [active, dispatchActive] = useReducer(
    (_active: boolean, nextActive: boolean) => nextActive,
    false
  );

  useEffect(() => {
    const node = ref.current;
    if (!node) return;
    if (reducedMotion) {
      dispatchActive(false);
      return;
    }
    if (typeof IntersectionObserver === "undefined") {
      // Non-browser or legacy env , fall through and auto-activate so the
      // diagram still renders its primary content.
      dispatchActive(true);
      return;
    }
    const observer = new IntersectionObserver(
      entries => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            dispatchActive(true);
            observer.disconnect();
            break;
          }
        }
      },
      { threshold }
    );
    observer.observe(node);
    return () => observer.disconnect();
  }, [threshold, reducedMotion]);

  return (
    <div
      ref={ref}
      className={cn("relative", className)}
      aria-label={ariaLabel}
      role={ariaLabel ? "group" : undefined}
    >
      {children({ active, reducedMotion })}
    </div>
  );
}
