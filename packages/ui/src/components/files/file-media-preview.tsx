"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

export interface FileMediaPreviewProps extends Omit<React.ComponentProps<"figure">, "children"> {
  kind: "video" | "audio";
  /** Same-origin URL that serves the bytes with HTTP Range support. */
  src: string;
  /** Accessible name, such as the file name. */
  title: string;
  /** Caption after the duration and dimensions, such as the file size. */
  meta?: React.ReactNode;
  /** Rendered when the browser cannot play the file (codec or container unsupported). */
  fallback?: React.ReactNode;
}

function formatDuration(seconds: number): string | null {
  if (!Number.isFinite(seconds) || seconds <= 0) return null;
  const total = Math.round(seconds);
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const rest = String(total % 60).padStart(2, "0");
  return hours > 0 ? `${hours}:${String(minutes).padStart(2, "0")}:${rest}` : `${minutes}:${rest}`;
}

interface MediaFacts {
  src: string;
  duration: string | null;
  size: string | null;
}

/**
 * A video or audio file played by the browser's native controls, streamed from `src`, captioned
 * with its duration (and dimensions for video) once the metadata loads.
 */
function FileMediaPreview({
  kind,
  src,
  title,
  meta,
  fallback = null,
  className,
  ...props
}: FileMediaPreviewProps) {
  const [facts, setFacts] = React.useState<MediaFacts | null>(null);
  const [failed, setFailed] = React.useState<string | null>(null);
  if (failed === src) return fallback;
  const loaded = facts?.src === src ? facts : null;
  const caption = [loaded?.size, loaded?.duration, meta].filter(
    part => part !== null && part !== undefined && part !== ""
  );
  const onLoadedMetadata = (event: React.SyntheticEvent<HTMLMediaElement>) => {
    const media = event.currentTarget;
    const video = media instanceof HTMLVideoElement ? media : null;
    setFacts({
      src,
      duration: formatDuration(media.duration),
      size: video && video.videoWidth > 0 ? `${video.videoWidth} × ${video.videoHeight}` : null,
    });
  };
  const shared = {
    src,
    controls: true,
    preload: "metadata" as const,
    "aria-label": title,
    onLoadedMetadata,
    onError: () => setFailed(src),
  };
  return (
    <figure
      data-slot="file-media-preview"
      data-kind={kind}
      className={cn(
        "flex min-h-0 flex-1 flex-col items-center justify-center gap-3 overflow-auto bg-canvas p-6",
        className
      )}
      {...props}
    >
      {kind === "video" ? (
        <video {...shared} playsInline className="min-h-0 max-w-full rounded-sm bg-black" />
      ) : (
        <audio {...shared} className="w-full max-w-xl" />
      )}
      {caption.length > 0 ? (
        <figcaption
          data-slot="file-media-preview-caption"
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

export { FileMediaPreview };
