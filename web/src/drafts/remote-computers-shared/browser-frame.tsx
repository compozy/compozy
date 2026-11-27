import type { ReactNode } from "react";
import { ChevronLeft, ChevronRight, Lock } from "@compozy/ui/icons";

/**
 * Draft-only browser window around a page the person opens in their own browser
 * (`cloud.compozy.com`, a computer's hosted address), so each state reads in context.
 * At phone widths only the address pill shows, as a mobile browser's URL bar does.
 */
export function DraftBrowserFrame({
  address,
  children,
  dataSlot = "draft-browser-window",
}: {
  address: string;
  children: ReactNode;
  dataSlot?: string;
}) {
  return (
    <div
      className="flex h-dvh min-h-dvh w-full flex-col overflow-hidden bg-canvas text-fg"
      data-slot={dataSlot}
    >
      <div
        aria-label="Browser address bar"
        className="flex h-12 shrink-0 items-center justify-center border-b border-line bg-rail px-3 select-none sm:grid sm:grid-cols-[1fr_minmax(0,440px)_1fr] sm:gap-3 sm:px-4"
      >
        <div aria-hidden="true" className="hidden items-center gap-3 sm:flex">
          <div className="flex items-center gap-2">
            <span className="size-3 rounded-full bg-line-strong" />
            <span className="size-3 rounded-full bg-line-strong" />
            <span className="size-3 rounded-full bg-line-strong" />
          </div>
          <div className="flex items-center gap-1 text-faint">
            <ChevronLeft className="size-4" />
            <ChevronRight className="size-4 opacity-50" />
          </div>
        </div>

        <div className="flex h-7.5 w-full max-w-[440px] min-w-0 items-center justify-center gap-1.5 rounded-pill border border-line-soft bg-canvas px-3.5 text-eyebrow text-muted-foreground">
          <Lock aria-hidden="true" className="size-3 shrink-0 text-subtle" />
          <span className="truncate font-mono text-mono-id">{address}</span>
        </div>

        <div aria-hidden="true" className="hidden sm:block" />
      </div>

      <div className="relative flex min-h-0 flex-1 flex-col overflow-y-auto">{children}</div>
    </div>
  );
}
