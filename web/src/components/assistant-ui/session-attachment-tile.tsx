import { X } from "lucide-react";
import { useEffect, useRef, useState, type ComponentProps, type RefObject } from "react";

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

interface AttachmentDimensions {
  width: number;
  height: number;
}

function initialDimensions(model: SessionAttachmentTileModel): AttachmentDimensions | null {
  return model.width && model.height ? { width: model.width, height: model.height } : null;
}

function attachmentNameTitle(
  model: SessionAttachmentTileModel,
  isImage: boolean,
  dimensions: AttachmentDimensions | null
): string {
  return isImage && dimensions
    ? `${model.name} · ${dimensions.width}×${dimensions.height}`
    : model.name;
}

function attachmentSizeTone(state: SessionAttachmentTileModel["state"]): string {
  return state === "error" || state === "rejected" ? "text-danger" : "text-faint";
}

interface AttachmentThumbProps {
  model: SessionAttachmentTileModel;
  isImage: boolean;
  canPreview: boolean;
  previewRef: RefObject<HTMLImageElement | null>;
  onPreviewLoad: (dimensions: AttachmentDimensions) => void;
}

/** The square thumbnail: an image preview or the extension mark, dimmed while uploading. */
function AttachmentThumb({
  model,
  isImage,
  canPreview,
  previewRef,
  onPreviewLoad,
}: AttachmentThumbProps) {
  const uploading = model.state === "uploading";
  return (
    <span
      aria-hidden="true"
      className={cn(
        "relative size-9 shrink-0 overflow-hidden rounded-md bg-surface-2",
        isImage ? null : "grid place-items-center"
      )}
    >
      {canPreview ? (
        <img
          ref={previewRef}
          alt=""
          onLoad={event => {
            onPreviewLoad({
              width: event.currentTarget.naturalWidth,
              height: event.currentTarget.naturalHeight,
            });
          }}
          className={cn("size-full object-cover", uploading ? "opacity-45" : null)}
        />
      ) : (
        <Eyebrow
          className={cn(
            "leading-none",
            model.state === "rejected" ? "text-danger" : "text-subtle",
            uploading ? "opacity-45" : null
          )}
        >
          {attachmentExtensionMark(model.name, model.mimeType)}
        </Eyebrow>
      )}
      {uploading ? (
        <span className="absolute inset-0 grid place-items-center text-fg">
          <Spinner className="size-3" />
        </span>
      ) : null}
    </span>
  );
}

export function SessionAttachmentTile({
  model,
  onRemove,
  onRetry,
  className,
  ...props
}: SessionAttachmentTileProps) {
  const isImage = isImageAttachmentMime(model.mimeType) && Boolean(model.file);
  const [dimensions, setDimensions] = useState<AttachmentDimensions | null>(() =>
    initialDimensions(model)
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
        "hover:bg-surface-2 focus-within:bg-surface-2",
        className
      )}
    >
      <AttachmentThumb
        canPreview={canPreview}
        isImage={isImage}
        model={model}
        onPreviewLoad={setDimensions}
        previewRef={previewRef}
      />
      <span className="flex min-w-0 flex-1 flex-col gap-px">
        <span
          className="truncate text-small-body font-medium text-fg"
          title={attachmentNameTitle(model, isImage, dimensions)}
        >
          {model.name}
        </span>
        <span
          className={cn(
            "font-mono text-eyebrow tabular-nums",
            attachmentSizeTone(model.state),
            model.state === "uploading" ? "text-subtle" : null
          )}
        >
          {model.sizeLabel}
        </span>
      </span>
      {showRetry ? (
        <Button type="button" variant="ghost" size="sm" onClick={onRetry}>
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
