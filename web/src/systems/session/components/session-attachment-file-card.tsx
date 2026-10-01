import type { ComponentProps } from "react";
import { Eyebrow } from "@compozy/ui";

import { cn } from "@/lib/utils";

export interface SessionAttachmentFileCardProps extends ComponentProps<"a"> {
  filename: string;
  extension: string;
  sizeLabel?: string;
}

export function SessionAttachmentFileCard({
  filename,
  extension,
  sizeLabel,
  className,
  title,
  ...props
}: SessionAttachmentFileCardProps) {
  return (
    <a
      data-testid="user-message-attachment-file-card"
      title={title ?? filename}
      className={cn(
        "att-file flex min-h-9 max-w-60 shrink-0 items-center gap-2",
        "rounded-lg bg-surface-2",
        "pr-2.5 text-left text-fg",
        "transition-[background-color,box-shadow] duration-base ease-out",
        "hover:bg-selected hover:shadow-card",
        "focus-visible:shadow-focus-ring focus-visible:outline-none",
        className
      )}
      {...props}
    >
      <span
        aria-hidden="true"
        className={cn("grid size-9 shrink-0 place-items-center rounded-l-lg bg-sunken")}
      >
        <Eyebrow className="leading-none text-subtle">{extension}</Eyebrow>
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-px py-1">
        <span className="truncate text-small-body font-medium text-fg">{filename}</span>
        {sizeLabel ? (
          <span className="font-mono text-micro text-faint tabular-nums">{sizeLabel}</span>
        ) : null}
      </span>
    </a>
  );
}
