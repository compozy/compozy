import * as React from "react";

/**
 * An element's own inline size — the pane-width question a viewport media
 * query cannot answer for a tiled window. The node is measured before paint
 * (no first-frame flash) and then follows resizes. An unmeasured element (no
 * layout yet, or no `ResizeObserver`) reads as `null`, so callers default to
 * the full control set.
 */
export function useInlineSize<T extends HTMLElement>(): readonly [
  (node: T | null) => void,
  number | null,
] {
  const [node, setNode] = React.useState<T | null>(null);
  const [width, setWidth] = React.useState<number | null>(null);

  React.useLayoutEffect(() => {
    if (!node) return;
    const update = (next: number) => setWidth(next === 0 ? null : next);
    update(node.getBoundingClientRect().width);
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(entries => {
      const entry = entries[0];
      if (entry) update(entry.contentRect.width);
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, [node]);

  return [setNode, width] as const;
}
