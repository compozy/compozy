import { HoverCard, HoverCardContent, HoverCardTrigger, StateGlyph, cn } from "@compozy/ui";

import { SubagentAvatar } from "./subagent-avatar";
import { subagentChipPreview, subagentChipState, type SubagentCounts } from "./subagent-format";
import { SubagentRowTrail } from "./subagent-row-trail";
import type { SubagentView } from "./types";

export interface SubagentChipProps {
  counts: SubagentCounts;
  /** The parent row's own turn: idle with live subagents reads `delegated`. */
  parentTurnRunning: boolean;
  /** Rows for the hover preview; the most urgent five are shown. */
  preview?: readonly SubagentView[];
  /** Opens the inspector's Subagents section. */
  onOpen?: () => void;
  className?: string;
}

/**
 * Sidebar parent-row chip (nav VC-02): the most urgent glyph and `live/total`,
 * or the total alone when only failures remain. Hidden once every subagent
 * settled cleanly. Hover previews the most urgent five.
 */
export function SubagentChip({
  counts,
  parentTurnRunning,
  preview,
  onOpen,
  className,
}: SubagentChipProps) {
  const state = subagentChipState(counts, parentTurnRunning);
  if (!state) return null;
  const list = preview ? subagentChipPreview(preview, counts.total) : null;

  return (
    <HoverCard>
      <HoverCardTrigger
        render={
          <button
            type="button"
            aria-label={state.ariaLabel}
            data-slot="subagent-chip"
            data-state={state.glyph}
            onClick={onOpen}
            className={cn(
              "inline-flex h-5 shrink-0 items-center gap-1.25 rounded-pill pr-1.5 pl-1.25 font-mono text-micro text-muted tabular-nums outline-none transition-colors duration-fast ease-out hover:bg-surface-2 hover:text-fg focus-visible:shadow-focus-ring data-popup-open:bg-surface-2 data-popup-open:text-fg",
              className
            )}
          />
        }
      >
        <StateGlyph size="sm" state={state.glyph} />
        {state.text}
      </HoverCardTrigger>
      <HoverCardContent side="right" align="start" className="w-70">
        <div className="mb-1.5 flex items-baseline gap-2">
          <span className="text-transcript-meta font-semibold text-fg-strong">
            {counts.total} {counts.total === 1 ? "subagent" : "subagents"}
          </span>
          {counts.live > 0 ? (
            <span className="ml-auto font-mono text-mono-id text-faint tabular-nums">
              {counts.live} running
            </span>
          ) : null}
        </div>
        {list ? (
          <ul className="-mx-1.5 flex flex-col gap-px">
            {list.rows.map(subagent => (
              <li
                key={subagent.id}
                className="grid grid-cols-[20px_minmax(0,1fr)_auto] items-center gap-2 rounded-sm px-1.5 py-1"
              >
                <SubagentAvatar
                  provider={subagent.runtime.provider}
                  status={subagent.status}
                  size="sm"
                  surface="elevated"
                />
                <span className="truncate text-transcript-caption text-fg">{subagent.title}</span>
                <SubagentRowTrail subagent={subagent} />
              </li>
            ))}
          </ul>
        ) : null}
        {list && list.more > 0 ? (
          <div className="mt-1.5 border-t border-line-soft pt-1.5 text-transcript-caption text-subtle">
            +{list.more} more
          </div>
        ) : null}
      </HoverCardContent>
    </HoverCard>
  );
}
