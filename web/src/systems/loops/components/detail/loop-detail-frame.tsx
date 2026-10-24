import type { ComponentProps, ReactNode } from "react";

import { cn, PAGE_CONTENT_GUTTER } from "@compozy/ui";

interface LoopDetailFrameProps extends Omit<ComponentProps<"div">, "children"> {
  lede: ReactNode;
  main: ReactNode;
  rail: ReactNode;
  /** Siblings of the page body that render nothing in it (the delete dialog). */
  children?: ReactNode;
}

/**
 * The Loop detail page's layout: lede over a main column and a rail. The loaded view and its
 * skeleton both render through it, so the column split, gaps and gutters cannot drift apart.
 */
export function LoopDetailFrame({
  lede,
  main,
  rail,
  children,
  className,
  ...props
}: LoopDetailFrameProps) {
  return (
    <div className={cn("flex min-h-0 flex-1 flex-col overflow-y-auto", className)} {...props}>
      {children}
      <div className={cn(PAGE_CONTENT_GUTTER, "flex flex-col")}>
        {lede}
        <div className="py-6">
          <div className="grid grid-cols-1 gap-8 lg:grid-cols-[minmax(0,1fr)_var(--width-detail-inspector-inline)]">
            <div className="flex flex-col gap-7">{main}</div>
            <aside className="flex flex-col gap-6">{rail}</aside>
          </div>
        </div>
      </div>
    </div>
  );
}
