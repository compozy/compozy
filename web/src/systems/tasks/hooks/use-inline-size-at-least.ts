import * as React from "react";

/**
 * Whether an element's own inline size is at least `minWidth` — the pane-width
 * question a viewport media query cannot answer for a tiled window. The node is
 * measured before paint (no first-frame flash) and then follows resizes. An
 * unmeasured element (no layout yet, or no `ResizeObserver`) reads as fitting,
 * so the full control set is the default.
 */
export function useInlineSizeAtLeast<T extends HTMLElement>(
  minWidth: number
): readonly [(node: T | null) => void, boolean] {
  const [node, setNode] = React.useState<T | null>(null);
  const [fits, setFits] = React.useState(true);

  React.useLayoutEffect(() => {
    if (!node) return;
    const update = (width: number) => setFits(width === 0 || width >= minWidth);
    update(node.getBoundingClientRect().width);
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(entries => {
      const entry = entries[0];
      if (entry) update(entry.contentRect.width);
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, [node, minWidth]);

  return [setNode, fits] as const;
}
