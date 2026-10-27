import type { ComponentProps, ReactNode } from "react";

import { cn, PAGE_CONTENT_GUTTER } from "@compozy/ui";

interface LoopRunPageFrameProps extends Omit<ComponentProps<"div">, "children"> {
  main: ReactNode;
  rail: ReactNode;
}

/**
 * The run page's layout: a main column and the rail beside it. The loaded body and its skeleton
 * both render through it, so the column split, gaps and gutters cannot drift apart.
 */
export function LoopRunPageFrame({ main, rail, className, ...props }: LoopRunPageFrameProps) {
  return (
    <div className={cn("flex min-h-0 flex-1 flex-col overflow-y-auto", className)} {...props}>
      <div className={cn(PAGE_CONTENT_GUTTER, "pt-6 pb-16")}>
        <div className="grid grid-cols-1 items-start gap-8 lg:grid-cols-[minmax(0,1fr)_var(--width-detail-inspector-inline)]">
          <main className="flex min-w-0 flex-col gap-6">{main}</main>
          <aside data-testid="loop-run-detail-rail">{rail}</aside>
        </div>
      </div>
    </div>
  );
}
