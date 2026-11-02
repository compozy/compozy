import { useEffect, useRef } from "react";

/**
 * Reports when the topbar has clipped the observed element. The status zone is end-aligned and
 * sheds from its start, so the chip is the first thing to go, at a width that depends on the title
 * and on everything beside it. Only the rendered geometry can answer that.
 */
export function useSessionTopbarClipped(onClippedChange: ((clipped: boolean) => void) | undefined) {
  const ref = useRef<HTMLSpanElement>(null);
  useEffect(() => {
    const node = ref.current;
    const root = node?.closest("[data-slot=topbar-status]");
    if (!node || !root || !onClippedChange || typeof IntersectionObserver === "undefined") {
      return undefined;
    }
    const observer = new IntersectionObserver(
      entries => {
        const entry = entries.at(-1);
        if (entry) onClippedChange(entry.intersectionRatio < 1);
      },
      { root, threshold: [1] }
    );
    observer.observe(node);
    return () => observer.disconnect();
  }, [onClippedChange]);
  return ref;
}
