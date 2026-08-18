import type { ComponentProps } from "react";
import type { LucideIcon } from "lucide-react";

import { cn } from "@compozy/ui";

interface LoopRunQuietNoteProps extends ComponentProps<"div"> {
  icon: LucideIcon;
  title?: string;
}

export function LoopRunQuietNote({
  icon: Icon,
  title,
  className,
  children,
  ...props
}: LoopRunQuietNoteProps) {
  return (
    <div
      className={cn(
        "flex items-start gap-2.25 rounded-md border border-line-soft bg-canvas-soft px-3.5 py-3 text-small-body leading-relaxed text-muted",
        className
      )}
      {...props}
    >
      <Icon aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-subtle" />
      <div className="min-w-0">
        {title ? <div className="text-ws-name font-medium text-fg-strong">{title}</div> : null}
        <span className={title ? "mt-1.5 block" : undefined}>{children}</span>
      </div>
    </div>
  );
}
