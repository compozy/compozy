import { useLayoutEffect, useState, type ReactNode, type RefObject } from "react";

import { cn } from "@compozy/ui";

export const WIDE_BROWSER_BREAKPOINT_PX = 1024;

/**
 * Observes the rendered container width (and initializes synchronously from
 * `window.innerWidth`) so phone (<1024 px) vs two-pane web app (≥1024 px)
 * switches reliably in both standalone viewports and gallery iframes.
 */
export function useIsWideBrowser(
  containerRef: RefObject<HTMLElement | null>,
  breakpointPx = WIDE_BROWSER_BREAKPOINT_PX
): boolean {
  const [isWide, setIsWide] = useState<boolean>(() => {
    if (typeof window === "undefined") return false;
    return window.innerWidth >= breakpointPx;
  });

  useLayoutEffect(() => {
    const element = containerRef.current;
    if (!element) return;

    const update = (width: number) => {
      if (width > 0) {
        setIsWide(width >= breakpointPx);
      }
    };

    update(element.getBoundingClientRect().width);

    if (typeof ResizeObserver === "undefined") {
      const handleResize = () => update(window.innerWidth);
      window.addEventListener("resize", handleResize);
      return () => window.removeEventListener("resize", handleResize);
    }

    const observer = new ResizeObserver(entries => {
      const entry = entries[0];
      if (entry) {
        update(entry.contentRect.width);
      }
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, [containerRef, breakpointPx]);

  return isWide;
}

export interface PhoneFrameProps {
  children: ReactNode;
  className?: string;
}

/**
 * Full-viewport root container for the hosted-route browser SPA (Screen 07 / U8).
 * Works cleanly at 390 px (phone layout) and ≥1024 px (two-pane web app / centered gate).
 */
export function PhoneFrame({ children, className }: PhoneFrameProps) {
  return (
    <div
      className={cn(
        "relative flex h-dvh min-h-dvh w-full flex-col overflow-hidden bg-canvas text-fg",
        className
      )}
      data-slot="phone-frame"
    >
      {children}
    </div>
  );
}
