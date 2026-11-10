// The one product-shot frame: 1 px line-strong border, 12 px radius, 36 px title bar, three dots.
import { cn } from "@compozy/ui/utils";
import type { ReactNode } from "react";

export function Window({
  title = "",
  className,
  children,
}: {
  title?: string;
  className?: string;
  children?: ReactNode;
}) {
  return (
    <div
      className={cn(
        "min-w-0 overflow-hidden rounded-[12px] border border-line-strong bg-sunken text-fg",
        className
      )}
    >
      <div className="flex h-9 items-center gap-2 border-b border-line px-3.5">
        <span className="size-2.5 rounded-full bg-line" />
        <span className="size-2.5 rounded-full bg-line" />
        <span className="size-2.5 rounded-full bg-line" />
        {title && (
          <span className="ml-3 truncate font-mono text-[11.5px] text-muted-foreground">
            {title}
          </span>
        )}
      </div>
      {children}
    </div>
  );
}
