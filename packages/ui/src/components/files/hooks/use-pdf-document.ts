import type { PDFDocumentProxy } from "pdfjs-dist";
import * as React from "react";

type PdfLib = typeof import("pdfjs-dist");

export type PdfLoadState =
  | { status: "loading" }
  | { status: "error" }
  | { status: "ready"; pdf: PDFDocumentProxy; aspect: number };

let workerPort: Worker | null = null;
let libPromise: Promise<PdfLib> | null = null;

/** pdf.js loads on first use and shares one worker across every preview. */
function loadPdfLib(workerFactory: () => Worker): Promise<PdfLib> {
  libPromise ??= import("pdfjs-dist").then(lib => {
    workerPort ??= workerFactory();
    lib.GlobalWorkerOptions.workerPort = workerPort;
    return lib;
  });
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
      .then(async lib => {
        // Wasm decoders would need `wasm-unsafe-eval` under the daemon CSP; the JS paths suffice.
        const task = lib.getDocument({ url, withCredentials: true, useWasm: false });
        destroy = () => void task.destroy();
        const pdf = await task.promise;
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
