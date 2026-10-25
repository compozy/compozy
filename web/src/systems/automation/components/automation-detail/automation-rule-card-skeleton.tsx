import type { ComponentProps } from "react";

import { cn, Skeleton, SkeletonText } from "@compozy/ui";

import { RuleRow } from "./automation-rule-row";

const NEXT_RUN_SKELETON_IDS = ["next-run-1", "next-run-2", "next-run-3"] as const;

/**
 * "How it works" while the automation loads: the real card and its Starts / Does rows. A schedule
 * lists its upcoming runs under Starts; an event does not.
 */
export function AutomationRuleCardSkeleton({
  kind,
  className,
  ...props
}: Omit<ComponentProps<"div">, "children"> & { kind: "job" | "trigger" }) {
  return (
    <div
      aria-hidden="true"
      className={cn("overflow-hidden rounded-lg bg-card shadow-card", className)}
      {...props}
    >
      <RuleRow label="Starts">
        <span className="flex flex-wrap items-center gap-2">
          <Skeleton className="size-3.5" />
          <b className="text-form-input font-medium">
            <SkeletonText className="w-48" />
          </b>
        </span>
        <span className="mt-1.5 text-eyebrow leading-normal">
          <SkeletonText className="w-64" />
        </span>
        {kind === "job" ? (
          <ol className="mt-2.5 flex flex-col">
            {NEXT_RUN_SKELETON_IDS.map(id => (
              <li
                className="flex min-h-7 items-center border-t border-line-soft text-eyebrow first:border-t-0"
                key={id}
              >
                <SkeletonText className="w-full" />
              </li>
            ))}
          </ol>
        ) : null}
      </RuleRow>
      <RuleRow label="Does">
        <span className="flex flex-wrap items-center gap-2">
          <Skeleton className="size-3.5" />
          <b className="text-form-input font-medium">
            <SkeletonText className="w-40" />
          </b>
        </span>
      </RuleRow>
    </div>
  );
}
