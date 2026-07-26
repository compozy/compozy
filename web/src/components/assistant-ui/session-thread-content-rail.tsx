import type { ComponentPropsWithoutRef, ReactNode } from "react";

import { cn } from "@/lib/utils";

export type SessionThreadContentInset = "px-4" | "px-8";

export const SESSION_THREAD_CONTENT_INSET_DEFAULT: SessionThreadContentInset = "px-4";

/**
 * Shared full-bleed inset rail applied to both the transcript viewport and the
 * composer so the two surfaces share the same horizontal padding.
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
} & Omit<ComponentPropsWithoutRef<"div">, "className" | "children">) {
  return (
    <div
      className={cn("w-full min-w-0", inset, className)}
      data-testid="thread-content-rail"
      {...props}
    >
      {children}
    </div>
  );
}
