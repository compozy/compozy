import type { ComponentProps } from "react";
import { CalendarClock, Hash, Info, ShieldCheck } from "lucide-react";

import { cn, PropertyRowSkeleton, Skeleton, SkeletonText } from "@compozy/ui";

import { RailSection } from "./automation-rail-section";

const RAIL_SKELETON_SECTIONS = {
  job: [
    { title: "Details", icon: Info, rows: 5 },
    { title: "Schedule", icon: CalendarClock, rows: 4 },
    { title: "Reliability", icon: ShieldCheck, rows: 2 },
    { title: "Identity", icon: Hash, rows: 2 },
  ],
  trigger: [
    { title: "Details", icon: Info, rows: 5 },
    { title: "Reliability", icon: ShieldCheck, rows: 2 },
    { title: "Identity", icon: Hash, rows: 2 },
  ],
} as const;

/**
 * The facts rail while the automation loads: the one card with the bands its kind has, each with
 * its real title over property rows, and the Inspect line below.
 */
export function AutomationRailSkeleton({
  kind,
  className,
  ...props
}: Omit<ComponentProps<"aside">, "children"> & { kind: "job" | "trigger" }) {
  return (
    <aside aria-hidden="true" className={cn("flex flex-col gap-2", className)} {...props}>
      <div className="overflow-hidden rounded-lg bg-card shadow-card">
        {RAIL_SKELETON_SECTIONS[kind].map(section => (
          <RailSection icon={section.icon} key={section.title} title={section.title}>
            <PropertyRowSkeleton count={section.rows} />
          </RailSection>
        ))}
      </div>
      <div className="flex items-center justify-between gap-2 px-1 pt-0.5">
        <Skeleton className="h-button-xs w-20" />
        <span className="font-mono text-badge">
          <SkeletonText className="w-40" />
        </span>
      </div>
    </aside>
  );
}
