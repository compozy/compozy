import { useState, type ReactNode } from "react";
import { AnimatePresence, m, useReducedMotionConfig } from "motion/react";

import { PopoverTitle } from "@compozy/ui";

import { useSubagentPanelFocus } from "../../hooks/use-subagent-panel-focus";
import { SubagentDetail } from "./subagent-detail";
import { SubagentFleetBar } from "./subagent-fleet-bar";
import { SUBAGENT_ROSTER_FIRST_PAGE } from "./subagent-roster-frame";
import type { SubagentCounts } from "./subagent-format";
import { subagentFinishedLabel, subagentRosterViewWith } from "./subagent-row-format";
import {
  SubagentRoster,
  type SubagentRosterProps,
  type SubagentRosterView,
} from "./subagent-roster";
import type { SubagentView } from "./types";

/** `--ease-spring`: two full-width panes moving as one track. */
const EASE = [0.32, 0.72, 0.28, 1] as const;
const SLIDE_SECONDS = 0.3;
const SLIDE = {
  enter: (direction: number) => ({ x: `${direction * 100}%` }),
  center: { x: "0%" },
  leave: (direction: number) => ({ x: `${direction * -100}%` }),
};

export interface SubagentPanelProps {
  subagents: readonly SubagentView[];
  counts: SubagentCounts;
  selectedId: string | null;
  onSelect: (id: string | null) => void;
  /** The opened subagent's full surface; ⌘/Ctrl on a row goes straight to a new window. */
  onOpen?: SubagentRosterProps["onOpen"];
  onStop?: SubagentRosterProps["onStop"];
  /** The opened subagent's activity; the host owns its transcript. */
  renderContent?: (subagent: SubagentView) => ReactNode;
  stale?: boolean;
  /** Replaces the roster while it loads or fails. */
  placeholder?: ReactNode;
}

function SubagentListPane({
  subagents,
  counts,
  onSelect,
  onOpen,
  onStop,
  stale,
  placeholder,
  view,
  onViewChange,
}: Omit<SubagentPanelProps, "selectedId" | "renderContent"> & {
  view: SubagentRosterView;
  onViewChange: (view: SubagentRosterView) => void;
}) {
  return (
    <>
      <header className="flex flex-col gap-2.5 px-4 pt-3.5 pb-1">
        <div className="flex items-baseline gap-2">
          <PopoverTitle className="text-card-title font-semibold text-fg-strong">
            Subagents
          </PopoverTitle>
          {counts.total > 0 ? (
            <span className="ml-auto text-transcript-meta text-muted-foreground tabular-nums">
              {subagentFinishedLabel(counts)}
            </span>
          ) : null}
        </div>
        {subagents.length > 0 ? <SubagentFleetBar subagents={subagents} /> : null}
      </header>
      {subagents.length > 0 ? (
        <SubagentRoster
          className="mx-0 max-h-[min(28rem,calc(var(--available-height,28rem)-5rem))] p-2"
          onOpen={(subagent, options) => {
            if (options.newWindow) onOpen?.(subagent, options);
            else onSelect(subagent.id);
          }}
          onStop={onStop}
          onViewChange={onViewChange}
          stale={stale}
          subagents={subagents}
          view={view}
        />
      ) : null}
      {placeholder ? <div className="px-4 pt-1 pb-3.5">{placeholder}</div> : null}
    </>
  );
}

/** The subagent list and one subagent's content on a single sliding track. */
export function SubagentPanel({
  selectedId,
  renderContent,
  stale = false,
  ...list
}: SubagentPanelProps) {
  const { subagents, onSelect, onOpen, onStop } = list;
  const reduced = useReducedMotionConfig();
  const selected = subagents.find(subagent => subagent.id === selectedId) ?? null;
  const track = useSubagentPanelFocus(selected?.id ?? null);
  // The list unmounts behind the detail; coming back finds it unfolded as it was left, with the
  // row in reach even if it finished meanwhile.
  const [view, setView] = useState<SubagentRosterView>({
    limit: SUBAGENT_ROSTER_FIRST_PAGE,
    previousOpen: list.counts.live === 0,
  });
  const [returningTo, setReturningTo] = useState<string | null>(null);
  const direction = selected ? 1 : -1;
  return (
    <div className="relative overflow-hidden">
      <AnimatePresence custom={direction} initial={false} mode="popLayout">
        <m.div
          animate="center"
          className="w-full"
          custom={direction}
          exit="leave"
          initial="enter"
          key={selected?.id ?? "list"}
          ref={track}
          transition={{ duration: reduced ? 0 : SLIDE_SECONDS, ease: EASE }}
          variants={SLIDE}
        >
          {selected ? (
            <SubagentDetail
              onBack={() => onSelect(null)}
              onOpen={onOpen ? newWindow => onOpen(selected, { newWindow }) : undefined}
              onStop={onStop}
              stale={stale}
              subagent={selected}
            >
              {renderContent?.(selected)}
            </SubagentDetail>
          ) : (
            <SubagentListPane
              {...list}
              onSelect={id => {
                setReturningTo(id);
                onSelect(id);
              }}
              onViewChange={next => {
                setReturningTo(null);
                setView(next);
              }}
              stale={stale}
              view={subagentRosterViewWith(subagents, view, returningTo)}
            />
          )}
        </m.div>
      </AnimatePresence>
    </div>
  );
}
