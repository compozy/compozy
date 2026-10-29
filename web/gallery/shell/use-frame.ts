import { useEffect, useEffectEvent, useRef, useState } from "react";

import { prototypeParams } from "#gallery/contract";

/** The document on screen in the frame, for keyboard shortcuts and review overlays. */
export type FrameInfo = { doc: Document; slug: string; state: string | null };

function infoOf(doc: Document): FrameInfo {
  return {
    doc,
    slug: doc.location.pathname.replace(/^\/+|\/+$/g, ""),
    state: new URLSearchParams(doc.location.search).get(prototypeParams.state),
  };
}

/**
 * Drives one prototype iframe: the first `src` loads normally, later ones rewrite the location in
 * place (no reload when only the query changes). The document is reported when it loads and on
 * each in-place change (`popstate`), never while a new document is still loading.
 */
export function useFrame(src: string, onInfo?: (info: FrameInfo | null) => void) {
  const ref = useRef<HTMLIFrameElement>(null);
  const [initialSrc] = useState(src);
  const [loaded, setLoaded] = useState<Document | null>(null);

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

  const report = useEffectEvent((doc: Document) => onInfo?.(infoOf(doc)));

  useEffect(() => {
    const view = loaded?.defaultView;
    if (!loaded || !view) return;
    const onPopState = () => report(loaded);
    view.addEventListener("popstate", onPopState);
    return () => view.removeEventListener("popstate", onPopState);
  }, [loaded]);

  const onLoad = () => {
    const doc = ref.current?.contentDocument;
    if (!doc) return;
    setLoaded(doc);
    onInfo?.(infoOf(doc));
  };
  return { ref, initialSrc, onLoad };
}
