import { AlertCircle, Search } from "lucide-react";
import type { ReactNode } from "react";

import { Empty, PAGE_CONTENT_GUTTER, Skeleton, cn } from "@compozy/ui";

/**
 * Loading keeps the shape of the answer, not a spinner: one sentence-width bar
 * where the rule will be, one meta bar under it. Never a guessed sentence.
 */
export function TriggerDetailSkeleton() {
  return (
    <div
      aria-busy="true"
      className={cn(PAGE_CONTENT_GUTTER, "flex min-h-0 flex-1 flex-col overflow-hidden")}
      data-testid="automation-detail-loading"
      role="status"
    >
      <div className="flex flex-col gap-3 border-b border-line pt-5 pb-4.5">
        <Skeleton className="h-5.5 w-[min(32.5rem,90%)]" />
        <Skeleton className="h-3 w-70" />
      </div>
      <div className="grid items-start gap-8 pt-5.5 lg:grid-cols-[minmax(0,1fr)_var(--width-detail-inspector-inline)]">
        <div className="flex flex-col gap-6">
          <Skeleton className="h-40 w-full" />
          <Skeleton className="h-32 w-full" />
        </div>
        <div className="flex flex-col gap-3">
          <Skeleton className="h-36 w-full" />
          <Skeleton className="h-11 w-full" />
        </div>
      </div>
      <span className="sr-only">Loading trigger details</span>
    </div>
  );
}

/**
 * A trigger the daemon will not hand over — unreachable manager, another
 * workspace's record, or a definition that no longer exists. The reason is the
 * message, because each one has a different next step.
 */
export function TriggerDetailUnavailable({
  action,
  description,
  reason,
}: {
  action?: ReactNode;
  description: string;
  reason: "error" | "gone";
}) {
  return (
    <div
      className={cn(PAGE_CONTENT_GUTTER, "flex min-h-0 flex-1 items-center justify-center py-10")}
      data-testid={reason === "gone" ? "automation-detail-empty" : "automation-detail-error"}
    >
      <Empty
        action={action}
        className="max-w-md"
        description={description}
        framed
        icon={reason === "gone" ? Search : AlertCircle}
        title={reason === "gone" ? "Trigger unavailable" : "Unable to load details"}
      />
    </div>
  );
}
