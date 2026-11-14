import * as React from "react";

/**
 * The content width of the element the returned callback ref is attached to, kept current with a
 * ResizeObserver. A replacement element (after an error state unmounts the first) is observed
 * in its place.
 */
export function useElementWidth(): [ref: (element: HTMLElement | null) => void, width: number] {
  const [element, setElement] = React.useState<HTMLElement | null>(null);
  const [width, setWidth] = React.useState(0);
  React.useEffect(() => {
    if (!element) return;
    const observer = new ResizeObserver(entries => {
      setWidth(Math.floor(entries[0]?.contentRect.width ?? 0));
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, [element]);
  return [setElement, width];
}
