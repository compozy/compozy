import type { ComponentProps } from "react";
import { Workflow } from "@compozy/ui/icons";

import { cn, PAGE_CONTENT_GUTTER, SkeletonRegion } from "@compozy/ui";

import { AutomationDetailHeadSkeleton } from "./automation-detail-head-skeleton";
import { AutomationDetailSectionSkeleton } from "./automation-detail-section";
import { AutomationRailSkeleton } from "./automation-rail-skeleton";
import { AutomationRuleCardSkeleton } from "./automation-rule-card-skeleton";
import { AutomationRunListSkeleton } from "./automation-run-list-skeleton";

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
