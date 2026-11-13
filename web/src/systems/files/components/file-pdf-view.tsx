import type { ReactNode } from "react";

import { FilePdfPreview } from "@compozy/ui";
import { createPdfWorker } from "@compozy/ui/lib/pdf-worker";

/**
 * The PDF branch, loaded on demand: pdf.js and its worker stay out of the Files bundle until a
 * PDF opens.
 */
export default function FilePdfView({
  url,
  title,
  fallback,
}: {
  url: string;
  title: string;
  fallback: ReactNode;
}) {
  return (
    <FilePdfPreview
      url={url}
      workerFactory={createPdfWorker}
      title={title}
      fallback={fallback}
      data-testid="file-viewer-pdf"
    />
  );
}
