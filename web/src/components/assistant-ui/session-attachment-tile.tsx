import { X } from "lucide-react";
import { useEffect, useState } from "react";

import { cn } from "@/lib/utils";
import {
  attachmentExtensionMark,
  isImageAttachmentMime,
} from "@/systems/session/lib/attachment-kinds";
import { Button, Spinner } from "@compozy/ui";

import type { SessionAttachmentTileModel } from "./session-attachment-tile-model";

export type {
  SessionAttachmentTileModel,
  SessionAttachmentTileState,
} from "./session-attachment-tile-model";

function isPersistFailureLabel(label: string): boolean {
  return label === "Couldn't save" || label.startsWith("Couldn't save");
}

export function SessionAttachmentTile({
  model,
  onRemove,
  onRetry,
}: {
  model: SessionAttachmentTileModel;
  onRemove: () => void;
  onRetry?: () => void;
}) {
  const isImage = isImageAttachmentMime(model.mimeType) && Boolean(model.file);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [dimensions, setDimensions] = useState<{ width: number; height: number } | null>(
    model.width && model.height ? { width: model.width, height: model.height } : null
  );

  useEffect(() => {
    if (!isImage || !model.file) {
      setPreviewUrl(null);
      return;
    }
    const url = URL.createObjectURL(model.file);
    setPreviewUrl(url);
    const image = new Image();
    const handleLoad = () => {
      setDimensions({ width: image.naturalWidth, height: image.naturalHeight });
    };
    image.addEventListener("load", handleLoad);
    image.src = url;
    return () => {
      image.removeEventListener("load", handleLoad);
      URL.revokeObjectURL(url);
    };
  }, [isImage, model.file]);

  const nameTitle =
    isImage && dimensions ? `${model.name} · ${dimensions.width}×${dimensions.height}` : model.name;
  const showRetry = model.retryable && isPersistFailureLabel(model.sizeLabel) && onRetry;

  return (
    <li
      data-state={model.state}
      data-testid="composer-attachment-tile"
      className={cn(
        "flex max-w-60 min-h-9 shrink-0 items-center gap-2 rounded-md",
        "transition-colors duration-base ease-out",
        "hover:bg-row-hover focus-within:bg-row-hover"
      )}
    >
      <span
        aria-hidden="true"
        className={cn(
          "relative size-9 shrink-0 overflow-hidden rounded-md border border-line bg-canvas-soft",
          isImage ? null : "grid place-items-center"
        )}
      >
        {isImage && previewUrl ? (
          <img
            src={previewUrl}
            alt=""
            className={cn(
              "size-full object-cover",
              model.state === "uploading" ? "opacity-45" : null
            )}
          />
        ) : (
          <span
            className={cn(
              "font-mono text-[9px] font-semibold leading-none",
              model.state === "rejected" ? "text-danger" : "text-subtle",
              model.state === "uploading" ? "opacity-45" : null
            )}
            style={{ letterSpacing: "0.06em" }}
          >
            {attachmentExtensionMark(model.name, model.mimeType)}
          </span>
        )}
        {model.state === "uploading" ? (
          <span className="absolute inset-0 grid place-items-center text-fg">
            <Spinner className="size-3" />
          </span>
        ) : null}
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-px">
        <span className="truncate text-[12px] font-medium text-fg" title={nameTitle}>
          {model.name}
        </span>
        <span
          className={cn(
            "font-mono text-[10.5px] tabular-nums",
            model.state === "error" || model.state === "rejected" ? "text-danger" : "text-faint",
            model.state === "uploading" ? "text-subtle" : null
          )}
        >
          {model.sizeLabel}
        </span>
      </span>
      {showRetry ? (
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={onRetry}
          className="h-[22px] px-[7px] text-[11px]"
        >
          Retry
        </Button>
      ) : null}
      <button
        type="button"
        aria-label={`Remove ${model.name}`}
        data-testid="composer-attachment-remove"
        onClick={onRemove}
        className={cn(
          "grid size-[22px] shrink-0 place-items-center rounded-sm text-faint",
          "transition-colors duration-fast ease-out",
          "hover:bg-danger-tint hover:text-danger",
          "focus-visible:shadow-focus-ring focus-visible:outline-none"
        )}
      >
        <X className="size-[11.5px]" />
      </button>
    </li>
  );
}
