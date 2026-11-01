import type { ComponentProps } from "react";

import { Button } from "@compozy/ui";

import { cn } from "@/lib/utils";
import {
  SESSION_ATTACHMENT_IMAGE_MAX_HEIGHT,
  SESSION_ATTACHMENT_IMAGE_MAX_WIDTH,
} from "../lib/session-attachment-geometry";

import { SessionAttachmentImagePreview } from "./session-attachment-image-preview";

export interface SessionAttachmentFrameProps extends ComponentProps<"button"> {
  filename: string;
  src: string;
  width?: number;
  height?: number;
}

/** Renders a transcript image as a preview button that preserves the session route. */
export function SessionAttachmentFrame({
  filename,
  src,
  className,
  title,
  width,
  height,
  ...props
}: SessionAttachmentFrameProps) {
  return (
    <SessionAttachmentImagePreview
      filename={filename}
      source={src}
      trigger={
        <Button
          type="button"
          variant="ghost"
          aria-label={`Preview ${filename}`}
          data-testid="user-message-attachment-frame"
          title={title ?? filename}
          className={cn(
            "att-frame group/att-frame relative block h-auto max-w-70 shrink-0 overflow-hidden p-0",
            "rounded-lg bg-surface-2",
            "transition-shadow duration-base ease-out hover:shadow-card",
            "focus-visible:shadow-focus-ring focus-visible:outline-none",
            className
          )}
          {...props}
        >
          <img
            src={src}
            alt={filename}
            {...(width && width > 0 ? { width } : {})}
            {...(height && height > 0 ? { height } : {})}
            style={{
              aspectRatio:
                width && height && width > 0 && height > 0 ? `${width} / ${height}` : "4 / 3",
              maxHeight: SESSION_ATTACHMENT_IMAGE_MAX_HEIGHT,
              maxWidth: SESSION_ATTACHMENT_IMAGE_MAX_WIDTH,
            }}
            className="block h-auto w-auto object-cover"
          />
          <span
            aria-hidden="true"
            className={cn(
              "pointer-events-none absolute bottom-2 left-2 max-w-[calc(100%-16px)] truncate",
              "rounded-xs bg-canvas/90 px-2 py-1 font-mono text-micro text-muted-foreground",
              "opacity-0 transition-opacity duration-base ease-out",
              "group-hover/att-frame:opacity-100",
              "group-focus-visible/att-frame:opacity-100"
            )}
          >
            {filename}
          </span>
        </Button>
      }
    />
  );
}
