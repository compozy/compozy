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
  // Set while a new document loads: the old one is still in the frame and must not be reported.
  const navigating = useRef(false);

  useEffect(() => {
    const view = ref.current?.contentWindow;
    if (!view) return;
    const target = new URL(src, window.location.href);
    try {
      if (view.location.pathname !== target.pathname) {
        navigating.current = true;
        view.location.replace(target.href);
      } else if (view.location.search !== target.search) {
        view.history.replaceState(null, "", target.href);
        view.dispatchEvent(new PopStateEvent("popstate"));
      }
    } catch {
      // Still on the initial about:blank document: load the target instead.
      navigating.current = true;
      view.location.replace(target.href);
    }
  }, [src]);

  const report = useEffectEvent((info: FrameInfo) => onInfo?.(info));

  useEffect(() => {
    const doc = ref.current?.contentDocument;
    if (!doc || loadCount === 0 || navigating.current) return;
    report({
      doc,
      slug: doc.location.pathname.replace(/^\/+|\/+$/g, ""),
      state: new URLSearchParams(doc.location.search).get(prototypeParams.state),
    });
  }, [loadCount, src]);

  const onLoad = () => {
    navigating.current = false;
    setLoadCount(count => count + 1);
  };
  return { ref, initialSrc, onLoad };
}
