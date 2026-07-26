import type { ReactNode } from "react";

import { cn } from "../../lib/utils";

export type RightRailMode = "thread" | "inspector";

export interface RightRailProps {
  open: boolean;
  mode: RightRailMode;
  children?: ReactNode;
  className?: string;
}

export function RightRail({ open, mode, children, className }: RightRailProps) {
  if (!open) {
    return null;
  }

  return (
    <aside
      aria-label={mode === "thread" ? "Thread overlay" : "Channel inspector"}
      className={cn(
        "flex min-h-0 h-full w-full flex-col border-l border-line bg-canvas-soft",
        className
      )}
      data-mode={mode}
      data-testid="network-right-rail"
    >
      {children}
    </aside>
  );
}
