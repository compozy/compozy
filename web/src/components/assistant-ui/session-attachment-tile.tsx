import { X } from "lucide-react";
import { useEffect, useState, type ComponentProps } from "react";

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

function isPersistFailureLabel(label: string): boolean {
  return label === "Couldn't save" || label.startsWith("Couldn't save");
}

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
  const [preview, setPreview] = useState<{ file: File; url: string } | null>(null);
  const [dimensions, setDimensions] = useState<{ width: number; height: number } | null>(
    model.width && model.height ? { width: model.width, height: model.height } : null
  );

  useEffect(() => {
    if (!isImage || !model.file) return;
    const file = model.file;
    const reader = new FileReader();
    const handleLoad = () => {
      if (typeof reader.result === "string") {
        setPreview({ file, url: reader.result });
      }
    };
    reader.addEventListener("load", handleLoad);
    reader.readAsDataURL(file);
    return () => {
      reader.removeEventListener("load", handleLoad);
      if (reader.readyState === FileReader.LOADING) {
        reader.abort();
      }
    };
  }, [isImage, model.file]);

  const previewUrl = preview && preview.file === model.file ? preview.url : null;

  const nameTitle =
    isImage && dimensions ? `${model.name} · ${dimensions.width}×${dimensions.height}` : model.name;
  const showRetry = model.retryable && isPersistFailureLabel(model.sizeLabel) && onRetry;

  return (
    <li
      {...props}
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
        {isImage && previewUrl ? (
          <img
            src={previewUrl}
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
