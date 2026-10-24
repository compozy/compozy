import { AlertCircle, FolderLock, Search, Workflow } from "lucide-react";
import type { ComponentProps } from "react";

import { Button, Empty, PAGE_CONTENT_GUTTER, SkeletonRegion, cn } from "@compozy/ui";

import { AutomationDetailHeadSkeleton } from "./automation-detail-head";
import { AutomationDetailSectionSkeleton } from "./automation-detail-section";
import { AutomationRailSkeleton } from "./automation-rail";
import { AutomationRuleCardSkeleton } from "./automation-rule-card";
import { AutomationRunListSkeleton } from "./automation-run-list";

interface AutomationDetailSkeletonProps extends Omit<ComponentProps<"section">, "children"> {
  /** Which entity the route names, so the rule card and rail hold the bands that kind has. */
  kind?: "job" | "trigger";
}

/**
 * Loading keeps the shape of the answer, not a spinner: the real page grid with the head's
 * sentence lines, "How it works", the runs and the rail held by placeholders. Never a guessed
 * sentence.
 */
export function AutomationDetailSkeleton({
  kind = "job",
  className,
  ...props
}: AutomationDetailSkeletonProps) {
  return (
    <section
      className={cn(PAGE_CONTENT_GUTTER, "flex min-h-0 flex-1 flex-col overflow-hidden", className)}
      data-testid="automation-detail-loading"
      {...props}
    >
      {/* Same scroller and container query as the loaded page, so the columns don't jump. */}
      <SkeletonRegion
        className="@container min-h-0 flex-1 overflow-y-auto pt-5 pb-16"
        label="Loading automation details"
      >
        <AutomationDetailHeadSkeleton sentenceLines={kind === "job" ? 1 : 2} />
        <div className="grid items-start gap-8 @3xl:grid-cols-[minmax(0,1fr)_var(--width-detail-inspector-inline)]">
          <main className="flex min-w-0 flex-col gap-6">
            <AutomationDetailSectionSkeleton icon={Workflow} label="How it works">
              <AutomationRuleCardSkeleton kind={kind} />
            </AutomationDetailSectionSkeleton>
            <AutomationRunListSkeleton />
          </main>
          <AutomationRailSkeleton kind={kind} />
        </div>
      </SkeletonRegion>
    </section>
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
