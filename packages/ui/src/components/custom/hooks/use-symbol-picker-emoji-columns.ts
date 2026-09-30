import * as React from "react";

/** Inline padding of one emoji row (`px-0.5` on each side). */
const ROW_INLINE_PADDING_PX = 4;

/**
 * Emoji columns that fill the pane: frimousse lays out a fixed column count,
 * so the count follows the viewport width divided by the shared picker cell,
 * the same cell the icon grid auto-fills with. Until the pane is measured the
 * picker keeps its own default.
 */
export function useSymbolPickerEmojiColumns() {
  const [columns, setColumns] = React.useState<number | undefined>(undefined);
  const cellProbe = React.useRef<HTMLSpanElement>(null);
  const viewport = React.useRef<HTMLDivElement>(null);

  React.useEffect(() => {
    const node = viewport.current;
    if (!node) return;
    const measure = () => {
      const cell = cellProbe.current?.offsetWidth ?? 0;
      const width = node.clientWidth - ROW_INLINE_PADDING_PX;
      if (cell <= 0 || width <= 0) return;
      setColumns(Math.max(1, Math.floor(width / cell)));
    };
    measure();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(measure);
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  return { columns, cellProbe, viewport };
}
