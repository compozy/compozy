// Vite-only entry: `?worker` bundles the pdf.js worker as a same-origin module worker (the daemon
// CSP allows `worker-src 'self' blob:`). It is reached as `@compozy/ui/lib/pdf-worker`, outside
// the root barrel, so non-Vite consumers of `@compozy/ui` never load it.
import PdfWorker from "pdfjs-dist/build/pdf.worker.min.mjs?worker";

export function createPdfWorker(): Worker {
  return new PdfWorker();
}
