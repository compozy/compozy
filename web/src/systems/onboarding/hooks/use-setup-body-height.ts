import { useEffect, useState } from "react";

/** Head (52) + step strip (44) + footer (58) — the body gets whatever is left. */
const PANEL_CHROME_HEIGHT = 154;
/** The scrim keeps a 24px gutter on both edges. */
const SCRIM_INSET = 48;
/** Below Tailwind's `md` the panel is a full-bleed sheet with an auto-height body. */
const SHEET_MAX_WIDTH = 767;

export interface SetupBodyHeight {
  /** Attach to the active pane's content box — its natural height is the measure. */
  measureRef: (node: HTMLElement | null) => void;
  /** Pixel height for the animated body, or null while unmeasured / as a sheet. */
  height: number | null;
}

/**
 * The body never outgrows what the viewport leaves after the panel chrome: the
 * footer carries the only way forward, so a short window shrinks the body and
 * scrolls it rather than pushing `Continue` outside an `overflow-hidden` panel.
 * `content <= 0` (unlaid-out) and a negative budget both hand sizing back to
 * flex instead of pinning a wrong number.
 */
export function resolveSetupBodyHeight(content: number, viewportHeight: number): number | null {
  if (content <= 0) return null;
  const available = viewportHeight - SCRIM_INSET - PANEL_CHROME_HEIGHT;
  if (available <= 0) return null;
  return Math.min(content, available);
}

/**
 * Each step owns its own shape: the body animates to the height of whatever pane
 * is mounted, clamped so a tall step scrolls inside the viewport instead of
 * pushing the footer off-screen.
 */
export function useSetupBodyHeight(): SetupBodyHeight {
  const [node, setNode] = useState<HTMLElement | null>(null);
  const [height, setHeight] = useState<number | null>(null);

  useEffect(() => {
    // Between panes React clears then sets the ref in one commit; keeping the
    // last height is what the new pane animates away from.
    if (node === null) return undefined;
    const measure = () => {
      if (window.matchMedia(`(max-width: ${SHEET_MAX_WIDTH}px)`).matches) {
        setHeight(null);
        return;
      }
      setHeight(resolveSetupBodyHeight(Math.ceil(node.offsetHeight), window.innerHeight));
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(node);
    window.addEventListener("resize", measure);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", measure);
    };
  }, [node]);

  return { measureRef: setNode, height };
}
