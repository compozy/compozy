"use client";

import type { PDFDocumentProxy } from "pdfjs-dist";
import * as React from "react";

import { cn } from "../../lib/utils";
import { useElementWidth } from "./hooks/use-element-width";
import { usePdfDocument } from "./hooks/use-pdf-document";
import { usePdfPage } from "./hooks/use-pdf-page";

export interface FilePdfPreviewProps extends Omit<React.ComponentProps<"div">, "children"> {
  /** Same-origin URL of the PDF bytes; requested with credentials. */
  url: string;
  /** Builds the pdf.js worker (`createPdfWorker` from `@compozy/ui/lib/pdf-worker`). */
  workerFactory: () => Worker;
  /** Accessible name of the document region, such as the file name. */
  title: string;
  /** Rendered when the document cannot be loaded or parsed. */
  fallback?: React.ReactNode;
}

const PAGE_GAP = 16;
const PAGE_PADDING = 24;
/** Pages stop growing past a comfortable reading width on wide panes. */
const MAX_PAGE_WIDTH = 960;

function PdfPage({
  pdf,
  pageNumber,
  width,
  aspect,
}: {
  pdf: PDFDocumentProxy;
  pageNumber: number;
  width: number;
  aspect: number;
}) {
  const { frameRef, canvasRef, near, failed, text, height } = usePdfPage(
    pdf,
    pageNumber,
    width,
    aspect
  );
  // The drawn canvas is decorative; the page's extracted text is what assistive technology reads.
  return (
    <section
      ref={frameRef}
      data-slot="file-pdf-preview-page"
      data-failed={failed || undefined}
      aria-label={`Page ${pageNumber}`}
      className="relative shrink-0 bg-white shadow-sm"
      style={{ width, height }}
    >
      {near && !failed ? <canvas ref={canvasRef} aria-hidden className="block size-full" /> : null}
      {failed ? (
        <p className="flex size-full items-center justify-center text-xs text-muted-foreground">
          {`Page ${pageNumber} couldn't be drawn`}
        </p>
      ) : null}
      {text ? <p className="sr-only whitespace-pre-line">{text}</p> : null}
    </section>
  );
}

/** A PDF rendered with pdf.js: every page stacked at the pane's width, drawn as it nears view. */
function FilePdfPreview({
  url,
  workerFactory,
  title,
  fallback = null,
  className,
  ...props
}: FilePdfPreviewProps) {
  const load = usePdfDocument(url, workerFactory);
  const [scrollRef, measured] = useElementWidth();
  const available = measured - PAGE_PADDING * 2;
  const width = Math.min(MAX_PAGE_WIDTH, Math.max(0, available));
  if (load.status === "error") return fallback;
  const pageNumbers =
    load.status === "ready" ? Array.from({ length: load.pdf.numPages }, (_, i) => i + 1) : [];
  return (
    <div
      ref={scrollRef}
      role="document"
      aria-label={title}
      aria-busy={load.status === "loading"}
      data-slot="file-pdf-preview"
      data-page-count={load.status === "ready" ? load.pdf.numPages : undefined}
      className={cn("flex min-h-0 flex-1 flex-col items-center overflow-auto bg-sunken", className)}
      style={{ gap: PAGE_GAP, padding: PAGE_PADDING }}
      {...props}
    >
      {load.status === "ready" && width > 0
        ? pageNumbers.map(pageNumber => (
            <PdfPage
              key={pageNumber}
              pdf={load.pdf}
              pageNumber={pageNumber}
              width={width}
              aspect={load.aspect}
            />
          ))
        : null}
    </div>
  );
}

export { FilePdfPreview };
