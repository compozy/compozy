import { AlertCircle, FolderLock, Search } from "lucide-react";
import type { ComponentProps } from "react";

import { Button, Empty, PAGE_CONTENT_GUTTER, Skeleton, cn } from "@compozy/ui";

/**
 * Loading keeps the shape of the answer, not a spinner: one sentence-width bar
 * where the sentence will be, one subhead bar, then the card geometry. Never a
 * guessed sentence.
 */
export function AutomationDetailSkeleton({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      aria-busy="true"
      className={cn(PAGE_CONTENT_GUTTER, "flex min-h-0 flex-1 flex-col overflow-hidden", className)}
      data-testid="automation-detail-loading"
      role="status"
      {...props}
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
        <Skeleton className="h-64 w-full" />
      </div>
      <span className="sr-only">Loading automation details</span>
    </div>
  );
}

/** Why the daemon will not hand this automation over; each reason has its own next step. */
export type AutomationDetailUnavailableReason = "missing" | "elsewhere" | "error";

const UNAVAILABLE_COPY = {
  missing: {
    icon: Search,
    title: "Automation unavailable",
    testId: "automation-detail-empty",
  },
  elsewhere: {
    icon: FolderLock,
    title: "Unable to load details",
    testId: "automation-detail-elsewhere",
  },
  error: {
    icon: AlertCircle,
    title: "Unable to load details",
    testId: "automation-detail-error",
  },
} as const satisfies Record<
  AutomationDetailUnavailableReason,
  { icon: typeof Search; title: string; testId: string }
>;

interface AutomationDetailUnavailableProps extends Omit<ComponentProps<"div">, "children"> {
  description: string;
  reason: AutomationDetailUnavailableReason;
  onBack: () => void;
}

/** A missing, other-project or unreadable automation — always with a way back. */
export function AutomationDetailUnavailable({
  className,
  description,
  reason,
  onBack,
  ...props
}: AutomationDetailUnavailableProps) {
  const copy = UNAVAILABLE_COPY[reason];
  return (
    <div
      className={cn(
        PAGE_CONTENT_GUTTER,
        "flex min-h-0 flex-1 items-center justify-center py-10",
        className
      )}
      data-testid={copy.testId}
      {...props}
    >
      <Empty
        action={
          <Button onClick={onBack} size="sm" type="button" variant="neutral">
            Back to Automations
          </Button>
        }
        className="max-w-md"
        description={description}
        framed
        icon={copy.icon}
        title={copy.title}
      />
    </div>
  );
}
