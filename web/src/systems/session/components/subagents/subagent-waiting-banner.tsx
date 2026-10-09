import { Fragment, type ReactNode } from "react";
import { Square } from "lucide-react";
import { useReducedMotionConfig } from "motion/react";

import { Button, StateGlyph, cn } from "@compozy/ui";

import { SUBAGENT_BANNER_STOP_FAILED } from "./subagent-format";
import type { SubagentView } from "./types";
import { useSubagentStop } from "./use-subagent-stop";

const NAME_LIMIT = 3;

export interface SubagentWaitingBannerProps {
  /** Live delegated subagents of an idle parent (see `subagentWaitingBannerRows`). */
  subagents: readonly SubagentView[];
  /** A settled subagent will wake the parent: the delegated glyph breathes. */
  wakePending?: boolean;
  onOpen?: (subagent: SubagentView) => void;
  /** `and N more` opens the inspector's Subagents section. */
  onShowAll?: () => void;
  /** Cancels every live delegated subagent of this session. */
  onStop?: () => Promise<unknown>;
  /** Owner-held pending state, for a stop already in flight elsewhere. */
  stopping?: boolean;
  className?: string;
}

const NAME_CLASS =
  "rounded-xxs text-fg-2 underline decoration-line-strong underline-offset-2 outline-none hover:text-fg-strong hover:decoration-fg-2 focus-visible:shadow-focus-ring";

function SubagentName({
  subagent,
  onOpen,
}: {
  subagent: SubagentView;
  onOpen?: (subagent: SubagentView) => void;
}) {
  return (
    <button
      type="button"
      aria-label={`Open subagent ${subagent.title}`}
      className={NAME_CLASS}
      onClick={() => onOpen?.(subagent)}
    >
      {subagent.title}
    </button>
  );
}

/**
 * Docked on the composer's top edge while the parent's turn is over and its
 * delegated subagents still work (composer VC-01/02): who it waits on, and one
 * Stop for all of them. The composer stays usable underneath.
 */
export function SubagentWaitingBanner({
  subagents,
  wakePending = false,
  onOpen,
  onShowAll,
  onStop,
  stopping = false,
  className,
}: SubagentWaitingBannerProps) {
  const reduced = useReducedMotionConfig();
  const { pending, stop } = useSubagentStop(onStop, SUBAGENT_BANNER_STOP_FAILED);
  if (subagents.length === 0) return null;

  const busy = stopping || pending;
  const single = subagents.length === 1 ? subagents[0] : null;
  const named = subagents.slice(0, NAME_LIMIT);
  const more = subagents.length - named.length;

  let names: ReactNode = null;
  if (!single) {
    names = (
      <span className="min-w-0 truncate text-transcript-caption text-subtle">
        {named.map((subagent, index) => (
          <Fragment key={subagent.id}>
            {index > 0 ? ", " : null}
            <SubagentName subagent={subagent} onOpen={onOpen} />
          </Fragment>
        ))}
        {more > 0 ? (
          <>
            {" and "}
            <button type="button" className={NAME_CLASS} onClick={onShowAll}>
              {more} more
            </button>
          </>
        ) : null}
      </span>
    );
  }

  return (
    <div
      role="status"
      data-slot="subagent-waiting-banner"
      data-stopping={busy ? "true" : undefined}
      className={cn(
        "relative mx-2.5 -mb-px flex min-h-10 items-center gap-2.5 rounded-t-lg border border-b-0 border-line bg-canvas-soft py-1.75 pr-2 pl-3",
        className
      )}
    >
      <StateGlyph
        state="delegated"
        className={cn("mt-0.5 self-start", wakePending && !reduced && "animate-pulse")}
      />
      <span className="flex min-w-0 flex-1 flex-col gap-px">
        <span className="truncate text-transcript-meta font-medium text-fg-strong">
          {single ? (
            <>
              Waiting on subagent <SubagentName subagent={single} onOpen={onOpen} />
            </>
          ) : (
            `Waiting on ${subagents.length} subagents`
          )}
        </span>
        {names}
      </span>
      <Button type="button" variant="ghost" size="sm" disabled={busy} onClick={stop}>
        {busy ? <StateGlyph size="sm" state="running" /> : <Square aria-hidden="true" />}
        {busy ? "Stopping…" : "Stop"}
      </Button>
    </div>
  );
}
