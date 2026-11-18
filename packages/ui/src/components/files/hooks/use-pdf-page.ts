import type { PDFDocumentProxy, RenderTask } from "pdfjs-dist";
import * as React from "react";

/**
 * One PDF page: a sized placeholder while it is far from the viewport, drawn into a HiDPI canvas
 * at `width` while it is near. Leaving the neighbourhood drops the canvas, so a long document
 * holds bitmaps only for the pages around the scroll position; the draw cancels on unmount,
 * resize or scroll-away.
 */
export function usePdfPage(
  pdf: PDFDocumentProxy,
  pageNumber: number,
  width: number,
  aspect: number
) {
  const frameRef = React.useRef<HTMLDivElement | null>(null);
  const canvasRef = React.useRef<HTMLCanvasElement | null>(null);
  const [near, setNear] = React.useState(false);
  const [pageAspect, setPageAspect] = React.useState(aspect);
  const [failed, setFailed] = React.useState(false);
  // The page's text for assistive technology, read once the page first comes near.
  const [text, setText] = React.useState<string | null>(null);
  React.useEffect(() => {
    const frame = frameRef.current;
    if (!frame) return;
    const observer = new IntersectionObserver(
      entries => {
        const entry = entries.at(-1);
        if (entry) setNear(entry.isIntersecting);
      },
      { rootMargin: "800px 0px" }
    );
    observer.observe(frame);
    return () => observer.disconnect();
  }, []);
  React.useEffect(() => {
    if (!near || width <= 0) return;
    let task: RenderTask | null = null;
    let cancelled = false;
    const draw = pdf.getPage(pageNumber).then(page => {
      const canvas = canvasRef.current;
      if (cancelled || !canvas) return;
      const base = page.getViewport({ scale: 1 });
      const viewport = page.getViewport({ scale: width / base.width });
      const ratio = window.devicePixelRatio || 1;
      canvas.width = Math.floor(viewport.width * ratio);
      canvas.height = Math.floor(viewport.height * ratio);
      setPageAspect(base.height / base.width);
      task = page.render({
        canvas,
        viewport,
        transform: ratio === 1 ? undefined : [ratio, 0, 0, ratio, 0, 0],
      });
      return task.promise;
    });
    draw.catch((error: unknown) => {
      // A cancelled draw rejects too; only a page that could not load or draw is a failure.
      if (
        !cancelled &&
        (error as { name?: string } | null)?.name !== "RenderingCancelledException"
      ) {
        setFailed(true);
      }
    });
    return () => {
      cancelled = true;
      task?.cancel();
    };
  }, [near, pdf, pageNumber, width]);
  React.useEffect(() => {
    if (!near || text !== null) return;
    let cancelled = false;
    pdf
      .getPage(pageNumber)
      .then(page => page.getTextContent())
      .then(content => {
        if (cancelled) return;
        const parts = content.items.map(item =>
          "str" in item ? `${item.str}${item.hasEOL ? "\n" : ""}` : ""
        );
        setText(parts.join(""));
      })
      .catch(() => {
        if (!cancelled) setText("");
      });
    return () => {
      cancelled = true;
    };
  }, [near, pdf, pageNumber, text]);
  return { frameRef, canvasRef, near, failed, text, height: Math.round(width * pageAspect) };
}
