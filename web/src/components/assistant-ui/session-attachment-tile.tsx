import { X } from "lucide-react";
import { useEffect, useRef, useState, type ComponentProps } from "react";

import { cn } from "@/lib/utils";
import {
  attachmentExtensionMark,
  isImageAttachmentMime,
} from "@/systems/session/lib/attachment-kinds";
import { Button, Eyebrow, Spinner } from "@compozy/ui";

import type { SessionAttachmentTileModel } from "./session-attachment-tile-model";

export type {
  SessionAttachmentTileModel,
  SessionAttachmentTileState,
} from "./session-attachment-tile-model";

export interface SessionAttachmentTileProps extends ComponentProps<"li"> {
  model: SessionAttachmentTileModel;
  onRemove: () => void;
  onRetry?: () => void;
}

export function SessionAttachmentTile({
  model,
  onRemove,
  onRetry,
  className,
  ...props
}: SessionAttachmentTileProps) {
  const isImage = isImageAttachmentMime(model.mimeType) && Boolean(model.file);
  const [dimensions, setDimensions] = useState<{ width: number; height: number } | null>(
    model.width && model.height ? { width: model.width, height: model.height } : null
  );
  const previewRef = useRef<HTMLImageElement>(null);

  useEffect(() => {
    const node = previewRef.current;
    if (!node || !model.file || typeof URL.createObjectURL !== "function") return;
    // oxlint-disable-next-line react-doctor/no-create-object-url-without-revoke -- the effect cleanup revokes this exact URL; Strict Mode coverage proves every allocation is released.
    const url = URL.createObjectURL(model.file);
    node.src = url;
    return () => URL.revokeObjectURL(url);
  }, [model.file]);

  const canPreview = isImage && typeof URL.createObjectURL === "function";

  const nameTitle =
    isImage && dimensions ? `${model.name} · ${dimensions.width}×${dimensions.height}` : model.name;
  const showRetry = model.retryable && Boolean(onRetry);

  return (
    <li
      {...props}
      data-attachment-rail-item
      data-state={model.state}
      data-testid="composer-attachment-tile"
      className={cn(
        "flex max-w-60 min-h-9 shrink-0 items-center gap-2 rounded-md",
        "transition-colors duration-base ease-out",
        "hover:bg-row-hover focus-within:bg-row-hover",
        className
      )}
    >
      <span
        aria-hidden="true"
        className={cn(
          "relative size-9 shrink-0 overflow-hidden rounded-md border border-line bg-canvas-soft",
          isImage ? null : "grid place-items-center"
        )}
      >
        {canPreview ? (
          <img
            ref={previewRef}
            alt=""
            onLoad={event => {
              setDimensions({
                width: event.currentTarget.naturalWidth,
                height: event.currentTarget.naturalHeight,
              });
            }}
            className={cn(
              "size-full object-cover",
              model.state === "uploading" ? "opacity-45" : null
            )}
          />
        ) : (
          <Eyebrow
            className={cn(
              "leading-none",
              model.state === "rejected" ? "text-danger" : "text-subtle",
              model.state === "uploading" ? "opacity-45" : null
            )}
          >
            {attachmentExtensionMark(model.name, model.mimeType)}
          </Eyebrow>
        )}
        {model.state === "uploading" ? (
          <span className="absolute inset-0 grid place-items-center text-fg">
            <Spinner className="size-3" />
          </span>
        ) : null}
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-px">
        <span className="truncate text-small-body font-medium text-fg" title={nameTitle}>
          {model.name}
        </span>
        <span
          className={cn(
            "font-mono text-micro tabular-nums",
            model.state === "error" || model.state === "rejected" ? "text-danger" : "text-faint",
            model.state === "uploading" ? "text-subtle" : null
          )}
        >
          {model.sizeLabel}
        </span>
      </span>
      {showRetry ? (
        <Button type="button" variant="ghost" size="sm" onClick={onRetry} className="text-micro">
          Retry upload
        </Button>
      ) : null}
      <Button
        type="button"
        variant="ghost"
        size="icon-xs"
        aria-label={`Remove ${model.name}`}
        data-testid="composer-attachment-remove"
        onClick={onRemove}
        className="text-faint hover:bg-danger-tint hover:text-danger"
      >
        <X className="size-3" />
      </Button>
    </li>
  );
}
