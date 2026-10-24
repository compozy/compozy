import {
  MetadataTile,
  PropertyRowSkeleton,
  Section,
  Skeleton,
  SkeletonText,
  Surface,
  Timeline,
  TimelineEvent,
} from "@compozy/ui";

import { TASK_SUBHEAD_CLASS, TASK_TIMELINE_CARD_CLASS } from "./task-surface-classes";

// Content-shaped placeholders for the task detail and run surfaces. Each shape is presentational:
// the call site owns the one `SkeletonRegion` around the loading area, so none renders a region
// of its own and they compose freely inside one. Repeated rows use static ids; a placeholder has
// no identity of its own. Shapes that have a real twin with distinctive structure (the rails,
// the runs table, the activity feed) live beside that twin.

/** The demoted meta line under the head, as a single block of text in the real line's box. */
export function TaskSubheadSkeleton() {
  return (
    <div aria-hidden="true" className={TASK_SUBHEAD_CLASS}>
      <SkeletonText className="w-40" />
    </div>
  );
}

/** Events a timeline stands in with, in order; each has a detail line or not, like real ones. */
const TIMELINE_EVENTS = [
  { id: "a", titleWidth: "w-56", detailWidth: "w-3/4" },
  { id: "b", titleWidth: "w-44", detailWidth: null },
  { id: "c", titleWidth: "w-52", detailWidth: "w-2/3" },
  { id: "d", titleWidth: "w-40", detailWidth: null },
  { id: "e", titleWidth: "w-48", detailWidth: "w-3/5" },
] as const;

export interface TaskTimelineSkeletonProps {
  /** Events to render, at most five. Defaults to 3. */
  events?: number;
  /** The feed's marker and rail (default), or the bare recent-activity list the Overview uses. */
  markers?: boolean;
}

/** A timeline of placeholder events, in the real timeline's rhythm. */
export function TaskTimelineSkeleton({ events = 3, markers = true }: TaskTimelineSkeletonProps) {
  const rows = TIMELINE_EVENTS.slice(0, events).map(event => (
    <TimelineEvent
      className={markers ? undefined : "pl-0"}
      description={event.detailWidth ? <SkeletonText className={event.detailWidth} /> : null}
      hasMarker={markers}
      key={event.id}
      time={<SkeletonText className="w-10" />}
      title={<SkeletonText className={event.titleWidth} />}
    />
  ));
  return markers ? (
    <Timeline aria-hidden="true">{rows}</Timeline>
  ) : (
    <ul aria-hidden="true" className="flex flex-col">
      {rows}
    </ul>
  );
}

/**
 * The Overview tab: description and recent activity, the two sections nearly every task has.
 * A task's optional sections (the state strip, result, subtasks, dependencies) are not guessed.
 */
export function TaskOverviewSkeleton() {
  return (
    <div aria-hidden="true" className="flex flex-col gap-6">
      <Section label="Description">
        <Surface className="flex flex-col px-4 py-3.5">
          <SkeletonText className="text-body" lines={3} />
        </Surface>
      </Section>
      <Section
        label="Recent activity"
        right={<Skeleton className="h-button-sm w-16 rounded-pill" />}
      >
        <div className={TASK_TIMELINE_CARD_CLASS}>
          <TaskTimelineSkeleton events={3} markers={false} />
        </div>
      </Section>
    </div>
  );
}

/** The run page's main column: the result and the run's activity, the two sections every run has. */
export function TaskRunMainSkeleton() {
  return (
    <main aria-hidden="true" className="flex min-w-0 flex-col gap-6">
      <Section label="Result">
        <div className="rounded-lg bg-card px-4 py-5 shadow-card">
          <SkeletonText className="text-small-body" lines={3} />
        </div>
      </Section>
      <Section label="Run activity">
        <div className={TASK_TIMELINE_CARD_CLASS}>
          <TaskTimelineSkeleton events={3} />
        </div>
      </Section>
    </main>
  );
}

/** The task Inspect drawer's Diagnostics pane: the snapshot tiles, a diagnostics note and the CLI hint. */
export function TaskDiagnosticsSkeleton() {
  return (
    <div aria-hidden="true" className="flex flex-col gap-4">
      <div className="grid grid-cols-2 gap-2.5">
        {["Next action", "Current run", "Session", "Scheduler"].map(label => (
          <MetadataTile key={label} label={label} value={<SkeletonText className="w-24" />} />
        ))}
      </div>
      <div className="rounded-md bg-sunken px-3.5 py-3 text-small-body leading-relaxed">
        <SkeletonText lines={2} />
      </div>
      <Skeleton className="h-16 rounded-md" />
      <SkeletonText className="w-40 text-form-hint" />
    </div>
  );
}

/** The run Inspect drawer's snapshot: the four lease tiles and the explanatory note. */
export function TaskRunInspectSkeleton() {
  return (
    <div aria-hidden="true">
      <div className="grid grid-cols-2 gap-2.5">
        {["Heartbeat", "Reserved until", "Claim token", "Idempotency key"].map(label => (
          <MetadataTile key={label} label={label} value={<SkeletonText className="w-20" />} />
        ))}
      </div>
      <div className="mt-4 rounded-md bg-sunken px-3.5 py-3 text-small-body leading-relaxed">
        <SkeletonText lines={2} />
      </div>
    </div>
  );
}

/** Setup groups a loading profile stands in with: the real group labels and a few rows each. */
const SETUP_GROUPS = [
  { label: "Worker", rows: 3 },
  { label: "Coordinator", rows: 2 },
  { label: "Review", rows: 1 },
  { label: "Environment", rows: 2 },
] as const;

/** The setup sheet's read view: the four profile groups, each a few key and value rows. */
export function TaskSetupProfileSkeleton() {
  return (
    <div aria-hidden="true">
      {SETUP_GROUPS.map(group => (
        <Section className="mb-5" key={group.label} label={group.label}>
          <PropertyRowSkeleton count={group.rows} />
        </Section>
      ))}
    </div>
  );
}

/** One page of an externally stored result: the range line and actions, then the code block. */
export function TaskExternalResultSkeleton() {
  return (
    <div aria-hidden="true" className="flex min-w-0 flex-col gap-2.5">
      <div className="flex items-center justify-between gap-2">
        <SkeletonText className="w-44 eyebrow" />
        <div className="flex items-center gap-2">
          <Skeleton className="h-button-sm w-24 rounded-pill" />
          <Skeleton className="size-button-icon-xs rounded-pill" />
          <Skeleton className="size-button-icon-xs rounded-pill" />
        </div>
      </div>
      <Skeleton className="h-40 rounded-lg" />
    </div>
  );
}
