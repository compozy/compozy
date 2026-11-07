"use client";

import * as React from "react";

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
  /** Follows the dimensions in the caption, such as the file size (`1280 × 720 · 245 KB`). */
  meta?: React.ReactNode;
}

interface ImageSize {
  width: number;
  height: number;
}

/** Centered image preview that never inlines SVG markup, captioned with its dimensions. */
function FileImagePreview({
  data,
  mimeType,
  alt,
  fallback = null,
  meta,
  className,
  ...props
}: FileImagePreviewProps) {
  const src = imageDataUrl(mimeType, data);
  // Dimensions come from the decoded image, so they are known only once it loads.
  const [size, setSize] = React.useState<{ src: string; size: ImageSize } | null>(null);
  if (!src) return fallback;
  const loaded = size?.src === src ? size.size : null;
  const caption = [loaded ? `${loaded.width} × ${loaded.height}` : null, meta].filter(
    part => part !== null && part !== undefined && part !== ""
  );
  return (
    <figure
      data-slot="file-image-preview"
      className={cn(
        "flex min-h-0 flex-1 flex-col items-center justify-center gap-3 overflow-auto bg-canvas p-6",
        className
      )}
      {...props}
    >
      <img
        src={src}
        alt={alt}
        onLoad={event => {
          const image = event.currentTarget;
          if (image.naturalWidth > 0 && image.naturalHeight > 0) {
            setSize({ src, size: { width: image.naturalWidth, height: image.naturalHeight } });
          }
        }}
        className="min-h-0 max-w-full object-contain"
      />
      {caption.length > 0 ? (
        <figcaption
          data-slot="file-image-preview-caption"
          className="shrink-0 text-xs text-muted-foreground tabular-nums"
        >
          {caption.map((part, index) => (
            <React.Fragment key={index}>
              {index > 0 ? " · " : null}
              {part}
            </React.Fragment>
          ))}
        </figcaption>
      ) : null}
    </figure>
  );
}

export { FileImagePreview };
