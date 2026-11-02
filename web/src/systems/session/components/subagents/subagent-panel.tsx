import { useState, type ReactNode } from "react";
import { AnimatePresence, m, useReducedMotionConfig } from "motion/react";

import { PopoverTitle } from "@compozy/ui";

import { useContentHeight } from "../../hooks/use-content-height";
import { useSubagentPanelFocus } from "../../hooks/use-subagent-panel-focus";

import { SubagentDetail } from "./subagent-detail";
import { SubagentFleetBar } from "./subagent-fleet-bar";
import {
  subagentFinishedLabel,
  subagentRosterGroups,
  type SubagentCounts,
} from "./subagent-format";
import { SubagentRoster, type SubagentRosterProps } from "./subagent-roster";
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

/** The subagent list and one subagent's content on a single sliding track. */
export function SubagentPanel({
  subagents,
  counts,
  selectedId,
  onSelect,
  onOpen,
  onStop,
  renderContent,
  stale = false,
  placeholder,
}: SubagentPanelProps) {
  const reduced = useReducedMotionConfig();
  const { ref, height } = useContentHeight();
  const selected = subagents.find(subagent => subagent.id === selectedId) ?? null;
  const track = useSubagentPanelFocus(selected?.id ?? null);
  const [lastOpened, setLastOpened] = useState<string | null>(null);
  const opened = subagents.find(subagent => subagent.id === lastOpened);
  const returnsToPrevious =
    opened !== undefined && subagentRosterGroups([opened]).previous.length > 0;

  const direction = selected ? 1 : -1;
  const duration = reduced ? 0 : SLIDE_SECONDS;
  return (
    <m.div
      animate={{ height }}
      className="relative overflow-hidden"
      initial={false}
      transition={{ duration, ease: EASE }}
    >
      <div ref={ref}>
        <AnimatePresence custom={direction} initial={false} mode="popLayout">
          <m.div
            animate="center"
            className="w-full"
            custom={direction}
            exit="leave"
            initial="enter"
            key={selected?.id ?? "list"}
            ref={track}
            transition={{ duration, ease: EASE }}
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
                    className="mx-0 max-h-[28rem] p-2"
                    defaultPreviousOpen={counts.live === 0 || returnsToPrevious}
                    onOpen={(subagent, options) => {
                      if (options.newWindow) {
                        onOpen?.(subagent, options);
                        return;
                      }
                      setLastOpened(subagent.id);
                      onSelect(subagent.id);
                    }}
                    onStop={onStop}
                    stale={stale}
                    subagents={subagents}
                  />
                ) : null}
                {placeholder ? <div className="px-4 pt-1 pb-3.5">{placeholder}</div> : null}
              </>
            )}
          </m.div>
        </AnimatePresence>
      </div>
    </m.div>
  );
}
