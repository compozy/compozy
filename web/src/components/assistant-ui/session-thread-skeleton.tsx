import { InputGroup, Skeleton, SkeletonRegion, SkeletonText } from "@compozy/ui";

import { cn } from "@/lib/utils";

import { ThreadContentRail } from "./session-thread-content-rail";
import { SESSION_THREAD_CONTENT_INSET_DEFAULT } from "./session-thread-content-rail-constants";

// Content-shaped placeholders for the session transcript and its composer. Every row repeats the
// real row's frame (`UserMessage`, `AssistantMessage`, the composer shell) and puts `SkeletonText`
// where text goes, inside the typography of the text it replaces, so a line is exactly one real
// line box tall. None of it is interactive or announced: whoever renders these wraps the whole
// area in the one `SkeletonRegion` that carries the loading semantics.

const USER_ROW_FRAME = "group/message flex w-full min-w-0 justify-end pt-1 pb-transcript-turn-gap";
const USER_COLUMN_FRAME = "flex max-w-[80%] min-w-0 flex-col items-end gap-transcript-meta-gap";
const USER_BUBBLE_FRAME =
  "w-fit max-w-full min-w-0 rounded-xl bg-chat-fill-user px-3.5 py-2 text-transcript-message leading-relaxed";
const ASSISTANT_ROW_FRAME = "group/message flex w-full min-w-0 pt-1 pb-transcript-turn-gap";
const ASSISTANT_COLUMN_FRAME = "flex min-w-0 flex-1 flex-col gap-transcript-inline-gap";
/** The room a settled message keeps for its 22 px copy and timestamp row. */
const ACTIONS_ROW_FRAME = "h-5.5";
const WORK_ROW_FRAME =
  "flex min-h-transcript-line min-w-0 items-center gap-1.5 px-1 text-small-body";

function UserRowSkeleton({ width, lines = 1 }: { width: string; lines?: number }) {
  return (
    <div className={USER_ROW_FRAME}>
      <div className={USER_COLUMN_FRAME}>
        <div className={USER_BUBBLE_FRAME}>
          <SkeletonText lines={lines} className={width} />
        </div>
        <div className={ACTIONS_ROW_FRAME} />
      </div>
    </div>
  );
}

function AssistantRowSkeleton({
  lines,
  tool = false,
}: {
  lines: number;
  /** A work-summary line (glyph and label) above the prose, as most agent turns open. */
  tool?: boolean;
}) {
  return (
    <div className={ASSISTANT_ROW_FRAME}>
      <div className={ASSISTANT_COLUMN_FRAME}>
        {tool ? (
          <div className={WORK_ROW_FRAME}>
            <Skeleton className="size-3.5 shrink-0 rounded-xs" />
            <SkeletonText className="w-44" />
          </div>
        ) : null}
        <SkeletonText lines={lines} className="text-body leading-prose" />
        <div className={ACTIONS_ROW_FRAME} />
      </div>
    </div>
  );
}

/** The rows of a transcript that has not arrived: an operator turn, an agent turn with its work, and a follow-up. */
export function SessionTranscriptRowsSkeleton() {
  return (
    <>
      <UserRowSkeleton width="w-64" lines={2} />
      <AssistantRowSkeleton lines={3} tool />
      <UserRowSkeleton width="w-40" />
      <AssistantRowSkeleton lines={2} />
    </>
  );
}

/**
 * The transcript while its history loads, drawn inside the viewport's own rail. The one status
 * region of the area; it never poses as an empty session.
 */
export function SessionTranscriptSkeleton() {
  return (
    <SkeletonRegion
      label="Loading transcript"
      data-testid="thread-transcript-skeleton"
      className="flex w-full min-w-0 flex-col py-6"
    >
      <SessionTranscriptRowsSkeleton />
    </SkeletonRegion>
  );
}

/**
 * The composer card at rest: the real `InputGroup` composer surface, a one-line prompt and a tool
 * row of the 28 px controls, sitting on the same rail and padding as the live composer.
 */
export function SessionComposerSkeleton() {
  return (
    <div className="bg-canvas" aria-hidden="true" data-testid="composer-skeleton">
      <ThreadContentRail inset={SESSION_THREAD_CONTENT_INSET_DEFAULT} className="pt-2 pb-5">
        <InputGroup variant="composer" role="presentation">
          <div className="flex min-w-0 flex-col gap-transcript-inline-gap">
            <div className="min-h-6 w-full text-body leading-relaxed">
              <SkeletonText className="w-56" />
            </div>
            <div className="flex items-center gap-1">
              <Skeleton className="size-button-icon-default rounded-md" />
              <Skeleton className="size-button-icon-default rounded-md" />
              <Skeleton className="size-button-icon-default rounded-md" />
              <Skeleton className="ml-auto size-button-icon-default rounded-full" />
            </div>
          </div>
        </InputGroup>
      </ThreadContentRail>
    </div>
  );
}

/**
 * The thread pane's placeholder shapes without their status region: the transcript viewport over
 * the composer card, in the pane the live thread fills. For a host that owns the one region
 * (the whole session window). A read-only host has no composer to hold a place for.
 */
export function SessionThreadSkeletonBody({ composer = true }: { composer?: boolean }) {
  return (
    <>
      <div className="min-h-0 min-w-0 flex-1 overflow-hidden">
        <ThreadContentRail inset={SESSION_THREAD_CONTENT_INSET_DEFAULT} className="min-h-full">
          <div className="flex w-full min-w-0 flex-col py-6">
            <SessionTranscriptRowsSkeleton />
          </div>
        </ThreadContentRail>
      </div>
      {composer ? <SessionComposerSkeleton /> : null}
    </>
  );
}

/** The thread pane before its chunk arrives: the placeholder shapes in their own status region. */
export function SessionThreadSkeleton({
  className,
  composer = true,
}: {
  className?: string;
  composer?: boolean;
}) {
  return (
    <SkeletonRegion
      label="Loading conversation"
      data-testid="session-thread-skeleton"
      className={cn("flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden", className)}
    >
      <SessionThreadSkeletonBody composer={composer} />
    </SkeletonRegion>
  );
}
