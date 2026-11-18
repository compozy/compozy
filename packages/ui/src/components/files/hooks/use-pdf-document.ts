import type { PDFDocumentProxy, PDFWorker } from "pdfjs-dist";
import * as React from "react";

type PdfLib = typeof import("pdfjs-dist");

export type PdfLoadState =
  | { status: "loading" }
  | { status: "error" }
  | { status: "ready"; pdf: PDFDocumentProxy; aspect: number };

let libPromise: Promise<{ lib: PdfLib; worker: PDFWorker }> | null = null;

/**
 * pdf.js loads on first use with one worker shared by every preview. Each load passes that
 * worker explicitly, so destroying a document's loading task never tears the shared worker
 * down under the next load.
 */
function loadPdfLib(workerFactory: () => Worker): Promise<{ lib: PdfLib; worker: PDFWorker }> {
  libPromise ??= import("pdfjs-dist").then(lib => ({
    lib,
    worker: lib.PDFWorker.create({ port: workerFactory() }),
  }));
  return libPromise;
}

/** Loads the PDF at `url` (with credentials) and the first page's aspect for placeholders. */
export function usePdfDocument(url: string, workerFactory: () => Worker): PdfLoadState {
  const [state, setState] = React.useState<{ url: string; load: PdfLoadState }>({
    url,
    load: { status: "loading" },
  });
  React.useEffect(() => {
    let cancelled = false;
    let destroy: (() => void) | null = null;
    void loadPdfLib(workerFactory)
      .then(async ({ lib, worker }) => {
        if (cancelled) return;
        // Wasm decoders would need `wasm-unsafe-eval` under the daemon CSP; the JS paths suffice.
        const task = lib.getDocument({ url, worker, withCredentials: true, useWasm: false });
        destroy = () => void task.destroy();
        const pdf = await task.promise;
        if (cancelled) return;
        const first = await pdf.getPage(1);
        const viewport = first.getViewport({ scale: 1 });
        if (!cancelled) {
          setState({
            url,
            load: { status: "ready", pdf, aspect: viewport.height / viewport.width },
          });
        }
      })
      .catch(() => {
        if (!cancelled) setState({ url, load: { status: "error" } });
      });
    return () => {
      cancelled = true;
      destroy?.();
    };
  }, [url, workerFactory]);
  return state.url === url ? state.load : { status: "loading" };
}
