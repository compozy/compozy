"use client";

import type * as React from "react";

import { cn } from "../../lib/utils";
import { imageDataUrl } from "./image-data-url";

export interface FileImagePreviewProps extends Omit<React.ComponentProps<"figure">, "children"> {
  /** Base64 image bytes. */
  data: string;
  /** png, jpeg, gif, webp, or svg+xml; anything else renders `fallback`. */
  mimeType: string;
  alt: string;
  /** Rendered when the type is not previewable or the payload is invalid. */
  fallback?: React.ReactNode;
}

/** Centered image preview that never inlines SVG markup. */
function FileImagePreview({
  data,
  mimeType,
  alt,
  fallback = null,
  className,
  ...props
}: FileImagePreviewProps) {
  const src = imageDataUrl(mimeType, data);
  if (!src) return fallback;
  return (
    <figure
      data-slot="file-image-preview"
      className={cn(
        "flex min-h-0 flex-1 items-center justify-center overflow-auto bg-canvas p-6",
        className
      )}
      {...props}
    >
      <img src={src} alt={alt} className="max-h-full max-w-full object-contain" />
    </figure>
  );
}

export { FileImagePreview };
