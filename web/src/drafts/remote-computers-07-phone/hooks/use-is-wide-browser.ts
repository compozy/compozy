import { useLayoutEffect, useState, type RefObject } from "react";

export const WIDE_BROWSER_BREAKPOINT_PX = 1024;

/**
 * Observes the rendered container width (and initializes synchronously from
 * `window.innerWidth`) so phone (<1024 px) vs web app shell (≥1024 px)
 * switches reliably in both standalone viewports and gallery iframes: phone layout below,
 * the CompozyOS web app shell at and above the breakpoint.
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
