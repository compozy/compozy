import {
  Metric,
  MetricGrid,
  Skeleton,
  SkeletonRegion,
  SkeletonText,
  StatusBreakdown,
} from "@compozy/ui";
import { SESSION_INSPECTOR_TILE } from "./session-inspector-tile";

// Content-shaped placeholders for the Context rail's sections while their reads are in flight.
// Each repeats the live section's primitives (`Metric`, `StatusBreakdown`, the turn row grid)
// with `SkeletonText` in the text slots, so a section is as tall loading as it is loaded. Every
// section is its own loading area, so each one is its own status region.

/** The meter before its first reading: the figure and amount, the track, and the tier legend. */
export function SessionContextMeterSkeleton() {
  return (
    <SkeletonRegion
      label="Loading context"
      className="flex flex-col gap-2.25"
      data-testid="session-context-meter-skeleton"
    >
      <div aria-hidden="true" className="flex flex-wrap items-baseline gap-2 tabular-nums">
        <span className="text-kpi-compact leading-none font-semibold tracking-tight">
          <SkeletonText className="w-12" />
        </span>
        <span className="font-mono text-mono-id">
          <SkeletonText className="w-24" />
        </span>
      </div>
      <Skeleton className="h-1.5 w-full rounded-pill" />
      <StatusBreakdown
        aria-hidden="true"
        total={1}
        items={[
          {
            id: "compozy",
            label: <SkeletonText className="w-24" />,
            swatch: <Skeleton className="size-2 shrink-0 rounded-[2px]" />,
            value: 0,
            formattedValue: <SkeletonText className="w-8" />,
            showBar: false,
          },
          {
            id: "agent",
            label: <SkeletonText className="w-28" />,
            swatch: <Skeleton className="size-2 shrink-0 rounded-[2px]" />,
            value: 0,
            formattedValue: <SkeletonText className="w-8" />,
            showBar: false,
          },
          {
            id: "free",
            label: <SkeletonText className="w-12" />,
            swatch: <Skeleton className="size-2 shrink-0 rounded-[2px]" />,
            value: 0,
            formattedValue: <SkeletonText className="w-8" />,
            showBar: false,
          },
        ]}
      />
    </SkeletonRegion>
  );
}

/**
 * The usage section's body before the first report: the real metric grid, a cost tile with its
 * provenance line, and the closed "Token breakdown" fold's head.
 */
export function SessionInspectorUsageSkeleton() {
  return (
    <SkeletonRegion
      label="Loading usage"
      className="flex flex-col gap-2.25"
      data-testid="session-inspector-usage-skeleton"
    >
      <MetricGrid aria-hidden="true" columns={2} className="grid-cols-2 gap-2">
        <Metric
          size="compact"
          className={SESSION_INSPECTOR_TILE}
          label={<SkeletonText className="w-16" />}
          value={<SkeletonText className="w-14" />}
          subtext={<SkeletonText className="w-24" />}
        />
        <Metric
          size="compact"
          className={SESSION_INSPECTOR_TILE}
          label={<SkeletonText className="w-20" />}
          value={<SkeletonText className="w-12" />}
        />
      </MetricGrid>
      <div aria-hidden="true" className="flex items-center gap-1.5">
        <Skeleton className="size-3.25 shrink-0 rounded-xs" />
        <span className="text-form-label font-medium">
          <SkeletonText className="w-24" />
        </span>
      </div>
    </SkeletonRegion>
  );
}

const TURN_ROW_KEYS = ["turn-a", "turn-b", "turn-c"] as const;

/** The per-turn history inside its fold: the live turn row's grid, one figure line and one detail line. */
export function SessionContextTurnsSkeleton() {
  return (
    <SkeletonRegion
      label="Loading turns"
      className="flex flex-col overflow-hidden rounded-lg bg-sunken py-1"
      data-testid="session-context-turns-skeleton"
    >
      <ul aria-hidden="true">
        {TURN_ROW_KEYS.map(key => (
          <li
            key={key}
            className="grid grid-cols-[3.75rem_minmax(0,1fr)_auto] items-center gap-2 px-2.5 py-1.75 text-form-label text-muted"
          >
            <span className="font-mono text-mono-id">
              <SkeletonText className="w-12" />
            </span>
            <span className="flex min-w-0 flex-col gap-0.5">
              <span className="font-mono text-mono-id">
                <SkeletonText className="w-24" />
              </span>
              <span className="text-micro leading-4">
                <SkeletonText className="w-32" />
              </span>
            </span>
            <span className="font-mono text-mono-id">
              <SkeletonText className="w-10" />
            </span>
          </li>
        ))}
      </ul>
    </SkeletonRegion>
  );
}
