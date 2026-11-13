import type { PDFDocumentProxy, RenderTask } from "pdfjs-dist";
import * as React from "react";

/**
 * One PDF page: it stays a sized placeholder until it scrolls near the viewport, then draws into
 * a HiDPI canvas at `width`, cancelling the draw when it unmounts or resizes.
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
  React.useEffect(() => {
    const frame = frameRef.current;
    if (!frame) return;
    const observer = new IntersectionObserver(
      entries => {
        if (entries.some(entry => entry.isIntersecting)) {
          setNear(true);
          observer.disconnect();
        }
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
    void pdf.getPage(pageNumber).then(page => {
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
      task.promise.catch(() => undefined);
    });
    return () => {
      cancelled = true;
      task?.cancel();
    };
  }, [near, pdf, pageNumber, width]);
  return { frameRef, canvasRef, near, height: Math.round(width * pageAspect) };
}
