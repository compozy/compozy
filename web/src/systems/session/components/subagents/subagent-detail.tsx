import type { ReactNode } from "react";
import { ArrowUpRight, ChevronLeft, Square } from "@compozy/ui/icons";

import { Button, PopoverTitle, StateGlyph } from "@compozy/ui";

import { SubagentElapsed } from "./subagent-elapsed";
import { SubagentWorktreeFactRows } from "./subagent-hover-content";
import {
  SUBAGENT_ROW_STOP_FAILED,
  isSubagentStoppable,
  subagentDisplay,
  subagentElapsedClock,
} from "./subagent-format";
import { SubagentRuntimeMeta } from "./subagent-runtime-meta";
import { subagentTokenLabel } from "./subagent-row-format";
import { SubagentTile } from "./subagent-tile";
import type { SubagentView } from "./types";
import { useSubagentStop } from "./use-subagent-stop";

function DetailFact({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex min-w-0 flex-col gap-0.5 px-2.5 py-2">
      <dt className="text-transcript-caption text-muted-foreground">{label}</dt>
      <dd className="truncate text-small-body text-fg tabular-nums">{children}</dd>
    </div>
  );
}

/** Only what was reported: a delegated row has no usage, and absent is never zero. */
function DetailFacts({ subagent, stale }: { subagent: SubagentView; stale: boolean }) {
  const display = subagentDisplay(subagent, { stale });
  const clock = subagentElapsedClock(subagent, { stale });
  const facts = [
    <DetailFact key="status" label="Status">
      <span className="inline-flex items-center gap-1.5">
        <StateGlyph size="sm" state={display.glyph} still={stale} />
        {display.word}
      </span>
    </DetailFact>,
  ];
  if (clock.kind !== "none") {
    facts.push(
      <DetailFact key="time" label="Time">
        <SubagentElapsed className="font-sans text-small-body text-fg" clock={clock} />
      </DetailFact>
    );
  }
  if (subagent.usage) {
    facts.push(
      <DetailFact key="tools" label="Tools">
        {subagent.usage.tool_uses}
      </DetailFact>,
      <DetailFact key="tokens" label="Tokens">
        {subagentTokenLabel(subagent.usage.total_tokens)}
      </DetailFact>
    );
  }
  return (
    <dl
      className="grid auto-cols-fr grid-flow-col divide-x divide-line rounded-lg border border-line"
      data-slot="subagent-detail-facts"
    >
      {facts}
    </dl>
  );
}

function DetailFallback({ subagent }: { subagent: SubagentView }) {
  if (subagent.error) {
    return (
      <p className="text-small-body text-pretty text-destructive" role="alert">
        {subagent.error}
      </p>
    );
  }
  const text = subagent.result_preview || subagent.progress;
  return text ? <p className="text-small-body text-pretty text-fg-2">{text}</p> : null;
}

export interface SubagentDetailProps {
  subagent: SubagentView;
  onBack: () => void;
  /** Opens the subagent in its full surface, this window or a new one with ⌘/Ctrl. */
  onOpen?: (newWindow: boolean) => void;
  onStop?: (subagent: SubagentView) => Promise<unknown>;
  /** The subagent's own activity; the host owns its transcript. */
  children?: ReactNode;
  stale?: boolean;
}

/** One subagent inside the panel: who it is, what it spent, and what it did. */
export function SubagentDetail({
  subagent,
  onBack,
  onOpen,
  onStop,
  children,
  stale = false,
}: SubagentDetailProps) {
  const stoppable = isSubagentStoppable(subagent) && onStop !== undefined;
  const { pending, stop } = useSubagentStop(
    stoppable ? () => onStop(subagent) : undefined,
    SUBAGENT_ROW_STOP_FAILED,
    [subagent]
  );
  return (
    <div
      className="flex min-h-0 flex-col"
      data-slot="subagent-detail"
      data-status={subagent.status}
    >
      <div className="flex items-center gap-1 px-2 pt-2 pb-1">
        <Button
          aria-label="Back to subagents"
          data-subagent-back
          onClick={onBack}
          size="xs"
          type="button"
          variant="ghost"
        >
          <ChevronLeft aria-hidden="true" />
          Subagents
        </Button>
        <span className="flex-1" />
        {stoppable ? (
          <Button disabled={pending} onClick={stop} size="xs" type="button" variant="ghost">
            {pending ? <StateGlyph size="sm" state="running" /> : <Square aria-hidden="true" />}
            {pending ? "Stopping…" : "Stop"}
          </Button>
        ) : null}
        {onOpen ? (
          <Button
            onClick={event => onOpen(event.metaKey || event.ctrlKey)}
            size="xs"
            type="button"
            variant="ghost"
          >
            Open
            <ArrowUpRight aria-hidden="true" />
          </Button>
        ) : null}
      </div>
      <div className="flex max-h-[min(30rem,calc(var(--available-height,30rem)-3rem))] min-h-0 flex-col gap-4 overflow-y-auto pt-2 pb-2">
        <div className="flex flex-col gap-4 px-4">
          <header className="flex min-w-0 items-start gap-3">
            <SubagentTile stale={stale} subagent={subagent} />
            <div className="flex min-w-0 flex-1 flex-col gap-0.5">
              <PopoverTitle className="text-card-title font-semibold text-pretty text-fg-strong">
                {subagent.title}
              </PopoverTitle>
              <SubagentRuntimeMeta providerMark={false} subagent={subagent} />
            </div>
          </header>
          <DetailFacts stale={stale} subagent={subagent} />
          <SubagentWorktreeFactRows subagent={subagent} />
        </div>
        {children ?? (
          <div className="px-4 pb-2">
            <DetailFallback subagent={subagent} />
          </div>
        )}
      </div>
    </div>
  );
}
