import { useCallback, useState } from "react";

/**
 * Whether a line-clamped element actually hides content: its scroll height
 * exceeds its box. Re-measured whenever the element resizes, so the answer
 * follows the window width and the clamp toggling on and off.
 */
export function useClampOverflow<T extends HTMLElement>() {
  const [overflowing, setOverflowing] = useState(false);
  const observe = useCallback((node: T | null) => {
    if (!node) return;
    // One pixel of slack absorbs sub-pixel line-height rounding.
    const measure = () => setOverflowing(node.scrollHeight > node.clientHeight + 1);
    measure();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(measure);
    observer.observe(node);
    return () => observer.disconnect();
  }, []);
  return { observe, overflowing };
}
