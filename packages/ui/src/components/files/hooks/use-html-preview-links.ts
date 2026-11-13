import * as React from "react";

/**
 * Routes link clicks inside a same-origin preview frame to `onLinkClick` instead of navigating
 * the frame; in-page `#anchors` scroll. The listener is re-attached on every frame load and
 * removed on unmount.
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
        typeof node?.closest === "function" ? node.closest("a[href]") : null
      ) as HTMLAnchorElement | null;
      if (!anchor) return;
      event.preventDefault();
      if (anchor.getAttribute("href")?.startsWith("#")) {
        attached?.getElementById(decodeURIComponent(anchor.hash.slice(1)))?.scrollIntoView();
        return;
      }
      onLinkClickRef.current?.(anchor.href);
    };
    const attach = () => {
      attached?.removeEventListener("click", onClick);
      attached = frame.contentDocument;
      attached?.addEventListener("click", onClick);
    };
    attach();
    frame.addEventListener("load", attach);
    return () => {
      frame.removeEventListener("load", attach);
      attached?.removeEventListener("click", onClick);
    };
  }, [frameRef]);
}
