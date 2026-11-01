import type { ReactNode } from "react";

import { Pill, Skeleton, SkeletonText } from "@compozy/ui";

import { placeholderSlots } from "../lib/placeholder-slots";

// The lede's frame and type live here once, so the skeleton below and the real lede cannot drift.
const LEDE_FRAME_CLASS = "flex flex-col gap-2 border-b border-line pt-4 pb-5";
const LEDE_HEADING_CLASS = "text-detail-h1 font-medium tracking-detail-h1 text-fg-strong";
const LEDE_META_CLASS =
  "flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-small-body text-subtle";
const LEDE_BODY_CLASS = "max-w-prose text-small-body text-muted-foreground";

interface LoopPageLedeProps {
  name: string;
  /** Rendered muted before the name (`Run` on the run form). */
  prefix?: string;
  tags: readonly string[];
  /** Short plain facts under the name (category, step count, recent success). */
  meta: readonly string[];
  lede?: ReactNode;
  testId?: string;
}

/**
 * Page lede for a single Loop: what it is called, what it is, and a few plain
 * facts at small scale.
 *
 * The name repeats the breadcrumb on purpose — the crumb is navigation, this is the
 * subject of the page.
 */
export function LoopPageLede({ name, prefix, tags, meta, lede, testId }: LoopPageLedeProps) {
  return (
    <div className={LEDE_FRAME_CLASS} data-testid={testId}>
      <LoopPageLedeHeading name={name} prefix={prefix}>
        {tags.map(tag => (
          <Pill key={tag} size="xs" tone="neutral">
            {tag}
          </Pill>
        ))}
      </LoopPageLedeHeading>
      {meta.length > 0 ? (
        <div className={LEDE_META_CLASS}>
          {meta.map((entry, index) => (
            <span className="contents" key={entry}>
              {index > 0 ? <span aria-hidden="true">·</span> : null}
              <span>{entry}</span>
            </span>
          ))}
        </div>
      ) : null}
      {lede ? <p className={LEDE_BODY_CLASS}>{lede}</p> : null}
    </div>
  );
}

function LoopPageLedeHeading({
  name,
  prefix,
  children,
}: {
  name: string;
  prefix?: string;
  children?: ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <h1 className={LEDE_HEADING_CLASS}>
        {prefix ? <span className="text-muted-foreground">{prefix} </span> : null}
        {name}
      </h1>
      {children}
    </div>
  );
}

interface LoopPageLedeSkeletonProps {
  /** The Loop's name, known from the route before any read answers. */
  name: string;
  prefix?: string;
  /** Pills the real lede will show after the name. */
  tags?: number;
  /** Plain facts the real lede will show under the name. */
  meta?: number;
  /**
   * The sentence under the facts: its text when it is static, or how many lines
   * the read will fill. Omit it for a lede that has none.
   */
  lede?: string | number;
  testId?: string;
}

/**
 * The lede while the Loop is being read. The name is already known, so it renders as
 * the real heading; the pills, facts and sentence the read will supply are placeholders
 * in the same slots, so the frame keeps its height when they arrive.
 */
export function LoopPageLedeSkeleton({
  name,
  prefix,
  tags = 0,
  meta = 0,
  lede,
  testId,
}: LoopPageLedeSkeletonProps) {
  return (
    <div className={LEDE_FRAME_CLASS} data-testid={testId}>
      <LoopPageLedeHeading name={name} prefix={prefix}>
        {placeholderSlots("lede-tag", tags).map(slot => (
          <Skeleton className="h-pill-xs w-12 rounded-pill" key={slot.id} />
        ))}
      </LoopPageLedeHeading>
      {meta > 0 ? (
        <div className={LEDE_META_CLASS}>
          {placeholderSlots("lede-meta", meta).map(slot => (
            <SkeletonText className="w-24" key={slot.id} />
          ))}
        </div>
      ) : null}
      {typeof lede === "string" ? <p className={LEDE_BODY_CLASS}>{lede}</p> : null}
      {typeof lede === "number" ? (
        <p className={LEDE_BODY_CLASS}>
          <SkeletonText lines={lede} />
        </p>
      ) : null}
    </div>
  );
}
