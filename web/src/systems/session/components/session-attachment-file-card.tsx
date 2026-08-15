import type { ComponentProps } from "react";

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
        "att-file flex min-h-9 max-w-[240px] shrink-0 items-center gap-2 rounded-md border border-line bg-elevated pr-[10px] text-left text-fg",
        "transition-colors duration-base ease-out hover:border-line-strong hover:bg-row-hover",
        "focus-visible:shadow-focus-ring focus-visible:outline-none",
        className
      )}
      {...props}
    >
      <span
        aria-hidden="true"
        className="grid size-9 shrink-0 place-items-center rounded-l-md border-r border-line bg-canvas-soft"
      >
        <span
          className="font-mono text-[9px] font-semibold leading-none text-subtle"
          style={{ letterSpacing: "0.06em" }}
        >
          {extension}
        </span>
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-px py-1">
        <span className="truncate text-[12px] font-medium text-fg">{filename}</span>
        {sizeLabel ? (
          <span className="font-mono text-micro text-faint tabular-nums">{sizeLabel}</span>
        ) : null}
      </span>
    </a>
  );
}
