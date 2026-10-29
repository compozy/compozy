import { useEffect, useState } from "react";

import { locateTarget, type Located, type Size } from "#gallery/shell/comments/dom";
import type { ReviewComment } from "#gallery/shell/comments/model";

function same(a: ReadonlyMap<string, Located>, b: ReadonlyMap<string, Located>) {
  return JSON.stringify([...a]) === JSON.stringify([...b]);
}

/**
 * Where each element/region comment sits in the frame now, re-measured whenever the prototype
 * mutates, scrolls or resizes (once per animation frame).
 */
export function useLocated(doc: Document | null, comments: readonly ReviewComment[], canvas: Size) {
  const [located, setLocated] = useState<ReadonlyMap<string, Located>>(new Map());
  const { width, height } = canvas;
  useEffect(() => {
    if (!doc?.body) return;
    let frame = 0;
    const measure = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        const next = new Map<string, Located>();
        for (const comment of comments) {
          if (comment.target.kind === "node") continue;
          next.set(comment.id, locateTarget(doc, comment.target, { width, height }));
        }
        // Most mutations move nothing: keep the same map and skip the render.
        setLocated(previous => (same(previous, next) ? previous : next));
      });
    };
    measure();
    const view = doc.defaultView;
    const observer = new MutationObserver(measure);
    observer.observe(doc.body, { subtree: true, childList: true, attributes: true });
    doc.addEventListener("scroll", measure, true);
    view?.addEventListener("resize", measure);
    // A web font that finishes loading reflows text without a mutation.
    doc.fonts.addEventListener("loadingdone", measure);
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
      doc.removeEventListener("scroll", measure, true);
      view?.removeEventListener("resize", measure);
      doc.fonts.removeEventListener("loadingdone", measure);
    };
  }, [doc, comments, width, height]);
  return located;
}
