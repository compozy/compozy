import type { ComponentProps } from "react";

import { cn } from "@/lib/utils";

export interface SessionAttachmentFrameProps extends ComponentProps<"a"> {
  filename: string;
  src: string;
}

export function SessionAttachmentFrame({
  filename,
  src,
  className,
  title,
  ...props
}: SessionAttachmentFrameProps) {
  return (
    <a
      data-testid="user-message-attachment-frame"
      title={title ?? filename}
      className={cn(
        "att-frame group/att-frame relative block max-w-[280px] shrink-0 overflow-hidden rounded-lg border border-line bg-elevated",
        "transition-colors duration-base ease-out hover:border-line-strong",
        "focus-visible:shadow-focus-ring focus-visible:outline-none",
        className
      )}
      {...props}
    >
      <img
        src={src}
        alt={filename}
        className="block h-auto max-h-[168px] w-auto max-w-[280px] object-cover"
      />
      <span
        className={cn(
          "pointer-events-none absolute bottom-2 left-2 max-w-[calc(100%-16px)] truncate rounded-xs px-[7px] py-[3px]",
          "bg-[color-mix(in_oklch,var(--canvas)_88%,transparent)] font-mono text-[10px] text-muted",
          "opacity-0 transition-opacity duration-base ease-out",
          "group-hover/att-frame:opacity-100 group-focus-visible/att-frame:opacity-100"
        )}
      >
        {filename}
      </span>
    </a>
  );
}
