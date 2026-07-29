import type { ComponentProps, ReactNode } from "react";
import { AlertTriangle } from "lucide-react";

import { cn } from "@compozy/ui";

export interface SessionDangerBannerProps extends Omit<ComponentProps<"div">, "title"> {
  title: ReactNode;
}

/** Shared session-level danger notice: one quiet wash, one icon, one title, one body. */
export function SessionDangerBanner({
  title,
  children,
  className,
  ...props
}: SessionDangerBannerProps) {
  return (
    <div
      {...props}
      role="alert"
      aria-live="assertive"
      className={cn(
        "flex min-w-0 items-start gap-2 rounded-lg border border-danger/28 bg-danger-tint px-3 py-2.5 text-small-body leading-normal text-fg",
        className
      )}
    >
      <AlertTriangle aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-danger" />
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <b className="font-semibold text-fg-strong">{title}</b>
        {children}
      </div>
    </div>
  );
}
