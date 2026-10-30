/**
 * Image attachments of a message: the input (paste, drop or pick, with upload state) and the
 * thumbnails of saved images, each opening full size in a dialog.
 */
import { ImageIcon, RotateCcwIcon, XIcon } from "lucide-react";
import { useRef, useState } from "react";

import {
  Button,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  cn,
} from "@compozy/ui";

import { maxAttachments, type CommentAttachment } from "#gallery/shell/comments/model";
import type { Upload, Uploads } from "#gallery/shell/comments/use-attachments";

const accepted = "image/png,image/jpeg,image/webp,image/gif";

function UploadTile({
  item,
  disabled,
  uploads,
}: {
  item: Upload;
  disabled: boolean;
  uploads: Uploads;
}) {
  return (
    <li className="relative flex w-20 flex-col gap-1" data-testid="comment-upload">
      <img
        alt={item.file.name}
        className={cn(
          "aspect-square w-full rounded-sm object-cover shadow-hairline",
          !item.image && !item.error && "opacity-60"
        )}
        src={item.preview}
      />
      <span className={cn("truncate text-meta", item.error ? "text-danger" : "text-subtle")}>
        {item.error ?? (item.image ? item.file.name : `${item.progress}%`)}
      </span>
      <span className="absolute top-1 right-1 flex gap-0.5">
        {item.error ? (
          <Button
            aria-label={`Retry ${item.file.name}`}
            disabled={disabled}
            onClick={() => uploads.retry(item)}
            size="icon-xs"
            type="button"
            variant="secondary"
          >
            <RotateCcwIcon aria-hidden="true" />
          </Button>
        ) : null}
        <Button
          aria-label={`Remove ${item.file.name}`}
          disabled={disabled}
          onClick={() => uploads.remove(item.key)}
          size="icon-xs"
          type="button"
          variant="secondary"
        >
          <XIcon aria-hidden="true" />
        </Button>
      </span>
    </li>
  );
}

/** Pick, paste or drop images; `capacity` is what the message can still take. */
export function AttachmentInput({
  uploads,
  capacity = maxAttachments,
  disabled = false,
}: {
  uploads: Uploads;
  capacity?: number;
  disabled?: boolean;
}) {
  const [limit, setLimit] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const accept = (files: File[]) => {
    setLimit(uploads.items.length + files.length > capacity);
    uploads.add(files, capacity);
  };
  return (
    <div
      className="flex flex-col gap-1.5"
      onDragOver={event => {
        if (!disabled) event.preventDefault();
      }}
      onDrop={event => {
        event.preventDefault();
        if (!disabled) accept([...event.dataTransfer.files]);
      }}
    >
      <div className="flex items-center gap-2">
        <Button
          disabled={disabled || uploads.items.length >= capacity}
          onClick={() => inputRef.current?.click()}
          size="xs"
          type="button"
          variant="ghost"
        >
          <ImageIcon aria-hidden="true" />
          Attach image
        </Button>
        <input
          accept={accepted}
          aria-label="Attach image"
          className="hidden"
          disabled={disabled}
          multiple
          onChange={event => {
            accept([...(event.target.files ?? [])]);
            event.target.value = "";
          }}
          ref={inputRef}
          tabIndex={-1}
          type="file"
        />
        <span className="text-meta text-subtle">Paste or drop · up to 6 images of 5 MB</span>
      </div>
      {limit ? (
        <p className="text-meta text-danger" role="alert">
          A message takes up to 6 images.
        </p>
      ) : null}
      {uploads.items.length > 0 ? (
        <ul className="flex flex-wrap gap-2">
          {uploads.items.map(item => (
            <UploadTile disabled={disabled} item={item} key={item.key} uploads={uploads} />
          ))}
        </ul>
      ) : null}
    </div>
  );
}

/** Saved images of a message as thumbnails. */
export function AttachmentImages({ images }: { images: readonly CommentAttachment[] }) {
  if (images.length === 0) return null;
  return (
    <ul className="flex flex-wrap gap-2">
      {images.map(image => (
        <li key={image.id}>
          <Dialog>
            <DialogTrigger
              aria-label={`Open image ${image.name}`}
              className="block size-16 overflow-hidden rounded-sm shadow-hairline outline-none focus-visible:shadow-focus-ring"
            >
              <img
                alt={image.name}
                className="size-full object-cover"
                height={image.height}
                src={image.url}
                width={image.width}
              />
            </DialogTrigger>
            <DialogContent className="max-w-[90vw] sm:max-w-[90vw]">
              <DialogHeader>
                <DialogTitle>{image.name}</DialogTitle>
              </DialogHeader>
              <img
                alt={image.name}
                className="max-h-[75vh] w-full object-contain"
                src={image.url}
              />
            </DialogContent>
          </Dialog>
        </li>
      ))}
    </ul>
  );
}
