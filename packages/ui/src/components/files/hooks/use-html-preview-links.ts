import * as React from "react";

/**
 * Routes link clicks inside a same-origin preview frame to `onLinkClick` instead of navigating
 * the frame; in-page `#anchors` scroll. Shortcut keys pressed inside the frame (Cmd/Ctrl chords,
 * Escape) re-dispatch on the frame element, so the host's key handlers still see them. The
 * listeners are re-attached on every frame load and removed on unmount.
 */
export function useHtmlPreviewLinks(
  frameRef: React.RefObject<HTMLIFrameElement | null>,
  onLinkClick: ((url: string) => void) | undefined
) {
  const onLinkClickRef = React.useRef(onLinkClick);
  React.useEffect(() => {
    onLinkClickRef.current = onLinkClick;
  });
  React.useEffect(() => {
    const frame = frameRef.current;
    if (!frame) return;
    let attached: Document | null = null;
    const onClick = (event: MouseEvent) => {
      // The frame is its own realm, so `instanceof` against this window's classes never matches.
      const node = event.target as Element | null;
      const anchor = (
        typeof node?.closest === "function" ? node.closest("a[href], area[href]") : null
      ) as HTMLAnchorElement | null;
      if (!anchor) return;
      event.preventDefault();
      if (anchor.getAttribute("href")?.startsWith("#")) {
        attached?.getElementById(decodeURIComponent(anchor.hash.slice(1)))?.scrollIntoView();
        return;
      }
      onLinkClickRef.current?.(anchor.href);
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (!event.metaKey && !event.ctrlKey && event.key !== "Escape") return;
      const forwarded = new KeyboardEvent("keydown", {
        key: event.key,
        code: event.code,
        metaKey: event.metaKey,
        ctrlKey: event.ctrlKey,
        shiftKey: event.shiftKey,
        altKey: event.altKey,
        repeat: event.repeat,
        bubbles: true,
        cancelable: true,
      });
      if (!frame.dispatchEvent(forwarded)) event.preventDefault();
    };
    const detach = () => {
      attached?.removeEventListener("click", onClick);
      attached?.removeEventListener("keydown", onKeyDown);
    };
    const attach = () => {
      detach();
      attached = frame.contentDocument;
      attached?.addEventListener("click", onClick);
      attached?.addEventListener("keydown", onKeyDown);
    };
    attach();
    frame.addEventListener("load", attach);
    return () => {
      frame.removeEventListener("load", attach);
      detach();
    };
  }, [frameRef]);
}
