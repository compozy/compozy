import { useEffect, useEffectEvent, useRef, useState } from "react";

import { prototypeParams } from "#gallery/contract";

/** The document on screen in the frame, for keyboard shortcuts and review overlays. */
export type FrameInfo = { doc: Document; slug: string; state: string | null };

/**
 * Drives one prototype iframe: the first `src` loads normally, later ones rewrite the location in
 * place (no reload when only the query changes), and every load or change reports the document.
 */
export function useFrame(src: string, onInfo?: (info: FrameInfo | null) => void) {
  const ref = useRef<HTMLIFrameElement>(null);
  const [initialSrc] = useState(src);
  const [loadCount, setLoadCount] = useState(0);

  useEffect(() => {
    const view = ref.current?.contentWindow;
    if (!view) return;
    const target = new URL(src, window.location.href);
    try {
      if (view.location.pathname !== target.pathname) {
        view.location.replace(target.href);
      } else if (view.location.search !== target.search) {
        view.history.replaceState(null, "", target.href);
        view.dispatchEvent(new PopStateEvent("popstate"));
      }
    } catch {
      // Still on the initial about:blank document: load the target instead.
      view.location.replace(target.href);
    }
  }, [src]);

  const report = useEffectEvent((info: FrameInfo) => onInfo?.(info));

  useEffect(() => {
    const doc = ref.current?.contentDocument;
    if (!doc || loadCount === 0) return;
    report({
      doc,
      slug: doc.location.pathname.replace(/^\/+|\/+$/g, ""),
      state: new URLSearchParams(doc.location.search).get(prototypeParams.state),
    });
  }, [loadCount, src]);

  return { ref, initialSrc, onLoad: () => setLoadCount(count => count + 1) };
}
