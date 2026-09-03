"use client";

import { useEffect, useRef, useState } from "react";

/**
 * Below this, the window can no longer host the project deck without
 * collapsing identity. Matches the catalog floor that used to block tiling.
 */
const COMPACT_WIDTH_PX = 420;
const COMPACT_HEIGHT_PX = 280;

export function useCompactTerminalWindow(): {
  rootRef: React.RefObject<HTMLDivElement | null>;
  compact: boolean;
} {
  const rootRef = useRef<HTMLDivElement | null>(null);
  const [compact, setCompact] = useState(false);

  useEffect(() => {
    const node = rootRef.current;
    const view = node?.ownerDocument.defaultView;
    if (!node || !view || typeof view.ResizeObserver !== "function") return undefined;
    const observer = new view.ResizeObserver(entries => {
      const entry = entries[0];
      if (!entry) return;
      const { width, height } = entry.contentRect;
      // An unmeasured box (jsdom, first layout) is not compact — collapsing
      // the deck on 0×0 would hide tabs the window has not yet sized.
      setCompact(
        width > 0 && height > 0 && (width < COMPACT_WIDTH_PX || height < COMPACT_HEIGHT_PX)
      );
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  return { rootRef, compact };
}
