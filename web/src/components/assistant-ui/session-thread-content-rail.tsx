import type { ComponentProps, ReactNode } from "react";

import { cn } from "@/lib/utils";

export type SessionThreadContentInset = "px-4" | "px-8";

/**
 * Shared centered column applied to both the transcript viewport and the
 * composer so the two surfaces share one measure (`max-w-transcript`) and the
 * same horizontal padding.
 */
export function ThreadContentRail({
  inset,
  className,
  children,
  ...props
}: {
  inset: SessionThreadContentInset;
  className?: string;
  children: ReactNode;
} & Omit<ComponentProps<"div">, "className" | "children">) {
  return (
    <div
      className={cn("mx-auto w-full min-w-0 max-w-transcript", inset, className)}
      data-testid="thread-content-rail"
      {...props}
    >
      {children}
    </div>
  );
}
